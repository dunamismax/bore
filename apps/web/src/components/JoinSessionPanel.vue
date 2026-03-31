<script setup lang="ts">
import { onMounted } from "vue";

import { useJoinSession } from "../composables/useJoinSession";

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

const numberFormatter = new Intl.NumberFormat();

function formatBytes(sizeBytes: number) {
  return `${numberFormatter.format(sizeBytes)} bytes`;
}

function formatTimestamp(timestamp?: string) {
  if (!timestamp) {
    return "pending";
  }

  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(timestamp));
}

onMounted(() => {
  void loadSession();
});
</script>

<template>
  <section class="panel stack">
    <div class="panel-header">
      <div>
        <p class="eyebrow">Typed receive flow</p>
        <h2>Join code: <code>{{ code }}</code></h2>
      </div>
      <button class="button" type="button" @click="loadSession">
        Refresh session
      </button>
    </div>

    <p>
      The browser now loads the real v2 session shell from the Elysia API. When a
      session is waiting for a receiver, this page can join it without falling back
      to untyped JSON or the legacy Go status path.
    </p>

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

      <div class="field-grid field-grid-two-up">
        <article class="panel panel-subtle stack">
          <h3>Participants</h3>
          <ul class="detail-list">
            <li v-for="participant in session.participants" :key="participant.role">
              <strong>{{ participant.role }}</strong>
              <span>{{ participant.displayName ?? "anonymous" }}</span>
              <span>{{ participant.status }}</span>
              <span>{{ formatTimestamp(participant.joinedAt) }}</span>
            </li>
          </ul>
        </article>

        <article class="panel panel-subtle stack">
          <h3>Recent events</h3>
          <ul class="detail-list">
            <li v-for="event in session.events" :key="event.id">
              <strong>{{ event.type }}</strong>
              <span>{{ event.actorRole ?? "system" }}</span>
              <span>{{ formatTimestamp(event.timestamp) }}</span>
            </li>
          </ul>
        </article>
      </div>

      <form v-if="canJoin" class="panel panel-subtle stack" @submit.prevent="submitJoin">
        <div>
          <p class="eyebrow">Join now</p>
          <h3>Attach as the receiver</h3>
        </div>

        <label class="field">
          <span>Receiver display name</span>
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
            {{ joining ? "Joining session..." : "Join session" }}
          </button>
          <span class="muted">Join requests reuse the shared contract package and API client.</span>
        </div>
      </form>

      <section v-else class="panel panel-subtle stack">
        <p class="eyebrow">Receiver status</p>
        <h3>Receiver already attached</h3>
        <p>
          This rendezvous code has moved past the waiting state. The page is still
          showing the live typed session snapshot from the v2 API.
        </p>
      </section>
    </template>
  </section>
</template>
