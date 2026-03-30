export interface RelayStatus {
  service: string;
  status: string;
  uptimeSeconds: number;
  rooms: {
    total: number;
    waiting: number;
    active: number;
  };
  limits: {
    maxRooms: number;
    roomTTLSeconds: number;
    reapIntervalSeconds: number;
    maxMessageSizeBytes: number;
  };
  transport: {
    signalExchanges: number;
    signalingStarted: number;
    roomsRelayed: number;
    bytesRelayed: number;
    framesRelayed: number;
  };
}

function expectRecord(value: unknown, label: string): Record<string, unknown> {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    throw new Error(`${label} must be an object`);
  }
  return value as Record<string, unknown>;
}

function expectString(value: unknown, label: string): string {
  if (typeof value !== "string" || value.length === 0) {
    throw new Error(`${label} must be a non-empty string`);
  }
  return value;
}

function expectNumber(value: unknown, label: string): number {
  if (typeof value !== "number" || !Number.isFinite(value)) {
    throw new Error(`${label} must be a finite number`);
  }
  return value;
}

export function parseRelayStatus(value: unknown): RelayStatus {
  const root = expectRecord(value, "relay status");
  const rooms = expectRecord(root.rooms, "relay status.rooms");
  const limits = expectRecord(root.limits, "relay status.limits");
  const transport = expectRecord(root.transport, "relay status.transport");

  return {
    service: expectString(root.service, "relay status.service"),
    status: expectString(root.status, "relay status.status"),
    uptimeSeconds: expectNumber(
      root.uptimeSeconds,
      "relay status.uptimeSeconds",
    ),
    rooms: {
      total: expectNumber(rooms.total, "relay status.rooms.total"),
      waiting: expectNumber(rooms.waiting, "relay status.rooms.waiting"),
      active: expectNumber(rooms.active, "relay status.rooms.active"),
    },
    limits: {
      maxRooms: expectNumber(limits.maxRooms, "relay status.limits.maxRooms"),
      roomTTLSeconds: expectNumber(
        limits.roomTTLSeconds,
        "relay status.limits.roomTTLSeconds",
      ),
      reapIntervalSeconds: expectNumber(
        limits.reapIntervalSeconds,
        "relay status.limits.reapIntervalSeconds",
      ),
      maxMessageSizeBytes: expectNumber(
        limits.maxMessageSizeBytes,
        "relay status.limits.maxMessageSizeBytes",
      ),
    },
    transport: {
      signalExchanges: expectNumber(
        transport.signalExchanges,
        "relay status.transport.signalExchanges",
      ),
      signalingStarted: expectNumber(
        transport.signalingStarted,
        "relay status.transport.signalingStarted",
      ),
      roomsRelayed: expectNumber(
        transport.roomsRelayed,
        "relay status.transport.roomsRelayed",
      ),
      bytesRelayed: expectNumber(
        transport.bytesRelayed,
        "relay status.transport.bytesRelayed",
      ),
      framesRelayed: expectNumber(
        transport.framesRelayed,
        "relay status.transport.framesRelayed",
      ),
    },
  };
}

export async function fetchRelayStatus(
  signal?: AbortSignal,
): Promise<RelayStatus> {
  const response = await fetch("/status", {
    cache: "no-store",
    headers: { Accept: "application/json" },
    signal,
  });
  if (!response.ok) {
    throw new Error(`GET /status returned ${response.status}`);
  }
  return parseRelayStatus(await response.json());
}

export function formatDuration(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds <= 0) {
    return "0s";
  }

  const units: Array<[string, number]> = [
    ["d", 86_400],
    ["h", 3_600],
    ["m", 60],
    ["s", 1],
  ];

  let remaining = Math.floor(seconds);
  const parts: string[] = [];

  for (const [label, size] of units) {
    if (parts.length === 2) {
      break;
    }
    const amount = Math.floor(remaining / size);
    remaining %= size;
    if (amount > 0) {
      parts.push(`${amount}${label}`);
    }
  }

  return parts.join(" ") || "0s";
}

export function formatBytes(numBytes: number): string {
  if (!Number.isFinite(numBytes) || numBytes <= 0) {
    return "0 B";
  }

  const units = ["B", "KB", "MB", "GB", "TB"];
  let value = numBytes;
  let unit = units[0];

  for (const nextUnit of units) {
    unit = nextUnit;
    if (value < 1024 || nextUnit === units.at(-1)) {
      break;
    }
    value /= 1024;
  }

  if (value >= 100) {
    return `${value.toFixed(0)} ${unit}`;
  }
  if (value >= 10) {
    return `${value.toFixed(1)} ${unit}`;
  }
  return `${value.toFixed(2)} ${unit}`;
}

export function roomFillPercent(value: number, maxRooms: number): number {
  if (maxRooms <= 0) {
    return 0;
  }
  return Math.min(100, Math.round((value / maxRooms) * 100));
}
