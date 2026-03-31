import { serviceName } from "@bore/contracts";

type JsonLogPrimitive = string | number | boolean | null;
type JsonLogValue =
  | JsonLogPrimitive
  | JsonLogValue[]
  | { [key: string]: JsonLogValue | undefined }
  | undefined;

export type LogFields = Record<string, JsonLogValue>;

export type Logger = {
  info(event: string, fields?: LogFields): void;
  warn(event: string, fields?: LogFields): void;
  error(event: string, fields?: LogFields): void;
};

type LogWriter = (message?: unknown, ...optionalParams: unknown[]) => void;

type LoggerSink = {
  info: LogWriter;
  warn: LogWriter;
  error: LogWriter;
};

function compactFields(fields: LogFields = {}) {
  return Object.fromEntries(
    Object.entries(fields).filter(([, value]) => value !== undefined),
  );
}

export function createJsonLogger(
  sink: LoggerSink = console,
  now: () => string = () => new Date().toISOString(),
): Logger {
  function write(
    level: "info" | "warn" | "error",
    event: string,
    fields?: LogFields,
  ) {
    const payload = JSON.stringify({
      timestamp: now(),
      level,
      service: serviceName,
      event,
      ...compactFields(fields),
    });

    sink[level](payload);
  }

  return {
    info(event, fields) {
      write("info", event, fields);
    },
    warn(event, fields) {
      write("warn", event, fields);
    },
    error(event, fields) {
      write("error", event, fields);
    },
  };
}
