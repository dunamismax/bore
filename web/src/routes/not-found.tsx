import { Link, createRoute } from "@tanstack/react-router";

import { Button } from "@/components/ui/button";
import { rootRoute } from "./root";

function NotFoundPage() {
  return (
    <section className="animate-rise-in rounded-2xl border bg-card p-6 shadow-sm lg:p-10">
      <p className="mb-2 font-mono text-xs uppercase tracking-widest text-primary">404</p>
      <h1 className="mb-4 font-display text-4xl leading-[0.98] tracking-tight sm:text-5xl">
        That route is not part of bore&rsquo;s current surface.
      </h1>
      <p className="mb-6 text-lg text-muted-foreground">
        Try the product homepage or the relay operator page. Bore keeps the browser surface narrow
        on purpose.
      </p>
      <div className="flex flex-wrap gap-3">
        <Button asChild>
          <Link to="/">Back to homepage</Link>
        </Button>
        <Button variant="outline" asChild>
          <Link to="/ops/relay">Open relay status</Link>
        </Button>
      </div>
    </section>
  );
}

export const notFoundRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "*",
  component: NotFoundPage,
});
