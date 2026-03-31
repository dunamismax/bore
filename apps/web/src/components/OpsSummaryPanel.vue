<script setup lang="ts">
import { onMounted } from "vue";

import { useOpsSummary } from "../composables/useOpsSummary";
import { buildReceivePath } from "../lib/routes";

const { error, loading, refresh, summary } = useOpsSummary();

const numberFormatter = new Intl.NumberFormat();

function formatBytes(sizeBytes: number) {
  return `${numberFormatter.format(sizeBytes)} bytes`;
}

function formatTimestamp(timestamp: string) {
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(timestamp));
}

onMounted(() => {
  void refresh();
});
</script>

<template>
  <section class="panel stack">
    <div class="panel-header">
      <div>
        <p class="eyebrow">v2 operator summary</p>
        <h2>Live session inventory</h2>
      </div>
      <button class="button" type="button" @click="refresh">Refresh summary</button>
    </div>

    <p>
      This operator view now renders directly from <code>/api/ops/summary</code> in
      the v2 API lane. It no longer needs the legacy Go relay <code>/status</code>
      contract to answer basic session questions.
    </p>

    <p v-if="loading">Loading session summary...</p>
    <p v-else-if="error" class="status-error">{{ error }}</p>

    <template v-else-if="summary">
      <div class="grid">
        <article class="metric">
          <span class="metric-label">Total</span>
          <strong>{{ summary.counts.total }}</strong>
        </article>
        <article class="metric">
          <span class="metric-label">Waiting</span>
          <strong>{{ summary.counts.waitingReceiver }}</strong>
        </article>
        <article class="metric">
          <span class="metric-label">Ready</span>
          <strong>{{ summary.counts.ready }}</strong>
        </article>
        <article class="metric">
          <span class="metric-label">Generated</span>
          <strong>{{ formatTimestamp(summary.generatedAt) }}</strong>
        </article>
      </div>

      <ul class="ops-list">
        <li v-for="entry in summary.sessions" :key="entry.id" class="ops-list-item">
          <div class="ops-list-main">
            <div>
              <p class="ops-code">
                <a :href="buildReceivePath(entry.code)">{{ entry.code }}</a>
              </p>
              <strong>{{ entry.fileName }}</strong>
              <p class="muted">{{ formatBytes(entry.fileSizeBytes) }}</p>
            </div>
            <div class="ops-list-meta">
              <span class="badge">{{ entry.status }}</span>
              <span>{{ formatTimestamp(entry.updatedAt) }}</span>
              <span v-if="entry.lastEventType">{{ entry.lastEventType }}</span>
            </div>
          </div>
        </li>
      </ul>
    </template>
  </section>
</template>
