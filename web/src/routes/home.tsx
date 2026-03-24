import { Link, createRoute } from "@tanstack/react-router";

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
            Relay-based. Encrypted. Self-hostable.
          </p>
          <h1 className="font-display text-4xl leading-[0.98] tracking-tight sm:text-5xl lg:text-6xl">
            File transfer that keeps the relay in the dark.
          </h1>
          <p className="mt-4 max-w-[42rem] text-lg text-muted-foreground">
            Bore moves a file between two machines with a short rendezvous code, a Noise handshake,
            and a relay that forwards encrypted bytes without becoming a storage or trust anchor.
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
              Verified runtime path today is relay-based, not direct peer-to-peer.
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
              ["Relay role", "encrypted frame broker"],
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
              Relay first, with the claims kept tight.
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-3 text-muted-foreground">
            <p>
              The active client in <code className="font-mono text-sm">cmd/bore</code> with
              implementation packages under{" "}
              <code className="font-mono text-sm">internal/client/</code> creates or parses the
              rendezvous code, completes the cryptographic handshake, and streams encrypted file
              frames through the relay in <code className="font-mono text-sm">cmd/relay</code> +{" "}
              <code className="font-mono text-sm">internal/relay/</code>.
            </p>
            <p>
              The relay is self-hostable and operationally narrow: it exposes{" "}
              <code className="font-mono text-sm">/healthz</code> and{" "}
              <code className="font-mono text-sm">/status</code>, brokers rooms, and forwards bytes.
              It does not decrypt or reinterpret transfer payloads.
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
            "Sender opens a relay room and receives a full rendezvous code.",
            "Receiver joins with the same code.",
            "Both peers mix that code into a Noise XXpsk0 handshake.",
            "Encrypted transfer frames move through the relay over WebSockets.",
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
          Direct transport groundwork exists in{" "}
          <code className="font-mono text-sm">internal/punchthrough/</code> and{" "}
          <code className="font-mono text-sm">cmd/punchthrough</code>, but it is not part of the
          verified transfer path yet.
        </p>
      </section>

      {/* Components */}
      <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-4">
        {[
          {
            label: "cmd/bore + internal/client/",
            title: "CLI transfer engine",
            desc: "Rendezvous, handshake, framing, relay transport, and file integrity checks.",
          },
          {
            label: "cmd/relay + internal/relay/",
            title: "Payload-blind relay",
            desc: "Pairs peers, forwards encrypted frames, and serves health, status, and the web UI.",
          },
          {
            label: "cmd/bore-admin",
            title: "CLI operator summary",
            desc: "Polls /status over HTTP for a terse terminal-side relay view.",
          },
          {
            label: "cmd/punchthrough + internal/punchthrough/",
            title: "Future direct lane",
            desc: "STUN probing and UDP hole-punching groundwork waiting for client integration.",
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
