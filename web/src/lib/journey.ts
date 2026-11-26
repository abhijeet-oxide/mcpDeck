import { post } from "./api";
import type { Home } from "./types";

/** The steps a new user is guided through (UI-11). They can be done in any
 *  order; the guide always points at the first one not done. */
export type StepKey = "api" | "auth" | "send" | "example" | "toolkit" | "tool" | "agent";
export const steps: Record<StepKey, { title: string; why: string; action: string }> = {
  api: {
    title: "Create an application",
    why: "An application is one system your tools call, like your CRM. Import its Postman collection or OpenAPI file, or add requests manually.",
    action: "Create application",
  },
  auth: {
    title: "Set up sign-in",
    why: "Tell mcpDeck how the application signs in: an API key, a bearer token, or a login request whose token is reused. Secrets are stored encrypted, never in tools.",
    action: "Open Auth",
  },
  send: {
    title: "Send a request",
    why: "Open a request and press Send. It goes through mcpDeck, the same way an agent's call will, so you see what the API returns.",
    action: "Open application",
  },
  example: {
    title: "Save a response as an example",
    why: "Examples let you test tools without the network and show agents what a response looks like. Values that look like secrets are masked.",
    action: "Open application",
  },
  toolkit: {
    title: "Create a toolkit",
    why: "A toolkit is a set of tools for one job, like support. Each toolkit is its own MCP server with one URL that you add to an agent.",
    action: "Create toolkit",
  },
  tool: {
    title: "Build your first tool",
    why: "A tool is a short TypeScript function that calls your applications and returns only what an agent needs.",
    action: "New tool",
  },
  agent: {
    title: "Connect an agent",
    why: "Add the toolkit to VS Code, Claude Code or Cursor. You choose whether it may read only, or read and write.",
    action: "Connect agent",
  },
};

export function nextStep(h: Home | undefined) {
  return h?.steps.find((s) => !s.done && !s.reason) ?? h?.steps.find((s) => !s.done);
}

/** Opens the demo in a new tab with the same theme, starting the tour. */
export async function openDemo(tour = true): Promise<void> {
  const r = await post<{ url: string }>("/api/demo");
  const u = new URL(r.url);
  const theme = document.documentElement.dataset.theme || "system";
  u.searchParams.set("next", `/?${tour ? "tour=1&" : ""}theme=${theme}`);
  window.open(u.toString(), "_blank", "noopener");
}

/** Applies ?theme= from the URL (the demo opens with the caller's theme). */
export function adoptThemeFromURL() {
  const p = new URLSearchParams(window.location.search);
  const t = p.get("theme");
  if (!t) return;
  try {
    localStorage.setItem("mcpdeck-theme", t);
  } catch {
    /* storage off */
  }
  if (t === "light" || t === "dark") document.documentElement.dataset.theme = t;
  else delete document.documentElement.dataset.theme;
  p.delete("theme");
  const q = p.toString();
  window.history.replaceState(null, "", window.location.pathname + (q ? `?${q}` : ""));
}
