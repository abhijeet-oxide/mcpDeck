import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render } from "@testing-library/react";
import type { ReactNode } from "react";
import { vi } from "vitest";
import { ToastProvider } from "../components/ui";
import { RouterProvider } from "../lib/router";

/** mockAPI answers fetch from a path → body map; anything else is a 404. */
export function mockAPI(routes: Record<string, unknown>) {
  const fn = vi.fn(async (input: RequestInfo | URL) => {
    const url = typeof input === "string" ? input : input.toString();
    const path = url.split("?")[0];
    if (path in routes)
      return new Response(JSON.stringify(routes[path]), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    return new Response(JSON.stringify({ error: "not found" }), { status: 404 });
  });
  vi.stubGlobal("fetch", fn);
  return fn;
}

export function renderApp(ui: ReactNode) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <RouterProvider>
        <ToastProvider>{ui}</ToastProvider>
      </RouterProvider>
    </QueryClientProvider>,
  );
}
