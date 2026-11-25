import "@fontsource-variable/inter";
import "@fontsource-variable/jetbrains-mono";
import "./styles/app.css";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App } from "./App";
import { ToastProvider } from "./components/ui";
import { RouterProvider } from "./lib/router";

const qc = new QueryClient({
  defaultOptions: {
    queries: { retry: (n, e: any) => e?.status !== 401 && e?.status !== 404 && n < 2, refetchOnWindowFocus: true },
  },
});

try {
  const t = localStorage.getItem("mcpdeck-theme");
  if (t === "light" || t === "dark") document.documentElement.dataset.theme = t;
} catch {
  /* storage off */
}

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <QueryClientProvider client={qc}>
      <RouterProvider>
        <ToastProvider>
          <App />
        </ToastProvider>
      </RouterProvider>
    </QueryClientProvider>
  </StrictMode>,
);
