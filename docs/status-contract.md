# Relay Status Contract

This document records the Go-owned JSON contract for the relay `GET /status` endpoint.

Phase 0 inventory for the frontend migration found two live consumers of this payload before the Astro cutover:

- the legacy Python browser surface at `frontend/src/app/templates/partials/relay_status.html`
- `cmd/bore-admin`

That legacy Python frontend has since been removed. Phase 1 kept the same contract for the new browser surface at `web/src/components/RelayStatusPanel.vue`. Phase 2 added the OpenTUI operator surface in `tui/` as another consumer of the same Go-owned payload.

## Contract shape

```json
{
  "service": "bore-relay",
  "status": "ok",
  "uptimeSeconds": 0,
  "rooms": {
    "total": 0,
    "waiting": 0,
    "active": 0
  },
  "limits": {
    "maxRooms": 0,
    "roomTTLSeconds": 0,
    "reapIntervalSeconds": 0,
    "maxMessageSizeBytes": 0
  },
  "transport": {
    "signalExchanges": 0,
    "signalingStarted": 0,
    "roomsRelayed": 0,
    "bytesRelayed": 0,
    "framesRelayed": 0
  }
}
```

## Field inventory

| Field | Used by removed Python frontend | Used by `bore-admin` | Used by Astro `/ops/relay` | Used by `tui/` |
| --- | --- | --- | --- | --- |
| `service` | yes | yes | yes | yes |
| `status` | yes | yes | yes | yes |
| `uptimeSeconds` | yes | yes | yes | yes |
| `rooms.total` | yes | yes | yes | yes |
| `rooms.waiting` | yes | yes | yes | yes |
| `rooms.active` | yes | yes | yes | yes |
| `limits.maxRooms` | yes | yes | yes | yes |
| `limits.roomTTLSeconds` | yes | yes | yes | yes |
| `limits.reapIntervalSeconds` | yes | yes | yes | yes |
| `limits.maxMessageSizeBytes` | yes | yes | yes | yes |
| `transport.signalExchanges` | yes | yes | yes | yes |
| `transport.signalingStarted` | yes | yes | yes | yes |
| `transport.roomsRelayed` | yes | yes | yes | yes |
| `transport.bytesRelayed` | yes | yes | yes | yes |
| `transport.framesRelayed` | yes | yes | yes | yes |

## Source of truth

- Go types: `internal/relay/status/status.go`
- Relay endpoint implementation: `internal/relay/transport/server.go`
- JSON field-name freeze test: `internal/relay/transport/transport_test.go`

If this payload needs to change, update the Go contract first and then change each consumer deliberately.
