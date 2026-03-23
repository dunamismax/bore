export function formatDuration(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds <= 0) {
    return "0s";
  }

  const units = [
    { label: "d", size: 60 * 60 * 24 },
    { label: "h", size: 60 * 60 },
    { label: "m", size: 60 },
    { label: "s", size: 1 },
  ];

  let remaining = Math.floor(seconds);
  const parts: string[] = [];

  for (const unit of units) {
    if (parts.length === 2) {
      break;
    }

    const amount = Math.floor(remaining / unit.size);
    if (amount === 0) {
      continue;
    }

    parts.push(`${amount}${unit.label}`);
    remaining -= amount * unit.size;
  }

  return parts.length > 0 ? parts.join(" ") : "0s";
}

export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) {
    return "0 B";
  }

  const units = ["B", "KB", "MB", "GB", "TB"];
  let value = bytes;
  let unit = units[0];

  for (const next of units) {
    unit = next;
    if (value < 1024 || next === units.at(-1)) {
      break;
    }
    value /= 1024;
  }

  const digits = value >= 100 ? 0 : value >= 10 ? 1 : 2;
  return `${value.toFixed(digits)} ${unit}`;
}

export function formatLocalTimestamp(value: Date): string {
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
    timeStyle: "medium",
  }).format(value);
}
