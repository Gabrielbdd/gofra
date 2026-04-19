import { createRoute } from "@tanstack/react-router";

import { Button } from "@/components/ui/button";
import { runtimeConfig } from "@/lib/runtime-config";
import { rootRoute } from "@/routes/__root";

export const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  component: HomePage,
});

function HomePage() {
  return (
    <main className="mx-auto max-w-2xl space-y-6 p-8">
      <header className="space-y-2">
        <h1 className="text-3xl font-bold tracking-tight">
          {runtimeConfig.appName ?? "Gofra starter"}
        </h1>
        <p className="text-muted-foreground">
          This is the generated starter frontend. Edit{" "}
          <code className="rounded bg-muted px-1 py-0.5 text-sm">
            web/src/routes/index.tsx
          </code>{" "}
          to change this page.
        </p>
      </header>
      <section className="space-y-2 rounded-lg border border-border p-4">
        <h2 className="font-semibold">Runtime config</h2>
        <dl className="grid grid-cols-[max-content_1fr] gap-x-4 text-sm">
          <dt className="text-muted-foreground">App name</dt>
          <dd>{runtimeConfig.appName ?? "—"}</dd>
          <dt className="text-muted-foreground">API base URL</dt>
          <dd>{runtimeConfig.apiBaseUrl ?? "—"}</dd>
        </dl>
      </section>
      <Button>Get started</Button>
    </main>
  );
}
