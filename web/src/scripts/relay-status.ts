import Alpine from "alpinejs";

import { formatBytes, formatDuration, formatLocalTimestamp } from "../lib/format";
import type { RelayStatus } from "../lib/relay-status";

declare global {
  interface Window {
    Alpine: typeof Alpine;
  }
}

type RelayStatusState = {
  relay: RelayStatus | null;
  error: string;
  loading: boolean;
  refreshing: boolean;
  autoRefresh: boolean;
  lastUpdated: string;
  rawStatus: string;
  refreshHandle: number | undefined;
  init: () => Promise<void>;
  teardown: () => void;
  refresh: () => Promise<void>;
  syncAutoRefresh: () => void;
  startPolling: () => void;
  stopPolling: () => void;
  roomFill: (value: number) => string;
  bytes: (value: number) => string;
  uptime: (value: number) => string;
};

function relayStatusPanel(): RelayStatusState {
  return {
    relay: null,
    error: "",
    loading: true,
    refreshing: false,
    autoRefresh: true,
    lastUpdated: "",
    rawStatus: "",
    refreshHandle: undefined,

    async init() {
      await this.refresh();
      this.startPolling();
      window.addEventListener("beforeunload", () => this.teardown(), { once: true });
    },

    teardown() {
      this.stopPolling();
    },

    async refresh() {
      this.refreshing = true;
      this.error = "";

      try {
        const response = await fetch("/status", {
          headers: { Accept: "application/json" },
          cache: "no-store",
        });
        if (!response.ok) {
          throw new Error(`HTTP ${response.status}`);
        }

        const relay = (await response.json()) as RelayStatus;
        this.relay = relay;
        this.rawStatus = JSON.stringify(relay, null, 2);
        this.lastUpdated = formatLocalTimestamp(new Date());
      } catch (error) {
        this.error = error instanceof Error ? error.message : "Unknown status fetch error";
      } finally {
        this.loading = false;
        this.refreshing = false;
      }
    },

    syncAutoRefresh() {
      if (this.autoRefresh) {
        this.startPolling();
        return;
      }

      this.stopPolling();
    },

    startPolling() {
      this.stopPolling();
      this.refreshHandle = window.setInterval(() => {
        void this.refresh();
      }, 5000);
    },

    stopPolling() {
      if (this.refreshHandle !== undefined) {
        window.clearInterval(this.refreshHandle);
      }
      this.refreshHandle = undefined;
    },

    roomFill(value: number) {
      if (!this.relay || this.relay.limits.maxRooms <= 0) {
        return "0%";
      }

      const percent = Math.min(100, (value / this.relay.limits.maxRooms) * 100);
      return `${Math.round(percent)}%`;
    },

    bytes(value: number) {
      return formatBytes(value);
    },

    uptime(value: number) {
      return formatDuration(value);
    },
  };
}

window.Alpine = Alpine;
Alpine.data("relayStatusPanel", relayStatusPanel);
Alpine.start();
