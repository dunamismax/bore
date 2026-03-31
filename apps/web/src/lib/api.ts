import {
  type ApiErrorPayload,
  apiErrorPayloadSchema,
  type CreateSessionRequest,
  createSessionRequestSchema,
  type HealthPayload,
  healthPayloadSchema,
  type JoinSessionRequest,
  joinSessionRequestSchema,
  type OperatorSummaryPayload,
  operatorSummaryPayloadSchema,
  type ReadinessPayload,
  readinessPayloadSchema,
  type SessionDetail,
  sessionDetailSchema,
  sessionRouteParamsSchema,
} from "@bore/contracts";

type FetchLike = (
  input: RequestInfo | URL,
  init?: RequestInit,
) => Promise<Response>;

type ApiClientOptions = {
  baseUrl?: string;
  fetch?: FetchLike;
};

type Schema<T> = {
  parse(input: unknown): T;
};

type RequestOptions<T> = {
  path: string;
  init?: RequestInit;
  schema: Schema<T>;
};

export class ApiClientError extends Error {
  readonly status: number;
  readonly payload?: ApiErrorPayload;

  constructor(status: number, message: string, payload?: ApiErrorPayload) {
    super(message);
    this.name = "ApiClientError";
    this.status = status;
    this.payload = payload;
  }
}

function buildUrl(path: string, baseUrl?: string) {
  if (!baseUrl) {
    return path;
  }

  return new URL(path, baseUrl).toString();
}

async function readJson(response: Response) {
  const text = await response.text();

  if (!text) {
    return null;
  }

  return JSON.parse(text) as unknown;
}

export function createBoreApiClient(options: ApiClientOptions = {}) {
  const fetcher = options.fetch ?? fetch;

  async function request<T>({ path, init, schema }: RequestOptions<T>) {
    const response = await fetcher(buildUrl(path, options.baseUrl), init);
    const payload = await readJson(response);

    if (!response.ok) {
      const parsedError = apiErrorPayloadSchema.safeParse(payload);

      if (parsedError.success) {
        throw new ApiClientError(
          response.status,
          parsedError.data.error.message,
          parsedError.data,
        );
      }

      throw new ApiClientError(
        response.status,
        `request failed with status ${response.status}`,
      );
    }

    return schema.parse(payload);
  }

  return {
    getHealth(): Promise<HealthPayload> {
      return request({
        path: "/api/health",
        schema: healthPayloadSchema,
      });
    },

    getReadiness(): Promise<ReadinessPayload> {
      return request({
        path: "/api/readiness",
        schema: readinessPayloadSchema,
      });
    },

    createSession(input: CreateSessionRequest): Promise<SessionDetail> {
      const payload = createSessionRequestSchema.parse(input);

      return request({
        path: "/api/sessions",
        init: {
          method: "POST",
          headers: {
            "content-type": "application/json",
          },
          body: JSON.stringify(payload),
        },
        schema: sessionDetailSchema,
      });
    },

    getSession(code: string): Promise<SessionDetail> {
      const params = sessionRouteParamsSchema.parse({ code });

      return request({
        path: `/api/sessions/${encodeURIComponent(params.code)}`,
        schema: sessionDetailSchema,
      });
    },

    joinSession(
      code: string,
      input: JoinSessionRequest,
    ): Promise<SessionDetail> {
      const params = sessionRouteParamsSchema.parse({ code });
      const payload = joinSessionRequestSchema.parse(input);

      return request({
        path: `/api/sessions/${encodeURIComponent(params.code)}/join`,
        init: {
          method: "POST",
          headers: {
            "content-type": "application/json",
          },
          body: JSON.stringify(payload),
        },
        schema: sessionDetailSchema,
      });
    },

    getOperatorSummary(): Promise<OperatorSummaryPayload> {
      return request({
        path: "/api/ops/summary",
        schema: operatorSummaryPayloadSchema,
      });
    },
  };
}

export type BoreApiClient = ReturnType<typeof createBoreApiClient>;
