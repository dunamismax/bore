<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";

import {
  fetchRelayStatus,
  formatBytes,
  formatDuration,
  type RelayStatus,
  roomFillPercent,
} from "../lib/status";

const refreshIntervalMs = 2_000;

const error = ref<string | null>(null);
const isRefreshing = ref(false);
const status = ref<RelayStatus | null>(null);
let refreshTimer: number | undefined;

const directInferred = computed(() => {
  if (!status.value) {
    return 0;
  }
  return Math.max(
    0,
    status.value.transport.signalExchanges -
      status.value.transport.roomsRelayed,
  );
});

const directRate = computed(() => {
  if (!status.value || status.value.transport.signalExchanges <= 0) {
    return "n/a";
  }
  return `${Math.round(
    (directInferred.value / status.value.transport.signalExchanges) * 100,
  )}%`;
});

const roomCards = computed(() => {
  if (!status.value) {
    return [];
  }

  const maxRooms = status.value.limits.maxRooms;
  return [
    {
      label: "Waiting rooms",
      value: status.value.rooms.waiting,
      fill: roomFillPercent(status.value.rooms.waiting, maxRooms),
      tone: "primary",
    },
    {
      label: "Active rooms",
      value: status.value.rooms.active,
      fill: roomFillPercent(status.value.rooms.active, maxRooms),
      tone: "secondary",
    },
    {
      label: "Total rooms tracked",
      value: status.value.rooms.total,
      fill: roomFillPercent(status.value.rooms.total, maxRooms),
      tone: "neutral",
    },
  ];
});

async function refreshStatus() {
  isRefreshing.value = true;
  error.value = null;

  const controller = new AbortController();
  try {
    status.value = await fetchRelayStatus(controller.signal);
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : String(cause);
  } finally {
    controller.abort();
    isRefreshing.value = false;
  }
}

onMounted(async () => {
  await refreshStatus();
  refreshTimer = window.setInterval(() => {
    void refreshStatus();
  }, refreshIntervalMs);
});

onBeforeUnmount(() => {
  if (refreshTimer !== undefined) {
    window.clearInterval(refreshTimer);
  }
});
</script>

<template>
  <section class="status-shell">
    <div class="status-shell__header">
      <div>
        <p class="eyebrow">Live snapshot</p>
        <h2>Relay status</h2>
      </div>
      <div class="status-shell__meta">
        <span v-if="isRefreshing">Refreshing...</span>
        <span>Auto-refreshes every 2s</span>
      </div>
    </div>

    <div v-if="error" class="callout callout--danger">
      Status fetch failed: {{ error }}
    </div>

    <div v-else-if="!status" class="callout">
      Loading relay status from <code>/status</code>...
    </div>

    <div v-else class="status-grid">
      <div class="metric-grid">
        <article class="metric-card">
          <p class="eyebrow">Service</p>
          <p class="metric-card__value">{{ status.service }}</p>
          <p class="metric-card__note">Current relay identity from <code>/status</code>.</p>
        </article>
        <article class="metric-card">
          <p class="eyebrow">Status</p>
          <p class="metric-card__value">{{ status.status }}</p>
          <p class="metric-card__note">Expected steady state is <code>ok</code>.</p>
        </article>
        <article class="metric-card">
          <p class="eyebrow">Uptime</p>
          <p class="metric-card__value">{{ formatDuration(status.uptimeSeconds) }}</p>
          <p class="metric-card__note">Process uptime reported by the relay.</p>
        </article>
        <article class="metric-card">
          <p class="eyebrow">Max WS message</p>
          <p class="metric-card__value">
            {{ formatBytes(status.limits.maxMessageSizeBytes) }}
          </p>
          <p class="metric-card__note">Per-message transport cap enforced by the relay.</p>
        </article>
      </div>

      <div class="room-grid">
        <article v-for="card in roomCards" :key="card.label" class="metric-card">
          <div class="room-grid__topline">
            <div>
              <p class="eyebrow">{{ card.label }}</p>
              <p class="metric-card__value">{{ card.value }}</p>
            </div>
            <span class="metric-card__note">{{ card.fill }}%</span>
          </div>
          <div class="meter">
            <span :class="`meter__fill meter__fill--${card.tone}`" :style="{ width: `${card.fill}%` }" />
          </div>
        </article>
      </div>

      <div class="metric-grid">
        <article class="metric-card">
          <p class="eyebrow">Signaling started</p>
          <p class="metric-card__value">{{ status.transport.signalingStarted }}</p>
          <p class="metric-card__note">Peers that opened <code>/signal</code> for candidate exchange.</p>
        </article>
        <article class="metric-card">
          <p class="eyebrow">Signal exchanges</p>
          <p class="metric-card__value">{{ status.transport.signalExchanges }}</p>
          <p class="metric-card__note">Completed candidate exchanges coordinated by the relay.</p>
        </article>
        <article class="metric-card">
          <p class="eyebrow">Direct (inferred)</p>
          <p class="metric-card__value">{{ directInferred }}</p>
          <p class="metric-card__note">Signaled sessions that did not fall back. Direct rate: {{ directRate }}.</p>
        </article>
        <article class="metric-card">
          <p class="eyebrow">Rooms relayed</p>
          <p class="metric-card__value">{{ status.transport.roomsRelayed }}</p>
          <p class="metric-card__note">Transfers that used the relay as fallback transport.</p>
        </article>
        <article class="metric-card">
          <p class="eyebrow">Bytes relayed</p>
          <p class="metric-card__value">{{ formatBytes(status.transport.bytesRelayed) }}</p>
          <p class="metric-card__note">Encrypted bytes forwarded through fallback transport.</p>
        </article>
        <article class="metric-card">
          <p class="eyebrow">Frames relayed</p>
          <p class="metric-card__value">{{ status.transport.framesRelayed }}</p>
          <p class="metric-card__note">WebSocket frames forwarded without payload inspection.</p>
        </article>
      </div>

      <div class="metric-grid metric-grid--compact">
        <article class="metric-card">
          <p class="eyebrow">Room TTL</p>
          <p class="metric-card__value">{{ formatDuration(status.limits.roomTTLSeconds) }}</p>
        </article>
        <article class="metric-card">
          <p class="eyebrow">Reap interval</p>
          <p class="metric-card__value">{{ formatDuration(status.limits.reapIntervalSeconds) }}</p>
        </article>
        <article class="metric-card">
          <p class="eyebrow">Room cap</p>
          <p class="metric-card__value">{{ status.limits.maxRooms }}</p>
        </article>
      </div>

      <details class="raw-json">
        <summary>Raw <code>/status</code> payload</summary>
        <pre>{{ JSON.stringify(status, null, 2) }}</pre>
      </details>
    </div>
  </section>
</template>
