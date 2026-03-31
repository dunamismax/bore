<script setup lang="ts">
import { computed } from "vue";

import { useCreateSessionForm } from "../composables/useCreateSessionForm";
import { buildReceivePath } from "../lib/routes";

const { createdSession, fieldErrors, form, submit, submitError, submitting } =
  useCreateSessionForm();

const receivePath = computed(() =>
  createdSession.value
    ? buildReceivePath(createdSession.value.code)
    : "/receive",
);

const numberFormatter = new Intl.NumberFormat();

function formatBytes(sizeBytes: number) {
  return `${numberFormatter.format(sizeBytes)} bytes`;
}
</script>

<template>
  <section class="panel stack">
    <div class="panel-header">
      <div>
        <p class="eyebrow">Typed send flow</p>
        <h2>Create a transfer session</h2>
      </div>
      <span class="badge">v2 API</span>
    </div>

    <p>
      This shell now creates a real PostgreSQL-backed session through the Elysia API.
      Transfer streaming still lands later, but the browser can already allocate a
      rendezvous code with typed validation and visible failure states.
    </p>

    <form class="stack" @submit.prevent="submit">
      <div class="field-grid field-grid-two-up">
        <label class="field">
          <span>File name</span>
          <input v-model="form.fileName" name="fileName" placeholder="report.pdf" />
          <small v-if="fieldErrors['file.name']" class="field-error">
            {{ fieldErrors["file.name"] }}
          </small>
        </label>

        <label class="field">
          <span>Size in bytes</span>
          <input
            v-model="form.sizeBytes"
            inputmode="numeric"
            name="sizeBytes"
            placeholder="58213"
          />
          <small v-if="fieldErrors['file.sizeBytes']" class="field-error">
            {{ fieldErrors["file.sizeBytes"] }}
          </small>
        </label>
      </div>

      <div class="field-grid field-grid-two-up">
        <label class="field">
          <span>MIME type</span>
          <input
            v-model="form.mimeType"
            name="mimeType"
            placeholder="application/pdf"
          />
          <small v-if="fieldErrors['file.mimeType']" class="field-error">
            {{ fieldErrors["file.mimeType"] }}
          </small>
        </label>

        <label class="field">
          <span>SHA-256 checksum</span>
          <input
            v-model="form.checksumSha256"
            name="checksumSha256"
            placeholder="optional 64-character digest"
          />
          <small v-if="fieldErrors['file.checksumSha256']" class="field-error">
            {{ fieldErrors["file.checksumSha256"] }}
          </small>
        </label>
      </div>

      <div class="field-grid field-grid-two-up">
        <label class="field">
          <span>Sender display name</span>
          <input
            v-model="form.senderDisplayName"
            name="senderDisplayName"
            placeholder="Stephen"
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
        <button class="button button-accent" type="submit" :disabled="submitting">
          {{ submitting ? "Creating session..." : "Create session" }}
        </button>
        <span class="muted">Shared request and response schemas come from <code>@bore/contracts</code>.</span>
      </div>
    </form>
  </section>

  <section v-if="createdSession" class="panel stack">
    <div class="panel-header">
      <div>
        <p class="eyebrow">Session created</p>
        <h2>{{ createdSession.code }}</h2>
      </div>
      <a class="button" :href="receivePath">Open receive shell</a>
    </div>

    <div class="grid">
      <article class="metric">
        <span class="metric-label">Status</span>
        <strong>{{ createdSession.status }}</strong>
      </article>
      <article class="metric">
        <span class="metric-label">File</span>
        <strong>{{ createdSession.file.name }}</strong>
      </article>
      <article class="metric">
        <span class="metric-label">Size</span>
        <strong>{{ formatBytes(createdSession.file.sizeBytes) }}</strong>
      </article>
      <article class="metric">
        <span class="metric-label">Events</span>
        <strong>{{ createdSession.events.length }}</strong>
      </article>
    </div>

    <p class="muted">
      Receiver route: <a :href="receivePath"><code>{{ receivePath }}</code></a>
    </p>
  </section>
</template>
