import { computed, reactive, ref } from "vue";

import {
  ApiClientError,
  type BoreApiClient,
  createBoreApiClient,
} from "../lib/api";
import {
  apiIssuesToFieldErrors,
  createDefaultJoinFormValues,
  type FieldErrors,
  prepareJoinSessionRequest,
} from "../lib/forms";

function toErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : "unable to load session";
}

export function useJoinSession(
  code: string,
  client: BoreApiClient = createBoreApiClient(),
) {
  const form = reactive(createDefaultJoinFormValues());
  const session = ref<Awaited<ReturnType<BoreApiClient["getSession"]>> | null>(
    null,
  );
  const loadingSession = ref(false);
  const loadError = ref<string | null>(null);
  const joining = ref(false);
  const joinError = ref<string | null>(null);
  const fieldErrors = ref<FieldErrors>({});

  const canJoin = computed(() => session.value?.status === "waiting_receiver");

  async function loadSession() {
    loadingSession.value = true;
    loadError.value = null;

    try {
      session.value = await client.getSession(code);
      return true;
    } catch (error) {
      session.value = null;
      loadError.value = toErrorMessage(error);
      return false;
    } finally {
      loadingSession.value = false;
    }
  }

  async function submitJoin() {
    joinError.value = null;
    fieldErrors.value = {};

    const prepared = prepareJoinSessionRequest(code, { ...form });

    if (!prepared.success) {
      fieldErrors.value = prepared.fieldErrors;
      return false;
    }

    joining.value = true;

    try {
      session.value = await client.joinSession(code, prepared.data);
      return true;
    } catch (error) {
      joinError.value = toErrorMessage(error);

      if (error instanceof ApiClientError) {
        fieldErrors.value = apiIssuesToFieldErrors(error.payload?.error.issues);
      }

      return false;
    } finally {
      joining.value = false;
    }
  }

  return {
    form,
    session,
    loadingSession,
    loadError,
    joining,
    joinError,
    fieldErrors,
    canJoin,
    loadSession,
    submitJoin,
  };
}
