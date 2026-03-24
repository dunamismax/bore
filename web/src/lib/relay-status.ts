import { z } from "zod";

export const relayStatusSchema = z.object({
  service: z.string(),
  status: z.string(),
  uptimeSeconds: z.number(),
  rooms: z.object({
    total: z.number(),
    waiting: z.number(),
    active: z.number(),
  }),
  limits: z.object({
    maxRooms: z.number(),
    roomTTLSeconds: z.number(),
    reapIntervalSeconds: z.number(),
    maxMessageSizeBytes: z.number(),
  }),
  transport: z.object({
    signalExchanges: z.number(),
    roomsRelayed: z.number(),
    bytesRelayed: z.number(),
    framesRelayed: z.number(),
  }),
});

export type RelayStatus = z.infer<typeof relayStatusSchema>;

export async function fetchRelayStatus(): Promise<RelayStatus> {
  const response = await fetch("/status", {
    headers: { Accept: "application/json" },
    cache: "no-store",
  });

  if (!response.ok) {
    throw new Error(`HTTP ${response.status}`);
  }

  const data: unknown = await response.json();
  return relayStatusSchema.parse(data);
}
