import type { ParticipantRole, WsControlMessage } from "@bore/contracts";
import {
  base64ToPublicKey,
  computeChunkCount,
  DEFAULT_CHUNK_SIZE,
  decryptChunk,
  deriveTransferKey,
  encryptChunk,
  generateKeyPair,
  publicKeyToBase64,
  sessionCodeToSalt,
  sha256Hex,
  sha256HexIncremental,
} from "@bore/crypto";
import { ref } from "vue";

export type TransferState =
  | "idle"
  | "connecting"
  | "waiting_peer"
  | "key_exchange"
  | "transferring"
  | "verifying"
  | "completed"
  | "failed";

type TransferEngineOptions = {
  sessionCode: string;
  role: ParticipantRole;
  file?: File;
  expectedFileSize?: number;
  expectedFileName?: string;
  expectedChecksum?: string;
};

function buildWsUrl(sessionCode: string, role: ParticipantRole): string {
  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
  return `${protocol}//${window.location.host}/api/sessions/${encodeURIComponent(sessionCode)}/ws?role=${role}`;
}

export function useTransferEngine() {
  const state = ref<TransferState>("idle");
  const error = ref<string | null>(null);
  const bytesSent = ref(0);
  const bytesReceived = ref(0);
  const totalBytes = ref(0);
  const progress = ref(0);
  const receivedBlob = ref<Blob | null>(null);
  const receivedChecksum = ref<string | null>(null);

  let ws: WebSocket | null = null;
  let transferKey: CryptoKey | null = null;
  let cleanup: (() => void) | null = null;

  function reset() {
    state.value = "idle";
    error.value = null;
    bytesSent.value = 0;
    bytesReceived.value = 0;
    totalBytes.value = 0;
    progress.value = 0;
    receivedBlob.value = null;
    receivedChecksum.value = null;
    transferKey = null;

    if (ws) {
      ws.close();
      ws = null;
    }

    if (cleanup) {
      cleanup();
      cleanup = null;
    }
  }

  function sendControl(msg: WsControlMessage) {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify(msg));
    }
  }

  async function startSend(options: TransferEngineOptions) {
    reset();

    if (!options.file) {
      error.value = "no file selected";
      state.value = "failed";
      return;
    }

    const file = options.file;
    totalBytes.value = file.size;
    state.value = "connecting";

    const keyPair = await generateKeyPair();
    const salt = sessionCodeToSalt(options.sessionCode);

    ws = new WebSocket(buildWsUrl(options.sessionCode, "sender"));
    ws.binaryType = "arraybuffer";

    ws.onopen = () => {
      state.value = "waiting_peer";
      sendControl({
        type: "key_exchange",
        publicKey: publicKeyToBase64(keyPair.publicKeyRaw),
      });
    };

    ws.onmessage = async (event: MessageEvent) => {
      if (typeof event.data !== "string") return;

      let msg: WsControlMessage;
      try {
        msg = JSON.parse(event.data) as WsControlMessage;
      } catch {
        return;
      }

      switch (msg.type) {
        case "key_exchange": {
          state.value = "key_exchange";
          const peerPublicKey = base64ToPublicKey(msg.publicKey);
          transferKey = await deriveTransferKey(
            keyPair.keyPair.privateKey,
            peerPublicKey,
            salt,
          );

          state.value = "transferring";
          const chunkSize = DEFAULT_CHUNK_SIZE;
          const chunkCount = computeChunkCount(file.size, chunkSize);

          sendControl({
            type: "transfer_start",
            totalChunks: chunkCount,
            chunkSize,
          });

          // Stream the file in chunks
          const fileBuffer = await file.arrayBuffer();
          const hasher = await sha256HexIncremental();

          for (let i = 0; i < chunkCount; i++) {
            const start = i * chunkSize;
            const end = Math.min(start + chunkSize, fileBuffer.byteLength);
            const plainChunk = fileBuffer.slice(start, end);

            hasher.update(plainChunk);

            const encryptedFrame = await encryptChunk(
              transferKey,
              i,
              plainChunk,
            );
            ws?.send(encryptedFrame);

            bytesSent.value = end;
            progress.value = Math.round((end / file.size) * 100);
          }

          state.value = "verifying";
          const checksum = await hasher.finalize();
          // Wait for receiver to confirm via transfer_complete
          // Store checksum for verification
          receivedChecksum.value = checksum;
          break;
        }

        case "transfer_complete": {
          if (
            receivedChecksum.value &&
            msg.checksumSha256.toLowerCase() ===
              receivedChecksum.value.toLowerCase()
          ) {
            state.value = "completed";
          } else {
            error.value = "integrity check failed: checksum mismatch";
            state.value = "failed";
          }
          ws?.close();
          break;
        }

        case "transfer_error": {
          error.value = msg.message;
          state.value = "failed";
          ws?.close();
          break;
        }
      }
    };

    ws.onerror = () => {
      error.value = "WebSocket connection error";
      state.value = "failed";
    };

    ws.onclose = (event: CloseEvent) => {
      if (state.value !== "completed" && state.value !== "failed") {
        error.value = event.reason || "connection closed unexpectedly";
        state.value = "failed";
      }
    };
  }

  async function startReceive(options: TransferEngineOptions) {
    reset();

    totalBytes.value = options.expectedFileSize ?? 0;
    state.value = "connecting";

    const keyPair = await generateKeyPair();
    const salt = sessionCodeToSalt(options.sessionCode);

    let chunkCount = 0;
    let receivedChunks: ArrayBuffer[] = [];

    ws = new WebSocket(buildWsUrl(options.sessionCode, "receiver"));
    ws.binaryType = "arraybuffer";

    ws.onopen = () => {
      state.value = "waiting_peer";
      sendControl({
        type: "key_exchange",
        publicKey: publicKeyToBase64(keyPair.publicKeyRaw),
      });
    };

    ws.onmessage = async (event: MessageEvent) => {
      // Binary message: encrypted chunk
      if (event.data instanceof ArrayBuffer) {
        if (!transferKey) {
          error.value = "received data before key exchange completed";
          state.value = "failed";
          ws?.close();
          return;
        }

        try {
          const { chunkIndex, plaintext } = await decryptChunk(
            transferKey,
            event.data,
          );

          receivedChunks[chunkIndex] = plaintext;
          bytesReceived.value += plaintext.byteLength;

          if (totalBytes.value > 0) {
            progress.value = Math.round(
              (bytesReceived.value / totalBytes.value) * 100,
            );
          }

          sendControl({
            type: "chunk_ack",
            chunkIndex,
          });

          // Check if we have all chunks
          if (receivedChunks.filter(Boolean).length === chunkCount) {
            state.value = "verifying";

            // Reassemble and verify
            const allParts: Uint8Array[] = [];
            for (let i = 0; i < chunkCount; i++) {
              const chunk = receivedChunks[i];
              if (!chunk) {
                error.value = `missing chunk ${i}`;
                state.value = "failed";
                sendControl({
                  type: "transfer_error",
                  message: `missing chunk ${i}`,
                });
                ws?.close();
                return;
              }
              allParts.push(new Uint8Array(chunk));
            }

            let totalLength = 0;
            for (const part of allParts) {
              totalLength += part.byteLength;
            }
            const combined = new Uint8Array(totalLength);
            let offset = 0;
            for (const part of allParts) {
              combined.set(part, offset);
              offset += part.byteLength;
            }

            const checksum = await sha256Hex(combined.buffer as ArrayBuffer);

            receivedBlob.value = new Blob([combined], {
              type: "application/octet-stream",
            });
            receivedChecksum.value = checksum;

            sendControl({
              type: "transfer_complete",
              checksumSha256: checksum,
            });

            state.value = "completed";
            ws?.close();
          }
        } catch {
          error.value = "decryption failed";
          state.value = "failed";
          sendControl({
            type: "transfer_error",
            message: "decryption failed",
          });
          ws?.close();
        }

        return;
      }

      // Text message: control
      if (typeof event.data !== "string") return;

      let msg: WsControlMessage;
      try {
        msg = JSON.parse(event.data) as WsControlMessage;
      } catch {
        return;
      }

      switch (msg.type) {
        case "key_exchange": {
          state.value = "key_exchange";
          const peerPublicKey = base64ToPublicKey(msg.publicKey);
          transferKey = await deriveTransferKey(
            keyPair.keyPair.privateKey,
            peerPublicKey,
            salt,
          );
          state.value = "waiting_peer";
          break;
        }

        case "transfer_start": {
          chunkCount = msg.totalChunks;
          receivedChunks = new Array(chunkCount);
          state.value = "transferring";
          break;
        }

        case "transfer_error": {
          error.value = msg.message;
          state.value = "failed";
          ws?.close();
          break;
        }
      }
    };

    ws.onerror = () => {
      error.value = "WebSocket connection error";
      state.value = "failed";
    };

    ws.onclose = (event: CloseEvent) => {
      if (state.value !== "completed" && state.value !== "failed") {
        error.value = event.reason || "connection closed unexpectedly";
        state.value = "failed";
      }
    };
  }

  function downloadReceivedFile(fileName: string) {
    if (!receivedBlob.value) return;

    const url = URL.createObjectURL(receivedBlob.value);
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = fileName;
    document.body.appendChild(anchor);
    anchor.click();
    document.body.removeChild(anchor);
    URL.revokeObjectURL(url);
  }

  function abort() {
    if (ws && ws.readyState === WebSocket.OPEN) {
      sendControl({
        type: "transfer_error",
        message: "transfer cancelled by user",
      });
    }
    reset();
  }

  return {
    state,
    error,
    bytesSent,
    bytesReceived,
    totalBytes,
    progress,
    receivedBlob,
    receivedChecksum,
    startSend,
    startReceive,
    downloadReceivedFile,
    abort,
    reset,
  };
}
