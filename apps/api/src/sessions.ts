import { randomInt } from "node:crypto";
import {
  type CreateSessionRequest,
  type JoinSessionRequest,
  type OperatorSummaryPayload,
  operatorSummaryPayloadSchema,
  type SessionDetail,
  sessionDetailSchema,
} from "@bore/contracts";

import type { Sql } from "postgres";

const rendezvousWords = [
  "amber",
  "anchor",
  "apex",
  "aster",
  "atlas",
  "aurora",
  "banner",
  "bravo",
  "cable",
  "cinder",
  "cobalt",
  "comet",
  "delta",
  "drift",
  "ember",
  "falcon",
  "fjord",
  "flux",
  "forest",
  "frost",
  "glow",
  "harbor",
  "helix",
  "iris",
  "jade",
  "kepler",
  "lagoon",
  "lumen",
  "maple",
  "mesa",
  "meteor",
  "monarch",
  "nova",
  "onyx",
  "orbit",
  "otter",
  "phoenix",
  "pine",
  "plume",
  "prairie",
  "quartz",
  "quill",
  "raven",
  "ridge",
  "river",
  "rook",
  "sable",
  "sage",
  "shadow",
  "signal",
  "solstice",
  "spruce",
  "summit",
  "thunder",
  "timber",
  "topaz",
  "vector",
  "velvet",
  "vivid",
  "willow",
  "winter",
  "yonder",
  "zenith",
] as const;

type TimestampValue = Date | string;
type NumericValue = number | string;

type SessionRow = {
  id: string;
  code: string;
  status: string;
  created_at: TimestampValue;
  updated_at: TimestampValue;
  expires_at: TimestampValue;
};

type ParticipantRow = {
  role: string;
  status: string;
  display_name: string | null;
  joined_at: TimestampValue | null;
};

type FileRow = {
  file_name: string;
  file_size_bytes: NumericValue;
  mime_type: string | null;
  checksum_sha256: string | null;
};

type EventRow = {
  id: string;
  event_type: string;
  actor_role: string | null;
  created_at: TimestampValue;
  payload: unknown;
};

export class SessionNotFoundError extends Error {
  constructor(code: string) {
    super(`session not found for code ${code}`);
    this.name = "SessionNotFoundError";
  }
}

export class SessionConflictError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "SessionConflictError";
  }
}

export type SessionService = {
  createSession(input: CreateSessionRequest): Promise<SessionDetail>;
  joinSession(code: string, input: JoinSessionRequest): Promise<SessionDetail>;
  getSession(code: string): Promise<SessionDetail | null>;
  getOperatorSummary(): Promise<OperatorSummaryPayload>;
  startTransfer(code: string): Promise<SessionDetail>;
  completeTransfer(
    code: string,
    checksumSha256: string,
  ): Promise<SessionDetail>;
  failTransfer(code: string, reason: string): Promise<SessionDetail>;
  recordTransferProgress(
    code: string,
    bytesSent: number,
    totalBytes: number,
  ): Promise<void>;
};

function generateSessionCode() {
  return [0, 1, 2]
    .map(() => rendezvousWords[randomInt(rendezvousWords.length)])
    .join("-");
}

function toIsoString(value: TimestampValue) {
  return value instanceof Date ? value.toISOString() : value;
}

function toInteger(value: NumericValue) {
  return typeof value === "number" ? value : Number.parseInt(value, 10);
}

async function loadSessionDetailByCode(
  sql: Sql,
  code: string,
): Promise<SessionDetail | null> {
  const sessionRows = await sql<SessionRow[]>`
    select id, code, status, created_at, updated_at, expires_at
    from transfer_sessions
    where code = ${code}
  `;

  const sessionRow = sessionRows[0];

  if (!sessionRow) {
    return null;
  }

  const [participantRows, fileRows, eventRows] = await Promise.all([
    sql<ParticipantRow[]>`
      select role, status, display_name, joined_at
      from session_participants
      where session_id = ${sessionRow.id}
      order by case role when 'sender' then 0 else 1 end
    `,
    sql<FileRow[]>`
      select file_name, file_size_bytes, mime_type, checksum_sha256
      from session_files
      where session_id = ${sessionRow.id}
      limit 1
    `,
    sql<EventRow[]>`
      select id, event_type, actor_role, created_at, payload
      from session_events
      where session_id = ${sessionRow.id}
      order by created_at asc, id asc
    `,
  ]);

  const fileRow = fileRows[0];

  if (!fileRow) {
    throw new Error(`session ${sessionRow.id} is missing file metadata`);
  }

  return sessionDetailSchema.parse({
    id: sessionRow.id,
    code: sessionRow.code,
    status: sessionRow.status,
    createdAt: toIsoString(sessionRow.created_at),
    updatedAt: toIsoString(sessionRow.updated_at),
    expiresAt: toIsoString(sessionRow.expires_at),
    file: {
      name: fileRow.file_name,
      sizeBytes: toInteger(fileRow.file_size_bytes),
      mimeType: fileRow.mime_type ?? undefined,
      checksumSha256: fileRow.checksum_sha256 ?? undefined,
    },
    participants: participantRows.map((row) => ({
      role: row.role,
      status: row.status,
      displayName: row.display_name ?? undefined,
      joinedAt: row.joined_at ? toIsoString(row.joined_at) : undefined,
    })),
    events: eventRows.map((row) => ({
      id: row.id,
      type: row.event_type,
      actorRole: row.actor_role ?? undefined,
      timestamp: toIsoString(row.created_at),
      payload:
        row.payload &&
        typeof row.payload === "object" &&
        !Array.isArray(row.payload)
          ? row.payload
          : {},
    })),
  });
}

async function createSessionWithCode(
  sql: Sql,
  code: string,
  input: CreateSessionRequest,
) {
  return sql.begin(async (tx) => {
    await tx<SessionRow[]>`
      insert into transfer_sessions (code, status, expires_at)
      values (
        ${code},
        'waiting_receiver',
        now() + (${input.expiresInMinutes} * interval '1 minute')
      )
    `;

    const sessionRows = await tx<SessionRow[]>`
      select id, code, status, created_at, updated_at, expires_at
      from transfer_sessions
      where code = ${code}
      limit 1
    `;

    const sessionRow = sessionRows[0];

    if (!sessionRow) {
      throw new Error("failed to read newly created session");
    }

    await tx`
      insert into session_participants (
        session_id,
        role,
        status,
        display_name,
        joined_at
      )
      values (
        ${sessionRow.id},
        'sender',
        'joined',
        ${input.senderDisplayName ?? null},
        now()
      )
    `;

    await tx`
      insert into session_files (
        session_id,
        file_name,
        file_size_bytes,
        mime_type,
        checksum_sha256
      )
      values (
        ${sessionRow.id},
        ${input.file.name},
        ${input.file.sizeBytes},
        ${input.file.mimeType ?? null},
        ${input.file.checksumSha256 ?? null}
      )
    `;

    await tx`
      insert into session_events (session_id, event_type, actor_role, payload)
      values (
        ${sessionRow.id},
        'session_created',
        'sender',
        ${JSON.stringify({
          expiresInMinutes: input.expiresInMinutes,
          senderDisplayName: input.senderDisplayName ?? null,
        })}::jsonb
      )
    `;

    await tx`
      insert into session_events (session_id, event_type, actor_role, payload)
      values (
        ${sessionRow.id},
        'file_registered',
        'sender',
        ${JSON.stringify({
          name: input.file.name,
          sizeBytes: input.file.sizeBytes,
          mimeType: input.file.mimeType ?? null,
          checksumSha256: input.file.checksumSha256 ?? null,
        })}::jsonb
      )
    `;

    const detail = await loadSessionDetailByCode(tx, code);

    if (!detail) {
      throw new Error("failed to load newly created session detail");
    }

    return detail;
  });
}

export function createDatabaseSessionService(sql: Sql): SessionService {
  return {
    async createSession(input) {
      for (let attempt = 0; attempt < 5; attempt += 1) {
        const code = generateSessionCode();

        try {
          return await createSessionWithCode(sql, code, input);
        } catch (error) {
          if (
            typeof error === "object" &&
            error !== null &&
            "code" in error &&
            error.code === "23505"
          ) {
            continue;
          }

          throw error;
        }
      }

      throw new Error("unable to allocate a unique session code");
    },

    async joinSession(code, input) {
      return sql.begin(async (tx) => {
        const sessionRows = await tx<SessionRow[]>`
          select id, code, status, created_at, updated_at, expires_at
          from transfer_sessions
          where code = ${code}
          for update
        `;

        const sessionRow = sessionRows[0];

        if (!sessionRow) {
          throw new SessionNotFoundError(code);
        }

        if (new Date(sessionRow.expires_at).getTime() <= Date.now()) {
          await tx`
            update transfer_sessions
            set status = 'expired', updated_at = now()
            where id = ${sessionRow.id}
          `;

          throw new SessionConflictError("session has expired");
        }

        if (sessionRow.status !== "waiting_receiver") {
          throw new SessionConflictError(
            `session cannot be joined from state ${sessionRow.status}`,
          );
        }

        const existingReceiverRows = await tx<{ count: number }[]>`
          select count(*)::int as count
          from session_participants
          where session_id = ${sessionRow.id}
            and role = 'receiver'
            and status = 'joined'
        `;

        const existingReceiver = existingReceiverRows[0]?.count ?? 0;

        if (existingReceiver > 0) {
          throw new SessionConflictError(
            "receiver already joined this session",
          );
        }

        await tx`
          insert into session_participants (
            session_id,
            role,
            status,
            display_name,
            joined_at
          )
          values (
            ${sessionRow.id},
            'receiver',
            'joined',
            ${input.displayName ?? null},
            now()
          )
        `;

        await tx`
          update transfer_sessions
          set status = 'ready', updated_at = now()
          where id = ${sessionRow.id}
        `;

        await tx`
          insert into session_events (session_id, event_type, actor_role, payload)
          values (
            ${sessionRow.id},
            'receiver_joined',
            'receiver',
            ${JSON.stringify({
              displayName: input.displayName ?? null,
            })}::jsonb
          )
        `;

        const detail = await loadSessionDetailByCode(tx, code);

        if (!detail) {
          throw new Error("failed to load joined session detail");
        }

        return detail;
      });
    },

    getSession(code) {
      return loadSessionDetailByCode(sql, code);
    },

    async getOperatorSummary() {
      const [countRows, sessionRows] = await Promise.all([
        sql<
          {
            total: number;
            waiting_receiver: number;
            ready: number;
            transferring: number;
            completed: number;
            failed: number;
            expired: number;
            cancelled: number;
          }[]
        >`
          select
            count(*)::int as total,
            count(*) filter (where status = 'waiting_receiver')::int as waiting_receiver,
            count(*) filter (where status = 'ready')::int as ready,
            count(*) filter (where status = 'transferring')::int as transferring,
            count(*) filter (where status = 'completed')::int as completed,
            count(*) filter (where status = 'failed')::int as failed,
            count(*) filter (where status = 'expired')::int as expired,
            count(*) filter (where status = 'cancelled')::int as cancelled
          from transfer_sessions
        `,
        sql<
          {
            id: string;
            code: string;
            status: string;
            created_at: TimestampValue;
            updated_at: TimestampValue;
            expires_at: TimestampValue;
            file_name: string;
            file_size_bytes: NumericValue;
            sender_joined_at: TimestampValue | null;
            receiver_joined_at: TimestampValue | null;
            last_event_type: string | null;
          }[]
        >`
          select
            sessions.id,
            sessions.code,
            sessions.status,
            sessions.created_at,
            sessions.updated_at,
            sessions.expires_at,
            files.file_name,
            files.file_size_bytes,
            sender.joined_at as sender_joined_at,
            receiver.joined_at as receiver_joined_at,
            events.event_type as last_event_type
          from transfer_sessions as sessions
          inner join session_files as files on files.session_id = sessions.id
          left join session_participants as sender
            on sender.session_id = sessions.id
            and sender.role = 'sender'
          left join session_participants as receiver
            on receiver.session_id = sessions.id
            and receiver.role = 'receiver'
          left join lateral (
            select event_type
            from session_events
            where session_id = sessions.id
            order by created_at desc, id desc
            limit 1
          ) as events on true
          order by sessions.created_at desc
          limit 20
        `,
      ]);

      const counts = countRows[0] ?? {
        total: 0,
        waiting_receiver: 0,
        ready: 0,
        transferring: 0,
        completed: 0,
        failed: 0,
        expired: 0,
        cancelled: 0,
      };

      return operatorSummaryPayloadSchema.parse({
        generatedAt: new Date().toISOString(),
        counts: {
          total: counts.total,
          waitingReceiver: counts.waiting_receiver,
          ready: counts.ready,
          transferring: counts.transferring,
          completed: counts.completed,
          failed: counts.failed,
          expired: counts.expired,
          cancelled: counts.cancelled,
        },
        sessions: sessionRows.map((row) => ({
          id: row.id,
          code: row.code,
          status: row.status,
          createdAt: toIsoString(row.created_at),
          updatedAt: toIsoString(row.updated_at),
          expiresAt: toIsoString(row.expires_at),
          fileName: row.file_name,
          fileSizeBytes: toInteger(row.file_size_bytes),
          senderJoinedAt: row.sender_joined_at
            ? toIsoString(row.sender_joined_at)
            : undefined,
          receiverJoinedAt: row.receiver_joined_at
            ? toIsoString(row.receiver_joined_at)
            : undefined,
          lastEventType: row.last_event_type ?? undefined,
        })),
      });
    },

    async startTransfer(code) {
      return sql.begin(async (tx) => {
        const sessionRows = await tx<SessionRow[]>`
          select id, code, status, created_at, updated_at, expires_at
          from transfer_sessions
          where code = ${code}
          for update
        `;

        const sessionRow = sessionRows[0];

        if (!sessionRow) {
          throw new SessionNotFoundError(code);
        }

        if (sessionRow.status !== "ready") {
          throw new SessionConflictError(
            `transfer cannot start from state ${sessionRow.status}`,
          );
        }

        await tx`
          update transfer_sessions
          set status = 'transferring', updated_at = now()
          where id = ${sessionRow.id}
        `;

        await tx`
          insert into session_events (session_id, event_type, actor_role, payload)
          values (
            ${sessionRow.id},
            'transfer_started',
            'sender',
            '{}'::jsonb
          )
        `;

        const detail = await loadSessionDetailByCode(tx, code);

        if (!detail) {
          throw new Error("failed to load session after starting transfer");
        }

        return detail;
      });
    },

    async completeTransfer(code, checksumSha256) {
      return sql.begin(async (tx) => {
        const sessionRows = await tx<SessionRow[]>`
          select id, code, status, created_at, updated_at, expires_at
          from transfer_sessions
          where code = ${code}
          for update
        `;

        const sessionRow = sessionRows[0];

        if (!sessionRow) {
          throw new SessionNotFoundError(code);
        }

        if (sessionRow.status !== "transferring") {
          throw new SessionConflictError(
            `transfer cannot complete from state ${sessionRow.status}`,
          );
        }

        await tx`
          update transfer_sessions
          set status = 'completed', updated_at = now()
          where id = ${sessionRow.id}
        `;

        await tx`
          insert into session_events (session_id, event_type, actor_role, payload)
          values (
            ${sessionRow.id},
            'transfer_completed',
            'receiver',
            ${JSON.stringify({ checksumSha256 })}::jsonb
          )
        `;

        const detail = await loadSessionDetailByCode(tx, code);

        if (!detail) {
          throw new Error("failed to load session after completing transfer");
        }

        return detail;
      });
    },

    async failTransfer(code, reason) {
      return sql.begin(async (tx) => {
        const sessionRows = await tx<SessionRow[]>`
          select id, code, status, created_at, updated_at, expires_at
          from transfer_sessions
          where code = ${code}
          for update
        `;

        const sessionRow = sessionRows[0];

        if (!sessionRow) {
          throw new SessionNotFoundError(code);
        }

        const failableStates = ["ready", "transferring"];

        if (!failableStates.includes(sessionRow.status)) {
          throw new SessionConflictError(
            `transfer cannot fail from state ${sessionRow.status}`,
          );
        }

        await tx`
          update transfer_sessions
          set status = 'failed', updated_at = now()
          where id = ${sessionRow.id}
        `;

        await tx`
          insert into session_events (session_id, event_type, actor_role, payload)
          values (
            ${sessionRow.id},
            'transfer_failed',
            null,
            ${JSON.stringify({ reason })}::jsonb
          )
        `;

        const detail = await loadSessionDetailByCode(tx, code);

        if (!detail) {
          throw new Error("failed to load session after failing transfer");
        }

        return detail;
      });
    },

    async recordTransferProgress(code, bytesSent, totalBytes) {
      const sessionRows = await sql<SessionRow[]>`
        select id from transfer_sessions where code = ${code} limit 1
      `;

      const sessionRow = sessionRows[0];

      if (!sessionRow) {
        return;
      }

      await sql`
        insert into session_events (session_id, event_type, actor_role, payload)
        values (
          ${sessionRow.id},
          'transfer_progress',
          'sender',
          ${JSON.stringify({ bytesSent, totalBytes })}::jsonb
        )
      `;
    },
  };
}
