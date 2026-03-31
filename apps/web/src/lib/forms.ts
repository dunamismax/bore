import {
  type ApiError,
  type CreateSessionRequest,
  createSessionRequestSchema,
  type JoinSessionRequest,
  joinSessionRequestSchema,
  sessionRouteParamsSchema,
} from "@bore/contracts";

type IssueLike = {
  message: string;
  path?: PropertyKey[];
};

export type CreateSessionFormValues = {
  fileName: string;
  sizeBytes: string;
  mimeType: string;
  checksumSha256: string;
  senderDisplayName: string;
  expiresInMinutes: string;
};

export type JoinSessionFormValues = {
  displayName: string;
};

export type FieldErrors = Record<string, string>;

export function createDefaultSessionFormValues(): CreateSessionFormValues {
  return {
    fileName: "",
    sizeBytes: "",
    mimeType: "",
    checksumSha256: "",
    senderDisplayName: "",
    expiresInMinutes: "15",
  };
}

export function createDefaultJoinFormValues(): JoinSessionFormValues {
  return {
    displayName: "",
  };
}

function toOptionalString(value: string) {
  const trimmed = value.trim();

  return trimmed.length > 0 ? trimmed : undefined;
}

function toFieldKey(path: PropertyKey[] | undefined) {
  if (!path || path.length === 0) {
    return "form";
  }

  return path.map((segment) => String(segment)).join(".");
}

export function zodIssuesToFieldErrors(
  issues: readonly IssueLike[],
): FieldErrors {
  const errors: FieldErrors = {};

  for (const issue of issues) {
    const key = toFieldKey(issue.path);

    if (!errors[key]) {
      errors[key] = issue.message;
    }
  }

  return errors;
}

export function apiIssuesToFieldErrors(
  issues: ApiError["issues"] | undefined,
): FieldErrors {
  const errors: FieldErrors = {};

  for (const issue of issues ?? []) {
    const key = toFieldKey(issue.path);

    if (!errors[key]) {
      errors[key] = issue.message;
    }
  }

  return errors;
}

export function prepareCreateSessionRequest(values: CreateSessionFormValues):
  | {
      success: true;
      data: CreateSessionRequest;
    }
  | {
      success: false;
      fieldErrors: FieldErrors;
    } {
  const parsed = createSessionRequestSchema.safeParse({
    file: {
      name: values.fileName.trim(),
      sizeBytes: Number(values.sizeBytes),
      mimeType: toOptionalString(values.mimeType),
      checksumSha256: toOptionalString(values.checksumSha256),
    },
    senderDisplayName: toOptionalString(values.senderDisplayName),
    expiresInMinutes: Number(values.expiresInMinutes),
  });

  if (!parsed.success) {
    return {
      success: false,
      fieldErrors: zodIssuesToFieldErrors(parsed.error.issues),
    };
  }

  return {
    success: true,
    data: parsed.data,
  };
}

export function prepareJoinSessionRequest(
  code: string,
  values: JoinSessionFormValues,
):
  | {
      success: true;
      data: JoinSessionRequest;
    }
  | {
      success: false;
      fieldErrors: FieldErrors;
    } {
  const codeResult = sessionRouteParamsSchema.safeParse({ code });
  const payloadResult = joinSessionRequestSchema.safeParse({
    displayName: toOptionalString(values.displayName),
  });

  if (codeResult.success && payloadResult.success) {
    return {
      success: true,
      data: payloadResult.data,
    };
  }

  return {
    success: false,
    fieldErrors: {
      ...(codeResult.success
        ? {}
        : zodIssuesToFieldErrors(codeResult.error.issues)),
      ...(payloadResult.success
        ? {}
        : zodIssuesToFieldErrors(payloadResult.error.issues)),
    },
  };
}
