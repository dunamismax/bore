import {
  formatAgeMs,
  formatBytes,
  formatClock,
  formatDurationSeconds,
  formatPercent,
  makeGauge,
} from "./format.ts";
import type { RelayStatus } from "./status.ts";

export interface DashboardState {
  relayURL: string;
  refreshIntervalMs: number;
  timeoutMs: number;
  loading: boolean;
  lastError: string | null;
  lastAttemptAt: number | null;
  lastSuccessAt: number | null;
  status: RelayStatus | null;
}

export interface DashboardViewModel {
  header: string;
  alertVisible: boolean;
  alertTitle: string;
  alertBody: string;
  overview: string;
  rooms: string;
  transport: string;
  limits: string;
  footer: string;
}

function directInferred(status: RelayStatus): number {
  return Math.max(
    0,
    status.transport.signalExchanges - status.transport.roomsRelayed,
  );
}

export function buildDashboardViewModel(
  state: DashboardState,
  now = Date.now(),
): DashboardViewModel {
  const status = state.status;
  const loadingSuffix = state.loading ? " | refreshing now" : "";
  const header = [
    `bore relay operator console | ${state.relayURL}`,
    `refresh ${formatDurationSeconds(state.refreshIntervalMs / 1000)} | timeout ${formatDurationSeconds(state.timeoutMs / 1000)}${loadingSuffix}`,
  ].join("\n");

  const alertVisible = Boolean(state.lastError);
  const alertTitle = status ? "stale snapshot" : "relay unavailable";
  const alertBody = state.lastError
    ? status
      ? [
          state.lastError,
          `showing last good /status snapshot from ${formatClock(state.lastSuccessAt)} (${formatAgeMs(state.lastSuccessAt, now)} stale).`,
        ].join("\n")
      : [
          state.lastError,
          "no valid /status payload yet. check the relay URL, reachability, or /healthz.",
        ].join("\n")
    : "";

  const overview = status
    ? [
        `service    ${status.service}`,
        `status     ${status.status}`,
        `uptime     ${formatDurationSeconds(status.uptimeSeconds)}`,
        `last ok    ${formatClock(state.lastSuccessAt)} (${formatAgeMs(state.lastSuccessAt, now)} ago)`,
      ].join("\n")
    : [
        "service    waiting for /status",
        "status     unavailable",
        `attempted  ${formatClock(state.lastAttemptAt)}`,
        "uptime     unknown",
      ].join("\n");

  const rooms = status
    ? [
        `waiting  [${makeGauge(status.rooms.waiting, status.limits.maxRooms)}] ${status.rooms.waiting}/${status.limits.maxRooms}`,
        `active   [${makeGauge(status.rooms.active, status.limits.maxRooms)}] ${status.rooms.active}/${status.limits.maxRooms}`,
        `total    [${makeGauge(status.rooms.total, status.limits.maxRooms)}] ${status.rooms.total}/${status.limits.maxRooms}`,
        `capacity ${formatPercent(status.rooms.total, status.limits.maxRooms)}`,
      ].join("\n")
    : [
        "waiting  [..................] ?/?",
        "active   [..................] ?/?",
        "total    [..................] ?/?",
        "capacity unknown",
      ].join("\n");

  const transport = status
    ? [
        `direct inferred  ${directInferred(status)}`,
        `relayed rooms    ${status.transport.roomsRelayed}`,
        `direct share     ${formatPercent(directInferred(status), status.transport.signalExchanges)}`,
        `signaling        ${status.transport.signalingStarted} started | ${status.transport.signalExchanges} exchanged`,
        `relay traffic    ${formatBytes(status.transport.bytesRelayed)} | ${status.transport.framesRelayed} frames`,
      ].join("\n")
    : [
        "direct inferred  unknown",
        "relayed rooms    unknown",
        "direct share     unknown",
        "signaling        unknown",
        "relay traffic    unknown",
      ].join("\n");

  const limits = status
    ? [
        `max rooms     ${status.limits.maxRooms}`,
        `room ttl      ${formatDurationSeconds(status.limits.roomTTLSeconds)}`,
        `reap interval ${formatDurationSeconds(status.limits.reapIntervalSeconds)}`,
        `max ws msg    ${formatBytes(status.limits.maxMessageSizeBytes)}`,
      ].join("\n")
    : [
        "max rooms     unknown",
        "room ttl      unknown",
        "reap interval unknown",
        "max ws msg    unknown",
      ].join("\n");

  const footer =
    "r refresh now | + slower | - faster | q quit | data path: relay /status only";

  return {
    header,
    alertVisible,
    alertTitle,
    alertBody,
    overview,
    rooms,
    transport,
    limits,
    footer,
  };
}
