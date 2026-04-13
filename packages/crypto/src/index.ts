/**
 * Bore v2 end-to-end encryption primitives.
 *
 * Uses the Web Crypto API exclusively so that the same code runs in
 * both Bun (server/relay-side tests) and modern browsers (sender and
 * receiver transfer engines).
 *
 * Key exchange: ECDH P-256
 * Key derivation: HKDF-SHA-256
 * Bulk encryption: AES-256-GCM (per-chunk unique nonce)
 * Integrity: SHA-256
 */

const ECDH_CURVE = "P-256";
const AES_KEY_BITS = 256;
const GCM_NONCE_BYTES = 12;
const GCM_TAG_BITS = 128;
const HKDF_HASH = "SHA-256";
const HKDF_INFO = new TextEncoder().encode("bore-v2-transfer");
const CHUNK_INDEX_BYTES = 4;

export const CHUNK_HEADER_BYTES = CHUNK_INDEX_BYTES + GCM_NONCE_BYTES;
export const DEFAULT_CHUNK_SIZE = 256 * 1024;

// ---------------------------------------------------------------------------
// Key pair generation and export
// ---------------------------------------------------------------------------

export type KeyPairResult = {
  keyPair: CryptoKeyPair;
  publicKeyRaw: ArrayBuffer;
};

export async function generateKeyPair(): Promise<KeyPairResult> {
  const keyPair = await crypto.subtle.generateKey(
    { name: "ECDH", namedCurve: ECDH_CURVE },
    false,
    ["deriveBits"],
  );

  const publicKeyRaw = await crypto.subtle.exportKey("raw", keyPair.publicKey);

  return { keyPair, publicKeyRaw };
}

export function publicKeyToBase64(raw: ArrayBuffer): string {
  return bufferToBase64(raw);
}

export function base64ToPublicKey(encoded: string): ArrayBuffer {
  return base64ToBuffer(encoded);
}

// ---------------------------------------------------------------------------
// Key agreement and derivation
// ---------------------------------------------------------------------------

export async function deriveTransferKey(
  privateKey: CryptoKey,
  peerPublicKeyRaw: ArrayBuffer,
  salt: ArrayBuffer,
): Promise<CryptoKey> {
  const peerPublicKey = await crypto.subtle.importKey(
    "raw",
    peerPublicKeyRaw,
    { name: "ECDH", namedCurve: ECDH_CURVE },
    false,
    [],
  );

  const sharedBits = await crypto.subtle.deriveBits(
    { name: "ECDH", public: peerPublicKey },
    privateKey,
    256,
  );

  const hkdfKey = await crypto.subtle.importKey(
    "raw",
    sharedBits,
    "HKDF",
    false,
    ["deriveKey"],
  );

  return crypto.subtle.deriveKey(
    { name: "HKDF", hash: HKDF_HASH, salt, info: HKDF_INFO },
    hkdfKey,
    { name: "AES-GCM", length: AES_KEY_BITS },
    false,
    ["encrypt", "decrypt"],
  );
}

export function sessionCodeToSalt(code: string): ArrayBuffer {
  return new TextEncoder().encode(`bore-salt:${code}`).buffer as ArrayBuffer;
}

// ---------------------------------------------------------------------------
// Chunk encryption and decryption
// ---------------------------------------------------------------------------

export async function encryptChunk(
  key: CryptoKey,
  chunkIndex: number,
  plaintext: ArrayBuffer,
): Promise<ArrayBuffer> {
  const nonce = crypto.getRandomValues(new Uint8Array(GCM_NONCE_BYTES));

  const ciphertext = await crypto.subtle.encrypt(
    { name: "AES-GCM", iv: nonce, tagLength: GCM_TAG_BITS },
    key,
    plaintext,
  );

  const header = new ArrayBuffer(CHUNK_HEADER_BYTES);
  const headerView = new DataView(header);
  headerView.setUint32(0, chunkIndex, false);
  new Uint8Array(header, CHUNK_INDEX_BYTES).set(nonce);

  const frame = new Uint8Array(CHUNK_HEADER_BYTES + ciphertext.byteLength);
  frame.set(new Uint8Array(header), 0);
  frame.set(new Uint8Array(ciphertext), CHUNK_HEADER_BYTES);

  return frame.buffer as ArrayBuffer;
}

export type DecryptedChunk = {
  chunkIndex: number;
  plaintext: ArrayBuffer;
};

export async function decryptChunk(
  key: CryptoKey,
  frame: ArrayBuffer,
): Promise<DecryptedChunk> {
  if (frame.byteLength < CHUNK_HEADER_BYTES + GCM_TAG_BITS / 8) {
    throw new Error("frame too small to contain header and GCM tag");
  }

  const headerView = new DataView(frame, 0, CHUNK_HEADER_BYTES);
  const chunkIndex = headerView.getUint32(0, false);
  const nonce = new Uint8Array(frame, CHUNK_INDEX_BYTES, GCM_NONCE_BYTES);
  const ciphertext = new Uint8Array(frame, CHUNK_HEADER_BYTES);

  const plaintext = await crypto.subtle.decrypt(
    { name: "AES-GCM", iv: nonce, tagLength: GCM_TAG_BITS },
    key,
    ciphertext,
  );

  return { chunkIndex, plaintext };
}

// ---------------------------------------------------------------------------
// Integrity hashing
// ---------------------------------------------------------------------------

export async function sha256Hex(data: ArrayBuffer): Promise<string> {
  const digest = await crypto.subtle.digest("SHA-256", data);
  return bufferToHex(digest);
}

export async function sha256HexIncremental(): Promise<{
  update(chunk: ArrayBuffer): void;
  finalize(): Promise<string>;
}> {
  const chunks: ArrayBuffer[] = [];

  return {
    update(chunk: ArrayBuffer) {
      chunks.push(chunk);
    },
    async finalize() {
      let totalLength = 0;
      for (const chunk of chunks) {
        totalLength += chunk.byteLength;
      }

      const combined = new Uint8Array(totalLength);
      let offset = 0;
      for (const chunk of chunks) {
        combined.set(new Uint8Array(chunk), offset);
        offset += chunk.byteLength;
      }

      return sha256Hex(combined.buffer);
    },
  };
}

// ---------------------------------------------------------------------------
// Chunking utilities
// ---------------------------------------------------------------------------

export function computeChunkCount(
  fileSize: number,
  chunkSize: number = DEFAULT_CHUNK_SIZE,
): number {
  if (fileSize === 0) return 1;
  return Math.ceil(fileSize / chunkSize);
}

export function sliceChunk(
  file: ArrayBuffer,
  chunkIndex: number,
  chunkSize: number = DEFAULT_CHUNK_SIZE,
): ArrayBuffer {
  const start = chunkIndex * chunkSize;
  const end = Math.min(start + chunkSize, file.byteLength);
  return file.slice(start, end);
}

// ---------------------------------------------------------------------------
// Encoding helpers
// ---------------------------------------------------------------------------

function bufferToBase64(buffer: ArrayBuffer): string {
  const bytes = new Uint8Array(buffer);
  const parts: string[] = [];
  for (const byte of bytes) {
    parts.push(String.fromCharCode(byte));
  }
  return btoa(parts.join(""));
}

function base64ToBuffer(encoded: string): ArrayBuffer {
  const binary = atob(encoded);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i);
  }
  return bytes.buffer;
}

function bufferToHex(buffer: ArrayBuffer): string {
  const bytes = new Uint8Array(buffer);
  const parts: string[] = [];
  for (const byte of bytes) {
    parts.push(byte.toString(16).padStart(2, "0"));
  }
  return parts.join("");
}
