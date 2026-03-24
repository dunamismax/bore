import { createRoute, Link } from "@tanstack/react-router";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { rootRoute } from "./root";

function HomePage() {
  return (
    <>
      {/* Hero */}
      <section className="animate-rise-in grid items-center gap-8 rounded-2xl border bg-card p-6 shadow-sm lg:grid-cols-[1.4fr_0.9fr] lg:p-10">
        <div>
          <p className="mb-2 font-mono text-xs uppercase tracking-widest text-primary">
            Peer-to-peer. Encrypted. Self-hostable.
          </p>
          <h1 className="font-display text-4xl leading-[0.98] tracking-tight sm:text-5xl lg:text-6xl">
            Direct file transfer with no one in the middle.
          </h1>
          <p className="mt-4 max-w-[42rem] text-lg text-muted-foreground">
            Bore moves a file directly between two machines using a short rendezvous code and a
            Noise handshake. The default path is peer-to-peer via STUN discovery and UDP
            hole-punching. When direct fails, a relay forwards encrypted bytes as a transparent
            fallback — never seeing plaintext.
          </p>

          <div className="mt-6 flex flex-wrap gap-3">
            <Button asChild>
              <Link to="/ops/relay">Open relay operator surface</Link>
            </Button>
            <Button variant="outline" asChild>
              <a href="#current-shape">See current shape</a>
            </Button>
          </div>

          <ul className="mt-6 grid gap-3 pl-0">
            <li className="border-l-2 border-secondary/30 pl-4 text-muted-foreground">
              No accounts, no cloud mailbox, no payload inspection.
            </li>
            <li className="border-l-2 border-secondary/30 pl-4 text-muted-foreground">
              Default transport is direct P2P. Relay is the automatic fallback.
            </li>
            <li className="border-l-2 border-secondary/30 pl-4 text-muted-foreground">
              Same-origin operator page reads aggregate relay state from{" "}
              <code className="font-mono text-sm">/status</code>.
            </li>
          </ul>
        </div>

        <div className="animate-rise-in rounded-xl bg-[hsl(210,40%,7%)] p-6 text-[hsl(40,20%,93%)] [animation-delay:0.14s]">
          <p className="mb-2 font-mono text-xs uppercase tracking-widest opacity-70">
            rendezvous code example
          </p>
          <p className="rounded-lg border border-white/10 bg-white/5 p-4 font-mono leading-relaxed break-all">
            Ahcj7nQZclo-j15A_xGS8w-868-outer-crane-crane
          </p>
          <dl className="mt-4 grid grid-cols-2 gap-4">
            {[
              ["Handshake", "Noise XXpsk0"],
              ["Channel", "ChaCha20-Poly1305"],
              ["Integrity", "SHA-256"],
              ["Default path", "direct P2P"],
            ].map(([label, value]) => (
              <div key={label}>
                <dt className="text-xs uppercase tracking-widest opacity-60">{label}</dt>
                <dd className="mt-1">{value}</dd>
              </div>
            ))}
          </dl>
        </div>
      </section>

      {/* Current shape */}
      <div id="current-shape" className="grid gap-6 md:grid-cols-2">
        <Card className="animate-rise-in">
          <CardHeader>
            <p className="font-mono text-xs uppercase tracking-widest text-primary">
              Current shipped path
            </p>
            <CardTitle className="font-display text-2xl tracking-tight">
              P2P first, relay fallback, claims kept tight.
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-3 text-muted-foreground">
            <p>
              The client in <code className="font-mono text-sm">cmd/bore</code> discovers each
              peer's public address via STUN, exchanges candidates through the relay's signaling
              channel, and attempts a direct UDP connection via hole-punching. If direct fails, the
              transfer falls back to the relay automatically.
            </p>
            <p>
              The relay in <code className="font-mono text-sm">cmd/relay</code> serves two roles:
              signaling server for P2P candidate exchange, and fallback transport for encrypted byte
              forwarding. It never sees plaintext regardless of which path is used.
            </p>
          </CardContent>
        </Card>

        <Card className="animate-rise-in">
          <CardHeader>
            <p className="font-mono text-xs uppercase tracking-widest text-primary">
              What this web surface is for
            </p>
            <CardTitle className="font-display text-2xl tracking-tight">
              A real product face plus a practical operator page.
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-3 text-muted-foreground">
            <p>
              Bore ships an in-repo React frontend that tells the product story at the root path and
              gives relay operators a live, browser-facing status page at{" "}
              <code className="font-mono text-sm">/ops/relay</code>.
            </p>
            <p>
              It stays intentionally thin: component-driven SPA with TanStack Query polling the
              relay status endpoint. No new control plane that would distort the current runtime.
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Transfer flow */}
      <section className="animate-rise-in rounded-2xl border bg-card p-6 shadow-sm lg:p-10">
        <p className="mb-2 font-mono text-xs uppercase tracking-widest text-primary">
          Transfer lane
        </p>
        <h2 className="mb-6 font-display text-3xl tracking-tight">
          How the secure path works today.
        </h2>
        <ol className="grid gap-4 pl-0 [counter-reset:steps]">
          {[
            "Sender creates a relay room and receives a full rendezvous code.",
            "Each peer discovers its public address via STUN and exchanges candidates through the relay.",
            "Peers attempt a direct UDP connection via hole-punching.",
            "Both peers mix the rendezvous code into a Noise XXpsk0 handshake over the established connection.",
            "Encrypted file frames flow directly between peers (or via relay if direct failed).",
            "Receiver verifies the final SHA-256 digest before accepting the file.",
          ].map((text, i) => (
            <li
              key={typeof text === "string" ? text : i}
              className="grid grid-cols-[2.5rem_1fr] gap-4 text-muted-foreground [counter-increment:steps]"
            >
              <span className="grid h-10 w-10 place-items-center rounded-full bg-secondary/10 font-mono text-secondary">
                {i + 1}
              </span>
              <span className="self-center">{text}</span>
            </li>
          ))}
        </ol>
        <p className="mt-6 border-t border-border pt-6 text-muted-foreground">
          If the direct connection fails at any step — unfavorable NAT, STUN failure, punch timeout
          — the transfer falls back to the relay automatically. Use{" "}
          <code className="font-mono text-sm">--relay-only</code> to skip the direct attempt.
        </p>
      </section>

      {/* Components */}
      <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-4">
        {[
          {
            label: "cmd/bore + internal/client/",
            title: "P2P transfer engine",
            desc: "Rendezvous, STUN discovery, signaling, direct transport, relay fallback, crypto, and file integrity.",
          },
          {
            label: "cmd/relay + internal/relay/",
            title: "Signaling server + fallback",
            desc: "Coordinates P2P candidate exchange, forwards encrypted frames as fallback, and serves operator surfaces.",
          },
          {
            label: "cmd/bore-admin",
            title: "CLI operator summary",
            desc: "Polls /status over HTTP for a terse terminal-side relay view.",
          },
          {
            label: "cmd/punchthrough + internal/punchthrough/",
            title: "NAT traversal engine",
            desc: "STUN probing, NAT classification, and UDP hole-punching — integrated into the default transfer path.",
          },
        ].map((c) => (
          <Card key={c.label} className="animate-rise-in min-h-[14rem]">
            <CardHeader>
              <p className="font-mono text-xs uppercase tracking-widest text-primary">{c.label}</p>
              <CardTitle className="text-lg">{c.title}</CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-sm text-muted-foreground">{c.desc}</p>
            </CardContent>
          </Card>
        ))}
      </div>

      {/* Operator callout */}
      <section className="animate-rise-in rounded-2xl border bg-card p-6 shadow-sm lg:p-10">
        <p className="mb-2 font-mono text-xs uppercase tracking-widest text-primary">
          Operator surface
        </p>
        <h2 className="mb-3 font-display text-3xl tracking-tight">Need the live relay picture?</h2>
        <p className="mb-6 text-muted-foreground">
          The relay operator page shows uptime, room counts, and configured limits from the same{" "}
          <code className="font-mono text-sm">/status</code> endpoint that powers{" "}
          <code className="font-mono text-sm">bore-admin status</code>.
        </p>
        <Button asChild>
          <Link to="/ops/relay">Open relay status page</Link>
        </Button>
      </section>
    </>
  );
}

export const homeRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  component: HomePage,
});
