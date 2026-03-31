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

export type RuntimeEnvironment = z.infer<typeof runtimeEnvironmentSchema>;
export type ReadinessStatus = z.infer<typeof readinessStatusSchema>;
export type ReadinessCheck = z.infer<typeof readinessCheckSchema>;
export type HealthPayload = z.infer<typeof healthPayloadSchema>;
export type ReadinessPayload = z.infer<typeof readinessPayloadSchema>;
