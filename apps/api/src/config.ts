import { runtimeEnvironmentSchema } from "@bore/contracts";
import { z } from "zod";

const portSchema = z.coerce.number().int().min(1).max(65535);

const envSchema = z.object({
  NODE_ENV: runtimeEnvironmentSchema.default("development"),
  BORE_V2_API_HOST: z.string().min(1).default("0.0.0.0"),
  BORE_V2_API_PORT: portSchema.default(3000),
  BORE_V2_APP_VERSION: z.string().min(1).default("0.0.0-phase1"),
  BORE_V2_PUBLIC_ORIGIN: z.string().url().default("http://localhost:8080"),
  BORE_V2_DATABASE_URL: z
    .string()
    .min(1)
    .refine(
      (value) =>
        value.startsWith("postgres://") || value.startsWith("postgresql://"),
      "BORE_V2_DATABASE_URL must start with postgres:// or postgresql://",
    )
    .default("postgres://bore:bore@localhost:15432/bore_v2"),
  BORE_V2_DATABASE_SSL: z
    .enum(["true", "false"])
    .default("false")
    .transform((value) => value === "true"),
});

export type AppConfig = ReturnType<typeof parseConfig>;

export function parseConfig(env: Record<string, string | undefined>) {
  const parsed = envSchema.parse(env);

  return {
    environment: parsed.NODE_ENV,
    host: parsed.BORE_V2_API_HOST,
    port: parsed.BORE_V2_API_PORT,
    version: parsed.BORE_V2_APP_VERSION,
    publicOrigin: parsed.BORE_V2_PUBLIC_ORIGIN,
    databaseUrl: parsed.BORE_V2_DATABASE_URL,
    databaseSsl: parsed.BORE_V2_DATABASE_SSL,
  };
}
