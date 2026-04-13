-- Add transferring status to session state machine
alter table transfer_sessions drop constraint transfer_sessions_status_check;
alter table transfer_sessions add constraint transfer_sessions_status_check check (
  status in (
    'waiting_receiver',
    'ready',
    'transferring',
    'completed',
    'failed',
    'expired',
    'cancelled'
  )
);

-- Add transfer lifecycle event types
alter table session_events drop constraint session_events_event_type_check;
alter table session_events add constraint session_events_event_type_check check (
  event_type in (
    'session_created',
    'file_registered',
    'receiver_joined',
    'transfer_started',
    'transfer_progress',
    'transfer_completed',
    'transfer_failed'
  )
);
