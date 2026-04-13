<script setup lang="ts">
import { computed, onMounted, ref } from "vue";

import { useJoinSession } from "../composables/useJoinSession";
import { useTransferEngine } from "../composables/useTransferEngine";

const props = defineProps<{
  code: string;
}>();

const {
  canJoin,
  fieldErrors,
  form,
  joinError,
  joining,
  loadError,
  loadingSession,
  loadSession,
  session,
  submitJoin,
} = useJoinSession(props.code);

const engine = useTransferEngine();
const joinedAndWaiting = ref(false);

const numberFormatter = new Intl.NumberFormat();

function formatBytes(sizeBytes: number) {
  if (sizeBytes < 1024) return `${numberFormatter.format(sizeBytes)} B`;
  if (sizeBytes < 1024 * 1024)
    return `${numberFormatter.format(Math.round(sizeBytes / 1024))} KB`;
  return `${numberFormatter.format(Math.round(sizeBytes / (1024 * 1024)))} MB`;
}

function formatTimestamp(timestamp?: string) {
  if (!timestamp) return "pending";

  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(timestamp));
}

async function handleJoinAndReceive() {
  const ok = await submitJoin();

  if (!ok || !session.value) return;

  joinedAndWaiting.value = true;

  await engine.startReceive({
    sessionCode: props.code,
    role: "receiver",
    expectedFileSize: session.value.file.sizeBytes,
    expectedFileName: session.value.file.name,
    expectedChecksum: session.value.file.checksumSha256,
  });
}

function handleDownload() {
  if (!session.value) return;
  engine.downloadReceivedFile(session.value.file.name);
}

const showTransferUI = computed(
  () =>
    joinedAndWaiting.value ||
    (session.value?.status === "ready" && engine.state.value !== "idle") ||
    (session.value?.status === "transferring" && engine.state.value !== "idle"),
);

const alreadyReady = computed(
  () =>
    session.value &&
    (session.value.status === "ready" ||
      session.value.status === "transferring") &&
    !canJoin.value &&
    !joinedAndWaiting.value &&
    engine.state.value === "idle",
);

async function handleReconnect() {
  if (!session.value) return;

  joinedAndWaiting.value = true;

  await engine.startReceive({
    sessionCode: props.code,
    role: "receiver",
    expectedFileSize: session.value.file.sizeBytes,
    expectedFileName: session.value.file.name,
    expectedChecksum: session.value.file.checksumSha256,
  });
}

const stateLabel = computed(() => {
  switch (engine.state.value) {
    case "connecting":
      return "Connecting to relay...";
    case "waiting_peer":
      return "Waiting for sender to connect...";
    case "key_exchange":
      return "Exchanging encryption keys...";
    case "transferring":
      return "Receiving encrypted file...";
    case "verifying":
      return "Verifying file integrity...";
    case "completed":
      return "Transfer completed";
    case "failed":
      return "Transfer failed";
    default:
      return "";
  }
});

onMounted(() => {
  void loadSession();
});
</script>

<template>
  <section class="panel stack">
    <div class="panel-header">
      <div>
        <p class="eyebrow">Encrypted file transfer</p>
        <h2>Receive: <code>{{ code }}</code></h2>
      </div>
      <button class="button" type="button" @click="loadSession">
        Refresh
      </button>
    </div>

    <p v-if="loadingSession">Loading session details...</p>
    <p v-else-if="loadError" class="status-error">{{ loadError }}</p>

    <template v-else-if="session">
      <div class="grid">
        <article class="metric">
          <span class="metric-label">Status</span>
          <strong>{{ session.status }}</strong>
        </article>
        <article class="metric">
          <span class="metric-label">File</span>
          <strong>{{ session.file.name }}</strong>
        </article>
        <article class="metric">
          <span class="metric-label">Size</span>
          <strong>{{ formatBytes(session.file.sizeBytes) }}</strong>
        </article>
        <article class="metric">
          <span class="metric-label">Expires</span>
          <strong>{{ formatTimestamp(session.expiresAt) }}</strong>
        </article>
      </div>

      <!-- Join form for new receivers -->
      <form v-if="canJoin" class="panel panel-subtle stack" @submit.prevent="handleJoinAndReceive">
        <div>
          <p class="eyebrow">Join and receive</p>
          <h3>This file is waiting for you</h3>
        </div>

        <label class="field">
          <span>Your display name</span>
          <input v-model="form.displayName" name="displayName" placeholder="Receiver" />
          <small v-if="fieldErrors.displayName" class="field-error">
            {{ fieldErrors.displayName }}
          </small>
        </label>

        <small v-if="fieldErrors.code" class="field-error">{{ fieldErrors.code }}</small>
        <small v-if="fieldErrors.form" class="field-error">{{ fieldErrors.form }}</small>
        <p v-if="joinError" class="status-error">{{ joinError }}</p>

        <div class="action-row">
          <button class="button button-accent" type="submit" :disabled="joining">
            {{ joining ? "Joining..." : "Join and start receiving" }}
          </button>
        </div>
      </form>

      <!-- Reconnect button for already-joined sessions -->
      <section v-else-if="alreadyReady" class="panel panel-subtle stack">
        <div>
          <p class="eyebrow">Receiver attached</p>
          <h3>Ready to receive</h3>
        </div>
        <p>You have already joined this session. Connect to the relay to start receiving the file.</p>
        <div class="action-row">
          <button class="button button-accent" type="button" @click="handleReconnect">
            Connect and receive
          </button>
        </div>
      </section>

      <!-- Transfer progress -->
      <section v-if="showTransferUI" class="panel panel-subtle stack">
        <div class="panel-header">
          <div>
            <p class="eyebrow">Transfer progress</p>
            <h2>{{ stateLabel }}</h2>
          </div>
          <button
            v-if="engine.state.value !== 'completed' && engine.state.value !== 'failed'"
            class="button"
            type="button"
            @click="engine.abort"
          >
            Cancel
          </button>
        </div>

        <div v-if="engine.state.value === 'transferring' || engine.state.value === 'verifying' || engine.state.value === 'completed'" class="progress-bar-container">
          <div class="progress-bar" :style="{ width: engine.progress.value + '%' }"></div>
          <span class="progress-label">{{ engine.progress.value }}%</span>
        </div>

        <div class="grid">
          <article class="metric">
            <span class="metric-label">Received</span>
            <strong>{{ formatBytes(engine.bytesReceived.value) }}</strong>
          </article>
          <article class="metric">
            <span class="metric-label">Total</span>
            <strong>{{ formatBytes(engine.totalBytes.value) }}</strong>
          </article>
        </div>

        <p v-if="engine.error.value" class="status-error">{{ engine.error.value }}</p>

        <div v-if="engine.state.value === 'completed'" class="stack">
          <p class="status-success">
            File received and integrity verified.
          </p>
          <div class="action-row">
            <button class="button button-accent" type="button" @click="handleDownload">
              Download {{ session.file.name }}
            </button>
          </div>
        </div>
      </section>

      <!-- Session completed/failed without transfer UI -->
      <section
        v-else-if="session.status === 'completed'"
        class="panel panel-subtle stack"
      >
        <p class="eyebrow">Transfer completed</p>
        <h3>This session has already completed.</h3>
      </section>

      <section
        v-else-if="session.status === 'failed'"
        class="panel panel-subtle stack"
      >
        <p class="eyebrow">Transfer failed</p>
        <h3>This session has failed.</h3>
      </section>

      <section
        v-else-if="session.status === 'expired'"
        class="panel panel-subtle stack"
      >
        <p class="eyebrow">Session expired</p>
        <h3>This session has expired.</h3>
      </section>
    </template>
  </section>
</template>
