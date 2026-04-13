<script setup lang="ts">
import type { SessionDetail } from "@bore/contracts";
import { computed, ref } from "vue";

import { useCreateSessionForm } from "../composables/useCreateSessionForm";
import { useTransferEngine } from "../composables/useTransferEngine";
import { buildReceivePath } from "../lib/routes";

const { createdSession, fieldErrors, form, submit, submitError, submitting } =
  useCreateSessionForm();

const engine = useTransferEngine();

const selectedFile = ref<File | null>(null);

const receivePath = computed(() =>
  createdSession.value
    ? buildReceivePath(createdSession.value.code)
    : "/receive",
);

const numberFormatter = new Intl.NumberFormat();

function formatBytes(sizeBytes: number) {
  if (sizeBytes < 1024) return `${numberFormatter.format(sizeBytes)} B`;
  if (sizeBytes < 1024 * 1024)
    return `${numberFormatter.format(Math.round(sizeBytes / 1024))} KB`;
  return `${numberFormatter.format(Math.round(sizeBytes / (1024 * 1024)))} MB`;
}

function handleFileSelect(event: Event) {
  const target = event.target as HTMLInputElement;
  const file = target.files?.[0];

  if (!file) {
    selectedFile.value = null;
    return;
  }

  selectedFile.value = file;
  form.fileName = file.name;
  form.sizeBytes = String(file.size);
  form.mimeType = file.type || "";
}

async function handleCreateAndWait() {
  const ok = await submit();

  if (!ok || !createdSession.value || !selectedFile.value) return;

  // Session created, now wait for receiver to join, then start transfer
  await engine.startSend({
    sessionCode: createdSession.value.code,
    role: "sender",
    file: selectedFile.value,
  });
}

const showTransferUI = computed(
  () => createdSession.value && engine.state.value !== "idle",
);

const stateLabel = computed(() => {
  switch (engine.state.value) {
    case "connecting":
      return "Connecting to relay...";
    case "waiting_peer":
      return "Waiting for receiver to connect...";
    case "key_exchange":
      return "Exchanging encryption keys...";
    case "transferring":
      return "Sending encrypted file...";
    case "verifying":
      return "Waiting for integrity verification...";
    case "completed":
      return "Transfer completed";
    case "failed":
      return "Transfer failed";
    default:
      return "";
  }
});
</script>

<template>
  <section class="panel stack">
    <div class="panel-header">
      <div>
        <p class="eyebrow">Encrypted file transfer</p>
        <h2>Send a file</h2>
      </div>
      <span class="badge">v2 relay</span>
    </div>

    <p>
      Select a file and create a session. Share the rendezvous code with the
      receiver. The file is encrypted end-to-end before it leaves your browser.
    </p>

    <form class="stack" @submit.prevent="handleCreateAndWait">
      <label class="field">
        <span>Choose file</span>
        <input type="file" name="file" @change="handleFileSelect" />
      </label>

      <div v-if="selectedFile" class="grid">
        <article class="metric">
          <span class="metric-label">File</span>
          <strong>{{ selectedFile.name }}</strong>
        </article>
        <article class="metric">
          <span class="metric-label">Size</span>
          <strong>{{ formatBytes(selectedFile.size) }}</strong>
        </article>
        <article class="metric">
          <span class="metric-label">Type</span>
          <strong>{{ selectedFile.type || "unknown" }}</strong>
        </article>
      </div>

      <div class="field-grid field-grid-two-up">
        <label class="field">
          <span>Your display name</span>
          <input
            v-model="form.senderDisplayName"
            name="senderDisplayName"
            placeholder="Sender"
          />
          <small v-if="fieldErrors.senderDisplayName" class="field-error">
            {{ fieldErrors.senderDisplayName }}
          </small>
        </label>

        <label class="field">
          <span>Expiry window (minutes)</span>
          <input
            v-model="form.expiresInMinutes"
            inputmode="numeric"
            name="expiresInMinutes"
            placeholder="15"
          />
          <small v-if="fieldErrors.expiresInMinutes" class="field-error">
            {{ fieldErrors.expiresInMinutes }}
          </small>
        </label>
      </div>

      <small v-if="fieldErrors.form" class="field-error">{{ fieldErrors.form }}</small>
      <p v-if="submitError" class="status-error">{{ submitError }}</p>

      <div class="action-row">
        <button
          class="button button-accent"
          type="submit"
          :disabled="submitting || !selectedFile || showTransferUI"
        >
          {{ submitting ? "Creating session..." : "Create session and send" }}
        </button>
      </div>
    </form>
  </section>

  <section v-if="createdSession" class="panel stack">
    <div class="panel-header">
      <div>
        <p class="eyebrow">Session active</p>
        <h2>{{ createdSession.code }}</h2>
      </div>
      <a class="button" :href="receivePath" target="_blank">Share receive link</a>
    </div>

    <p>
      Share this code with the receiver:
      <strong class="code-display">{{ createdSession.code }}</strong>
    </p>

    <p>
      Or share this link:
      <a :href="receivePath"><code>{{ receivePath }}</code></a>
    </p>
  </section>

  <section v-if="showTransferUI" class="panel stack">
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
        <span class="metric-label">Sent</span>
        <strong>{{ formatBytes(engine.bytesSent.value) }}</strong>
      </article>
      <article class="metric">
        <span class="metric-label">Total</span>
        <strong>{{ formatBytes(engine.totalBytes.value) }}</strong>
      </article>
    </div>

    <p v-if="engine.error.value" class="status-error">{{ engine.error.value }}</p>
    <p v-if="engine.state.value === 'completed'" class="status-success">
      File sent and verified. The receiver has your file.
    </p>
  </section>
</template>
