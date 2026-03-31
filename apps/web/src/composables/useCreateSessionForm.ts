import type { SessionDetail } from "@bore/contracts";
import { reactive, ref } from "vue";

import {
  ApiClientError,
  type BoreApiClient,
  createBoreApiClient,
} from "../lib/api";
import {
  apiIssuesToFieldErrors,
  createDefaultSessionFormValues,
  type FieldErrors,
  prepareCreateSessionRequest,
} from "../lib/forms";

function toErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : "unable to create session";
}

export function useCreateSessionForm(
  client: BoreApiClient = createBoreApiClient(),
) {
  const form = reactive(createDefaultSessionFormValues());
  const createdSession = ref<SessionDetail | null>(null);
  const submitting = ref(false);
  const submitError = ref<string | null>(null);
  const fieldErrors = ref<FieldErrors>({});

  async function submit() {
    submitError.value = null;
    fieldErrors.value = {};

    const prepared = prepareCreateSessionRequest({ ...form });

    if (!prepared.success) {
      createdSession.value = null;
      fieldErrors.value = prepared.fieldErrors;
      return false;
    }

    submitting.value = true;

    try {
      createdSession.value = await client.createSession(prepared.data);
      return true;
    } catch (error) {
      createdSession.value = null;
      submitError.value = toErrorMessage(error);

      if (error instanceof ApiClientError) {
        fieldErrors.value = apiIssuesToFieldErrors(error.payload?.error.issues);
      }

      return false;
    } finally {
      submitting.value = false;
    }
  }

  return {
    form,
    createdSession,
    submitting,
    submitError,
    fieldErrors,
    submit,
  };
}
