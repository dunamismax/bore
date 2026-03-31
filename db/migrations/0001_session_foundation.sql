create extension if not exists pgcrypto;

create table if not exists transfer_sessions (
  id uuid primary key default gen_random_uuid(),
  code text not null unique,
  status text not null check (
    status in (
      'waiting_receiver',
      'ready',
      'completed',
      'failed',
      'expired',
      'cancelled'
    )
  ),
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  expires_at timestamptz not null
);

create index if not exists transfer_sessions_status_idx
  on transfer_sessions (status, created_at desc);

create index if not exists transfer_sessions_expires_at_idx
  on transfer_sessions (expires_at);

create table if not exists session_participants (
  id uuid primary key default gen_random_uuid(),
  session_id uuid not null references transfer_sessions(id) on delete cascade,
  role text not null check (role in ('sender', 'receiver')),
  status text not null check (status in ('pending', 'joined')),
  display_name text,
  joined_at timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (session_id, role)
);

create index if not exists session_participants_session_id_idx
  on session_participants (session_id);

create table if not exists session_files (
  id uuid primary key default gen_random_uuid(),
  session_id uuid not null unique references transfer_sessions(id) on delete cascade,
  file_name text not null,
  file_size_bytes bigint not null check (file_size_bytes >= 0),
  mime_type text,
  checksum_sha256 text,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  check (checksum_sha256 is null or checksum_sha256 ~ '^[A-Fa-f0-9]{64}$')
);

create index if not exists session_files_session_id_idx
  on session_files (session_id);

create table if not exists session_events (
  id uuid primary key default gen_random_uuid(),
  session_id uuid not null references transfer_sessions(id) on delete cascade,
  event_type text not null check (
    event_type in ('session_created', 'file_registered', 'receiver_joined')
  ),
  actor_role text check (actor_role in ('sender', 'receiver')),
  payload jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now()
);

create index if not exists session_events_session_id_created_at_idx
  on session_events (session_id, created_at asc);
