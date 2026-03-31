<script setup lang="ts">
import {
  type HealthPayload,
  healthPayloadSchema,
  type ReadinessPayload,
  readinessPayloadSchema,
} from "@bore/contracts";
import { onMounted, ref } from "vue";

const health = ref<HealthPayload | null>(null);
const readiness = ref<ReadinessPayload | null>(null);
const error = ref<string | null>(null);
const loading = ref(true);

async function refresh() {
  loading.value = true;
  error.value = null;

  try {
    const [healthResponse, readinessResponse] = await Promise.all([
      fetch("/api/health"),
      fetch("/api/readiness"),
    ]);

    const healthJson = await healthResponse.json();
    const readinessJson = await readinessResponse.json();

    health.value = healthPayloadSchema.parse(healthJson);
    readiness.value = readinessPayloadSchema.parse(readinessJson);
  } catch (caughtError) {
    error.value =
      caughtError instanceof Error
        ? caughtError.message
        : "unable to load status";
  } finally {
    loading.value = false;
  }
}

onMounted(() => {
  void refresh();
});
</script>

<template>
  <section class="panel">
    <div class="panel-header">
      <h2>Runtime snapshot</h2>
      <button class="button" type="button" @click="refresh">Refresh</button>
    </div>

    <p v-if="loading">Loading the v2 API health checks through Caddy.</p>
    <p v-else-if="error">{{ error }}</p>
    <div v-else class="grid">
      <article class="metric">
        <span class="metric-label">Service</span>
        <strong>{{ health?.service }}</strong>
      </article>
      <article class="metric">
        <span class="metric-label">Health</span>
        <strong>{{ health?.status }}</strong>
      </article>
      <article class="metric">
        <span class="metric-label">Readiness</span>
        <strong>{{ readiness?.status }}</strong>
      </article>
      <article class="metric">
        <span class="metric-label">Version</span>
        <strong>{{ health?.version }}</strong>
      </article>
    </div>

    <ul v-if="readiness" class="check-list">
      <li v-for="check in readiness.checks" :key="check.name">
        <strong>{{ check.name }}</strong>
        <span>{{ check.status }}</span>
        <span v-if="check.latencyMs !== undefined">{{ check.latencyMs }} ms</span>
        <span v-if="check.detail">{{ check.detail }}</span>
      </li>
    </ul>
  </section>
</template>
