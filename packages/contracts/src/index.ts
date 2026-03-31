import { z } from "zod";

export const serviceName = "bore-v2-api";

export const runtimeEnvironmentSchema = z.enum([
  "development",
  "test",
  "production",
]);

export const readinessStatusSchema = z.enum(["ready", "not_ready"]);

export const readinessCheckSchema = z.object({
  name: z.enum(["config", "database"]),
  status: readinessStatusSchema,
  detail: z.string().min(1).optional(),
  latencyMs: z.number().int().nonnegative().optional(),
});

export const healthPayloadSchema = z.object({
  service: z.literal(serviceName),
  status: z.literal("ok"),
  version: z.string().min(1),
  environment: runtimeEnvironmentSchema,
  uptimeSeconds: z.number().nonnegative(),
  timestamp: z.string().datetime(),
  readiness: readinessStatusSchema,
});

export const readinessPayloadSchema = z.object({
  service: z.literal(serviceName),
  status: readinessStatusSchema,
  version: z.string().min(1),
  timestamp: z.string().datetime(),
  checks: z.array(readinessCheckSchema).min(1),
});

export const apiErrorCodeSchema = z.enum([
  "bad_request",
  "not_found",
  "conflict",
  "payload_too_large",
  "rate_limited",
  "timeout",
  "internal_error",
]);

export const apiErrorIssueSchema = z.object({
  code: z.string().min(1),
  message: z.string().min(1),
  path: z.array(z.union([z.string(), z.number()])).optional(),
});

export const apiErrorSchema = z.object({
  code: apiErrorCodeSchema,
  message: z.string().min(1),
  issues: z.array(apiErrorIssueSchema).optional(),
});

export const apiErrorPayloadSchema = z.object({
  error: apiErrorSchema,
});

export const sessionCodeSchema = z
  .string()
  .trim()
  .min(7)
  .max(128)
  .regex(
    /^[a-z0-9]+(?:-[a-z0-9]+){2,}$/,
    "session codes must be lowercase hyphenated rendezvous words",
  );

export const participantRoleSchema = z.enum(["sender", "receiver"]);
export const participantStatusSchema = z.enum(["pending", "joined"]);

export const sessionStatusSchema = z.enum([
  "waiting_receiver",
  "ready",
  "completed",
  "failed",
  "expired",
  "cancelled",
]);

export const transferFileSchema = z.object({
  name: z.string().trim().min(1).max(512),
  sizeBytes: z.number().int().nonnegative(),
  mimeType: z.string().trim().min(1).max(255).optional(),
  checksumSha256: z
    .string()
    .trim()
    .regex(
      /^[a-f0-9]{64}$/i,
      "checksumSha256 must be a 64-character hex digest",
    )
    .optional(),
});

export const sessionParticipantSchema = z.object({
  role: participantRoleSchema,
  status: participantStatusSchema,
  displayName: z.string().trim().min(1).max(120).optional(),
  joinedAt: z.string().datetime().optional(),
});

const jsonLiteralSchema = z.union([
  z.string(),
  z.number(),
  z.boolean(),
  z.null(),
]);

export type JsonValue =
  | string
  | number
  | boolean
  | null
  | JsonValue[]
  | { [key: string]: JsonValue };

export const jsonValueSchema: z.ZodType<JsonValue> = z.lazy(() =>
  z.union([
    jsonLiteralSchema,
    z.array(jsonValueSchema),
    z.record(z.string(), jsonValueSchema),
  ]),
);

export const sessionEventTypeSchema = z.enum([
  "session_created",
  "file_registered",
  "receiver_joined",
]);

export const sessionEventSchema = z.object({
  id: z.string().uuid(),
  type: sessionEventTypeSchema,
  actorRole: participantRoleSchema.optional(),
  timestamp: z.string().datetime(),
  payload: z.record(z.string(), jsonValueSchema),
});

export const sessionDetailSchema = z.object({
  id: z.string().uuid(),
  code: sessionCodeSchema,
  status: sessionStatusSchema,
  createdAt: z.string().datetime(),
  updatedAt: z.string().datetime(),
  expiresAt: z.string().datetime(),
  file: transferFileSchema,
  participants: z.array(sessionParticipantSchema).min(1).max(2),
  events: z.array(sessionEventSchema),
});

export const coordinationEnvelopeTypeSchema = z.enum([
  "session_snapshot",
  "session_event",
  "keepalive",
  "error",
]);

export const sessionSnapshotEnvelopeSchema = z.object({
  type: z.literal("session_snapshot"),
  session: sessionDetailSchema,
});

export const sessionEventEnvelopeSchema = z.object({
  type: z.literal("session_event"),
  sessionCode: sessionCodeSchema,
  status: sessionStatusSchema,
  event: sessionEventSchema,
});

export const coordinationKeepaliveEnvelopeSchema = z.object({
  type: z.literal("keepalive"),
  sessionCode: sessionCodeSchema,
  timestamp: z.string().datetime(),
});

export const coordinationErrorEnvelopeSchema = z.object({
  type: z.literal("error"),
  sessionCode: sessionCodeSchema.optional(),
  error: apiErrorSchema,
});

export const coordinationEnvelopeSchema = z.discriminatedUnion("type", [
  sessionSnapshotEnvelopeSchema,
  sessionEventEnvelopeSchema,
  coordinationKeepaliveEnvelopeSchema,
  coordinationErrorEnvelopeSchema,
]);

export const createSessionRequestSchema = z.object({
  file: transferFileSchema,
  senderDisplayName: z.string().trim().min(1).max(120).optional(),
  expiresInMinutes: z.number().int().min(1).max(60).default(15),
});

export const joinSessionRequestSchema = z.object({
  displayName: z.string().trim().min(1).max(120).optional(),
});

export const sessionRouteParamsSchema = z.object({
  code: sessionCodeSchema,
});

export const operatorSessionSummaryEntrySchema = z.object({
  id: z.string().uuid(),
  code: sessionCodeSchema,
  status: sessionStatusSchema,
  createdAt: z.string().datetime(),
  updatedAt: z.string().datetime(),
  expiresAt: z.string().datetime(),
  fileName: z.string().min(1),
  fileSizeBytes: z.number().int().nonnegative(),
  senderJoinedAt: z.string().datetime().optional(),
  receiverJoinedAt: z.string().datetime().optional(),
  lastEventType: sessionEventTypeSchema.optional(),
});

export const operatorSessionCountsSchema = z.object({
  total: z.number().int().nonnegative(),
  waitingReceiver: z.number().int().nonnegative(),
  ready: z.number().int().nonnegative(),
  completed: z.number().int().nonnegative(),
  failed: z.number().int().nonnegative(),
  expired: z.number().int().nonnegative(),
  cancelled: z.number().int().nonnegative(),
});

export const operatorSummaryPayloadSchema = z.object({
  generatedAt: z.string().datetime(),
  counts: operatorSessionCountsSchema,
  sessions: z.array(operatorSessionSummaryEntrySchema),
});

export type RuntimeEnvironment = z.infer<typeof runtimeEnvironmentSchema>;
export type ReadinessStatus = z.infer<typeof readinessStatusSchema>;
export type ReadinessCheck = z.infer<typeof readinessCheckSchema>;
export type HealthPayload = z.infer<typeof healthPayloadSchema>;
export type ReadinessPayload = z.infer<typeof readinessPayloadSchema>;
export type ApiError = z.infer<typeof apiErrorSchema>;
export type ApiErrorPayload = z.infer<typeof apiErrorPayloadSchema>;
export type ParticipantRole = z.infer<typeof participantRoleSchema>;
export type ParticipantStatus = z.infer<typeof participantStatusSchema>;
export type SessionStatus = z.infer<typeof sessionStatusSchema>;
export type TransferFile = z.infer<typeof transferFileSchema>;
export type SessionParticipant = z.infer<typeof sessionParticipantSchema>;
export type SessionEventType = z.infer<typeof sessionEventTypeSchema>;
export type SessionEvent = z.infer<typeof sessionEventSchema>;
export type SessionDetail = z.infer<typeof sessionDetailSchema>;
export type CoordinationEnvelopeType = z.infer<
  typeof coordinationEnvelopeTypeSchema
>;
export type SessionSnapshotEnvelope = z.infer<
  typeof sessionSnapshotEnvelopeSchema
>;
export type SessionEventEnvelope = z.infer<typeof sessionEventEnvelopeSchema>;
export type CoordinationKeepaliveEnvelope = z.infer<
  typeof coordinationKeepaliveEnvelopeSchema
>;
export type CoordinationErrorEnvelope = z.infer<
  typeof coordinationErrorEnvelopeSchema
>;
export type CoordinationEnvelope = z.infer<typeof coordinationEnvelopeSchema>;
export type CreateSessionRequest = z.infer<typeof createSessionRequestSchema>;
export type JoinSessionRequest = z.infer<typeof joinSessionRequestSchema>;
export type SessionRouteParams = z.infer<typeof sessionRouteParamsSchema>;
export type OperatorSessionSummaryEntry = z.infer<
  typeof operatorSessionSummaryEntrySchema
>;
export type OperatorSessionCounts = z.infer<typeof operatorSessionCountsSchema>;
export type OperatorSummaryPayload = z.infer<
  typeof operatorSummaryPayloadSchema
>;
