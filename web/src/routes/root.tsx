import { Link, Outlet, createRootRoute } from "@tanstack/react-router";

function RootLayout() {
  return (
    <div className="mx-auto w-full max-w-[1160px] px-4 py-4 pb-12">
      <header className="sticky top-0 z-10 flex flex-col items-start justify-between gap-4 py-4 backdrop-blur-xl sm:flex-row sm:items-center">
        <Link to="/" className="flex items-center gap-3 no-underline">
          <span className="rounded-full border border-border bg-card px-3 py-1 font-mono text-sm tracking-widest">
            bore
          </span>
          <span className="text-sm text-muted-foreground">payload-blind encrypted transfer</span>
        </Link>
        <nav className="flex items-center gap-1" aria-label="Primary">
          <Link
            to="/"
            className="rounded-full px-3 py-2 text-sm text-muted-foreground transition-colors hover:text-foreground"
          >
            Product
          </Link>
          <Link
            to="/ops/relay"
            className="rounded-full px-3 py-2 text-sm text-muted-foreground transition-colors hover:text-foreground"
          >
            Relay Ops
          </Link>
        </nav>
      </header>

      <main className="grid gap-8">
        <Outlet />
      </main>

      <footer className="mt-14 border-t border-border pt-8 text-sm text-muted-foreground">
        <p>Bore ships a verified relay-based path today. Direct transport remains planned work.</p>
        <p className="mt-2">
          The relay pairs peers and forwards encrypted bytes. It should stay payload-blind.
        </p>
      </footer>
    </div>
  );
}

export const rootRoute = createRootRoute({
  component: RootLayout,
});
