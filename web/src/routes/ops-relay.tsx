import { useQuery } from "@tanstack/react-query";
import { createRoute } from "@tanstack/react-router";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { formatBytes, formatDuration, formatLocalTimestamp } from "@/lib/format";
import { type RelayStatus, fetchRelayStatus } from "@/lib/relay-status";
import { rootRoute } from "./root";

function roomFill(value: number, maxRooms: number): string {
  if (maxRooms <= 0) return "0%";
  const percent = Math.min(100, (value / maxRooms) * 100);
  return `${Math.round(percent)}%`;
}

function MetricCard({
  label,
  value,
  note,
}: {
  label: string;
  value: string;
  note: string;
}) {
  return (
    <Card className="animate-rise-in">
      <CardContent className="p-4">
        <p className="mb-1 font-mono text-xs uppercase tracking-widest text-primary">{label}</p>
        <p className="font-display text-3xl leading-none tracking-tight">{value}</p>
        <p className="mt-2 text-xs text-muted-foreground">{note}</p>
      </CardContent>
    </Card>
  );
}

function RoomCard({
  label,
  value,
  fill,
  barClass,
}: {
  label: string;
  value: number;
  fill: string;
  barClass: string;
}) {
  return (
    <Card className="animate-rise-in">
      <CardContent className="p-4">
        <div className="flex items-center justify-between">
          <div>
            <p className="mb-1 font-mono text-xs uppercase tracking-widest text-primary">{label}</p>
            <p className="font-display text-3xl leading-none tracking-tight">{value}</p>
          </div>
          <span className="font-mono text-sm text-muted-foreground">{fill}</span>
        </div>
        <div className="mt-3 h-2.5 overflow-hidden rounded-full bg-muted">
          <span
            className={`block h-full rounded-full transition-all duration-200 ${barClass}`}
            style={{ width: fill }}
          />
        </div>
      </CardContent>
    </Card>
  );
}

function RelayStatusView({ relay }: { relay: RelayStatus }) {
  return (
    <>
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <MetricCard
          label="Service"
          value={relay.service}
          note="Current relay identity from /status."
        />
        <MetricCard label="Status" value={relay.status} note="Expected steady state is ok." />
        <MetricCard
          label="Uptime"
          value={formatDuration(relay.uptimeSeconds)}
          note="Process uptime reported by the relay."
        />
        <MetricCard
          label="Max WebSocket message"
          value={formatBytes(relay.limits.maxMessageSizeBytes)}
          note="Per-message transport cap enforced by the relay."
        />
      </div>

      <div className="grid gap-4 sm:grid-cols-3">
        <RoomCard
          label="Waiting rooms"
          value={relay.rooms.waiting}
          fill={roomFill(relay.rooms.waiting, relay.limits.maxRooms)}
          barClass="bg-gradient-to-r from-primary/70 to-primary"
        />
        <RoomCard
          label="Active rooms"
          value={relay.rooms.active}
          fill={roomFill(relay.rooms.active, relay.limits.maxRooms)}
          barClass="bg-gradient-to-r from-secondary/70 to-secondary"
        />
        <RoomCard
          label="Total rooms tracked"
          value={relay.rooms.total}
          fill={roomFill(relay.rooms.total, relay.limits.maxRooms)}
          barClass="bg-gradient-to-r from-foreground/40 to-foreground/70"
        />
      </div>

      <div className="grid gap-4 sm:grid-cols-3">
        {[
          {
            label: "Room TTL",
            value: formatDuration(relay.limits.roomTTLSeconds),
          },
          {
            label: "Reap interval",
            value: formatDuration(relay.limits.reapIntervalSeconds),
          },
          { label: "Room cap", value: String(relay.limits.maxRooms) },
        ].map((item) => (
          <Card key={item.label} className="animate-rise-in">
            <CardContent className="p-4">
              <p className="mb-1 font-mono text-xs uppercase tracking-widest text-primary">
                {item.label}
              </p>
              <p className="font-display text-2xl leading-none tracking-tight">{item.value}</p>
            </CardContent>
          </Card>
        ))}
      </div>
    </>
  );
}

function OpsRelayPage() {
  const [autoRefresh, setAutoRefresh] = useState(true);

  const { data, error, isLoading, dataUpdatedAt, refetch, isFetching } = useQuery({
    queryKey: ["relay-status"],
    queryFn: fetchRelayStatus,
    refetchInterval: autoRefresh ? 5_000 : false,
  });

  return (
    <>
      {/* Hero */}
      <section className="animate-rise-in grid gap-8 rounded-2xl border bg-card p-6 shadow-sm lg:grid-cols-[1.4fr_0.9fr] lg:p-10">
        <div>
          <p className="mb-2 font-mono text-xs uppercase tracking-widest text-primary">
            Operator-facing surface
          </p>
          <h1 className="font-display text-4xl leading-[0.98] tracking-tight sm:text-5xl">
            Relay status, without pretending it is a control plane.
          </h1>
          <p className="mt-4 text-lg text-muted-foreground">
            This page reads aggregate service data from the current relay at{" "}
            <code className="font-mono text-sm">/status</code>. It is same-origin, read-only, and
            scoped to uptime, room counts, and configured relay limits.
          </p>
        </div>
        <div className="rounded-xl border border-dashed border-border bg-card/50 p-6">
          <p className="mb-2 font-mono text-xs uppercase tracking-widest text-primary">
            Current scope
          </p>
          <p className="text-muted-foreground">
            No auth. No file inspection. No direct transport claims. No hidden backend.
          </p>
        </div>
      </section>

      {/* Status panel */}
      <section className="grid gap-6 rounded-2xl border bg-card p-6 shadow-sm lg:p-8">
        <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-center">
          <div>
            <p className="mb-1 font-mono text-xs uppercase tracking-widest text-primary">
              Live snapshot
            </p>
            <h2 className="font-display text-2xl tracking-tight">Relay status</h2>
          </div>
          <div className="flex flex-wrap items-center gap-3">
            <label className="flex items-center gap-2 text-sm text-muted-foreground">
              <input
                type="checkbox"
                checked={autoRefresh}
                onChange={(e) => setAutoRefresh(e.target.checked)}
                className="h-4 w-4"
              />
              Auto-refresh every 5s
            </label>
            <Button
              variant="outline"
              size="sm"
              disabled={isFetching}
              onClick={() => void refetch()}
            >
              {isFetching ? "Refreshing..." : "Refresh now"}
            </Button>
          </div>
        </div>

        <div className="flex flex-wrap items-center gap-4 text-sm">
          {dataUpdatedAt > 0 && (
            <p className="text-muted-foreground">
              Last updated: <strong>{formatLocalTimestamp(new Date(dataUpdatedAt))}</strong>
            </p>
          )}
          {isLoading && <p className="text-muted-foreground">Loading current relay state...</p>}
          {error && (
            <p className="text-destructive">
              Status fetch failed: {error instanceof Error ? error.message : "Unknown error"}
            </p>
          )}
        </div>

        {data && <RelayStatusView relay={data} />}

        {data && (
          <details className="rounded-xl border bg-muted/30 p-4">
            <summary className="cursor-pointer text-sm font-medium">
              Raw <code className="font-mono">/status</code> payload
            </summary>
            <pre className="mt-3 overflow-x-auto rounded-lg bg-[hsl(210,40%,7%)] p-4 text-sm text-[hsl(40,20%,93%)]">
              {JSON.stringify(data, null, 2)}
            </pre>
          </details>
        )}
      </section>
    </>
  );
}

export const relayOpsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/ops/relay",
  component: OpsRelayPage,
});
