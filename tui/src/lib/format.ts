export function formatDurationSeconds(totalSeconds: number): string {
  if (!Number.isFinite(totalSeconds) || totalSeconds <= 0) {
    return "0s";
  }

  let remaining = Math.floor(totalSeconds);
  const days = Math.floor(remaining / 86_400);
  remaining -= days * 86_400;
  const hours = Math.floor(remaining / 3_600);
  remaining -= hours * 3_600;
  const minutes = Math.floor(remaining / 60);
  remaining -= minutes * 60;

  const parts: string[] = [];
  if (days > 0) {
    parts.push(`${days}d`);
  }
  if (hours > 0) {
    parts.push(`${hours}h`);
  }
  if (minutes > 0) {
    parts.push(`${minutes}m`);
  }
  if (remaining > 0 || parts.length === 0) {
    parts.push(`${remaining}s`);
  }

  return parts.slice(0, 2).join(" ");
}

export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) {
    return "0 B";
  }

  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  let value = bytes;
  let unitIndex = 0;

  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024;
    unitIndex += 1;
  }

  const precision = value >= 10 || unitIndex === 0 ? 0 : 1;
  return `${value.toFixed(precision)} ${units[unitIndex]}`;
}

export function formatPercent(numerator: number, denominator: number): string {
  if (
    !Number.isFinite(numerator) ||
    !Number.isFinite(denominator) ||
    denominator <= 0
  ) {
    return "0%";
  }

  const percent = (numerator / denominator) * 100;
  return `${percent.toFixed(percent >= 10 ? 0 : 1)}%`;
}

export function formatClock(timestamp: number | null): string {
  if (!timestamp) {
    return "never";
  }

  return new Intl.DateTimeFormat("en-US", {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  }).format(new Date(timestamp));
}

export function formatAgeMs(timestamp: number | null, now: number): string {
  if (!timestamp) {
    return "never";
  }

  const deltaMs = Math.max(0, now - timestamp);
  return formatDurationSeconds(deltaMs / 1000);
}

export function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

export function makeGauge(value: number, max: number, width = 18): string {
  if (width <= 0) {
    return "";
  }

  const safeMax = max > 0 ? max : 0;
  const ratio = safeMax > 0 ? clamp(value / safeMax, 0, 1) : 0;
  const filled = Math.round(ratio * width);
  return `${"#".repeat(filled)}${".".repeat(width - filled)}`;
}
