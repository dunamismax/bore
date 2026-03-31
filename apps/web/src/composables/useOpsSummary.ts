import type { OperatorSummaryPayload } from "@bore/contracts";
import { ref } from "vue";

import { type BoreApiClient, createBoreApiClient } from "../lib/api";

function toErrorMessage(error: unknown) {
  return error instanceof Error
    ? error.message
    : "unable to load operator summary";
}

export function useOpsSummary(client: BoreApiClient = createBoreApiClient()) {
  const summary = ref<OperatorSummaryPayload | null>(null);
  const loading = ref(false);
  const error = ref<string | null>(null);

  async function refresh() {
    loading.value = true;
    error.value = null;

    try {
      summary.value = await client.getOperatorSummary();
      return true;
    } catch (caughtError) {
      summary.value = null;
      error.value = toErrorMessage(caughtError);
      return false;
    } finally {
      loading.value = false;
    }
  }

  return {
    summary,
    loading,
    error,
    refresh,
  };
}
