import type { ParticipantRole, WsControlMessage } from "@bore/contracts";
import { wsControlMessageSchema } from "@bore/contracts";

import type { Logger } from "./logger";
import type { SessionService } from "./sessions";

type PeerConnection = {
  ws: ServerWebSocket<PeerData>;
  role: ParticipantRole;
};

type PeerData = {
  sessionCode: string;
  role: ParticipantRole;
};

type SessionRoom = {
  sender: PeerConnection | null;
  receiver: PeerConnection | null;
};

type ServerWebSocket<T> = {
  data: T;
  send(data: string | ArrayBuffer | Uint8Array): void;
  close(code?: number, reason?: string): void;
  readyState: number;
};

export type RelayRoom = {
  rooms: Map<string, SessionRoom>;
  handleOpen(ws: ServerWebSocket<PeerData>): void;
  handleMessage(
    ws: ServerWebSocket<PeerData>,
    message: string | ArrayBuffer,
  ): void;
  handleClose(ws: ServerWebSocket<PeerData>): void;
};

export function createRelay(
  sessions: SessionService,
  logger: Logger,
): RelayRoom {
  const rooms = new Map<string, SessionRoom>();

  function getRoom(code: string): SessionRoom {
    let room = rooms.get(code);

    if (!room) {
      room = { sender: null, receiver: null };
      rooms.set(code, room);
    }

    return room;
  }

  function getPeer(
    room: SessionRoom,
    role: ParticipantRole,
  ): PeerConnection | null {
    return role === "sender" ? room.receiver : room.sender;
  }

  function cleanupRoom(code: string) {
    const room = rooms.get(code);

    if (!room) return;

    if (!room.sender && !room.receiver) {
      rooms.delete(code);
      logger.info("relay_room_removed", { sessionCode: code });
    }
  }

  function sendControl(ws: ServerWebSocket<PeerData>, msg: WsControlMessage) {
    ws.send(JSON.stringify(msg));
  }

  function handleControlMessage(
    ws: ServerWebSocket<PeerData>,
    room: SessionRoom,
    msg: WsControlMessage,
  ) {
    const { sessionCode, role } = ws.data;
    const peer = getPeer(room, role);

    switch (msg.type) {
      case "key_exchange": {
        if (peer) {
          sendControl(peer.ws, msg);
        }
        break;
      }

      case "transfer_start": {
        if (role !== "sender") {
          sendControl(ws, {
            type: "transfer_error",
            message: "only the sender can start a transfer",
          });
          return;
        }

        sessions.startTransfer(sessionCode).then(
          () => {
            if (peer) {
              sendControl(peer.ws, msg);
            }
          },
          (error) => {
            logger.error("relay_start_transfer_failed", {
              sessionCode,
              error: error instanceof Error ? error.message : "unknown",
            });
            sendControl(ws, {
              type: "transfer_error",
              message: "failed to start transfer",
            });
          },
        );
        break;
      }

      case "chunk_ack": {
        if (peer) {
          sendControl(peer.ws, msg);
        }
        break;
      }

      case "transfer_complete": {
        if (role !== "receiver") {
          sendControl(ws, {
            type: "transfer_error",
            message: "only the receiver can confirm completion",
          });
          return;
        }

        sessions.completeTransfer(sessionCode, msg.checksumSha256).then(
          () => {
            if (peer) {
              sendControl(peer.ws, msg);
            }
          },
          (error) => {
            logger.error("relay_complete_transfer_failed", {
              sessionCode,
              error: error instanceof Error ? error.message : "unknown",
            });
            sendControl(ws, {
              type: "transfer_error",
              message: "failed to complete transfer",
            });
          },
        );
        break;
      }

      case "transfer_error": {
        sessions.failTransfer(sessionCode, msg.message).catch((error) => {
          logger.error("relay_fail_transfer_failed", {
            sessionCode,
            error: error instanceof Error ? error.message : "unknown",
          });
        });

        if (peer) {
          sendControl(peer.ws, msg);
        }
        break;
      }
    }
  }

  return {
    rooms,

    handleOpen(ws) {
      const { sessionCode, role } = ws.data;
      const room = getRoom(sessionCode);

      if (role === "sender") {
        if (room.sender) {
          ws.close(4409, "sender already connected");
          return;
        }
        room.sender = { ws, role };
      } else {
        if (room.receiver) {
          ws.close(4409, "receiver already connected");
          return;
        }
        room.receiver = { ws, role };
      }

      logger.info("relay_peer_connected", { sessionCode, role });
    },

    handleMessage(ws, message) {
      const { sessionCode, role } = ws.data;
      const room = rooms.get(sessionCode);

      if (!room) {
        ws.close(4404, "session room not found");
        return;
      }

      if (typeof message === "string") {
        const parsed = wsControlMessageSchema.safeParse(JSON.parse(message));

        if (!parsed.success) {
          sendControl(ws, {
            type: "transfer_error",
            message: "invalid control message",
          });
          return;
        }

        handleControlMessage(ws, room, parsed.data);
        return;
      }

      // Binary message: forward encrypted chunk to peer
      const peer = getPeer(room, role);

      if (!peer) {
        logger.warn("relay_no_peer_for_binary", { sessionCode, role });
        return;
      }

      peer.ws.send(message);
    },

    handleClose(ws) {
      const { sessionCode, role } = ws.data;
      const room = rooms.get(sessionCode);

      if (!room) return;

      if (role === "sender" && room.sender?.ws === ws) {
        room.sender = null;
      } else if (role === "receiver" && room.receiver?.ws === ws) {
        room.receiver = null;
      }

      logger.info("relay_peer_disconnected", { sessionCode, role });

      // Notify remaining peer
      const remaining = role === "sender" ? room.receiver : room.sender;

      if (remaining) {
        sendControl(remaining.ws, {
          type: "transfer_error",
          message: `${role} disconnected`,
        });
      }

      cleanupRoom(sessionCode);
    },
  };
}
