import { ArrowLeft, ArrowRight, Check, X } from "lucide-react";
import { useEffect, useState } from "react";
import { useConnect } from "../App";
import { post, put } from "../lib/api";
import { useAction, useAPIs, useHome, useInfo, useToolkits } from "../lib/hooks";
import { nextStep, type StepKey, steps } from "../lib/journey";
import { navigate, useLocation } from "../lib/router";
import { Button } from "./ui";

/** useGoStep runs a step's action: the page where that step happens. */
export function useGoStep() {
  const apis = useAPIs();
  const tks = useToolkits();
  const connect = useConnect();
  return (key: StepKey) => {
    const app = apis.data?.[0];
    const tk = tks.data?.[0];
    switch (key) {
      case "api":
        return navigate("/apps/new");
      case "auth":
        return navigate(app ? `/apps/${app.id}?tab=auth` : "/apps/new");
      case "send":
      case "example":
        return navigate(app ? `/apps/${app.id}` : "/apps/new");
      case "toolkit":
        return navigate("/toolkits?new=1");
      case "tool":
        return navigate(tk ? `/toolkits/${tk.name}/tools/new` : "/toolkits?new=1");
      case "agent":
        return tk ? connect(tk.id) : navigate("/toolkits?new=1");
    }
  };
}

/** SetupGuide is the small card at the bottom of the sidebar: progress and
 *  the one next step. It goes away when everything is done or hidden. */
export function SetupGuide() {
  const home = useHome();
  const go = useGoStep();
  const hide = useAction(() => put("/api/settings", { hideOnboarding: "true" }));
  const h = home.data;
  if (!h || h.demo || h.hideOnboarding) return null;
  const done = h.steps.filter((s) => s.done).length;
  const next = nextStep(h);
  if (!next) return null;
  const s = steps[next.key as StepKey];
  return (
    <section className="panel mx-0.5 mb-2 p-3" aria-label="Setup guide">
      <div className="flex items-center gap-2">
        <span className="text-[12px] font-semibold">Get started</span>
        <span className="muted text-[11.5px]">
          {done} of {h.steps.length}
        </span>
        <button
          type="button"
          className="btn btn-ghost btn-sm btn-icon ml-auto"
          style={{ width: 22, height: 22 }}
          aria-label="Hide the setup guide"
          title="Hide the setup guide"
          onClick={() => hide.mutate(undefined)}
        >
          <X size={13} />
        </button>
      </div>
      <div className="my-2 flex gap-1" aria-hidden>
        {h.steps.map((x) => (
          <span
            key={x.key}
            className="h-1 flex-1 rounded-full"
            style={{ background: x.done ? "var(--ok-dot)" : x === next ? "var(--accent)" : "var(--line)" }}
          />
        ))}
      </div>
      <button
        type="button"
        className="flex w-full items-center gap-2 rounded-md text-left text-[12.5px] font-medium"
        style={{ background: "none", border: 0, padding: 0, cursor: "pointer", color: "var(--text)" }}
        onClick={() => go(next.key as StepKey)}
      >
        <span className="min-w-0 flex-1">
          <span className="muted block text-[11px] font-normal">Next</span>
          {s.title}
        </span>
        <ArrowRight size={14} className="muted" />
      </button>
    </section>
  );
}

/** GettingStarted is Home's full checklist: every step, done or not, any order. */
export function GettingStarted({ compact }: { compact?: boolean }) {
  const home = useHome();
  const go = useGoStep();
  const hide = useAction(() => put("/api/settings", { hideOnboarding: "true" }));
  const h = home.data;
  if (!h) return null;
  const next = nextStep(h);
  const done = h.steps.filter((s) => s.done).length;
  return (
    <section className="panel" aria-label="Getting started">
      <div className="panel-head">
        <h2>Getting started</h2>
        <span className="muted text-[12.5px]">
          {done} of {h.steps.length} done
        </span>
        <div className="right">
          {compact && (
            <Button size="sm" kind="ghost" onClick={() => hide.mutate(undefined)}>
              Hide
            </Button>
          )}
        </div>
      </div>
      <ol className="m-0 list-none p-0">
        {h.steps.map((st, i) => {
          const s = steps[st.key as StepKey];
          const isNext = st === next;
          if (compact && !isNext && st.done) return null;
          return (
            <li
              key={st.key}
              className="list-row items-start"
              style={isNext ? { background: "var(--accent-soft)" } : undefined}
            >
              <span className={`step-dot mt-0.5 ${st.done ? "done" : isNext ? "next" : ""}`}>
                {st.done ? <Check size={12} strokeWidth={3} /> : i + 1}
              </span>
              <div className="min-w-0 flex-1">
                <div className={st.done ? "muted line-through" : "font-medium"}>{s.title}</div>
                {(isNext || !compact) && !st.done && <div className="muted text-[12.5px]">{s.why}</div>}
                {st.reason && !st.done && <div className="faint text-[12px]">{st.reason}</div>}
              </div>
              {!st.done && (
                <Button
                  size="sm"
                  kind={isNext ? "ink" : "default"}
                  disabled={!!st.reason}
                  onClick={() => go(st.key as StepKey)}
                >
                  {s.action}
                </Button>
              )}
            </li>
          );
        })}
      </ol>
    </section>
  );
}

// ───────── The demo tour ─────────

type TourStep = {
  path: string | ((ids: Record<string, string>) => string);
  spot?: string;
  title: string;
  body: string;
};
const tour: TourStep[] = [
  {
    path: "/",
    title: "Welcome to the demo",
    body: "This is a sample company: a CRM, billing, a support desk and an old ERP with a cookie login. Everything here is temporary and never touches your own mcpDeck. This tour takes about two minutes.",
  },
  {
    path: "/apps",
    spot: "nav-apps",
    title: "Applications are the systems you call",
    body: "Each application holds the requests you import from Postman, OpenAPI or curl, its environments and its credentials. Keys are encrypted on this computer and never reach an agent.",
  },
  {
    path: (ids) => (ids.crm ? `/apps/${ids.crm}` : "/apps"),
    spot: "send",
    title: "Try a request",
    body: "Pick a request on the left and press Send. The response shows its size in tokens: what an agent would pay to read it. Save a good response as an example to test tools offline.",
  },
  {
    path: "/toolkits",
    spot: "nav-toolkits",
    title: "Toolkits are what agents get",
    body: "A toolkit is a set of tools for one job, like support. Each one is its own MCP server with a single URL you add to Claude Code, VS Code or Cursor.",
  },
  {
    path: "/toolkits/support",
    spot: "preview",
    title: "Tools return only what's needed",
    body: "A tool is a short TypeScript function that calls one or more applications and trims the answer. Preview as agent shows exactly what a connected agent receives.",
  },
  {
    path: "/agents",
    spot: "nav-agents",
    title: "Agents are the clients you connect",
    body: "Each client has its own token and access: read only, or read and write. Writes from a read agent are blocked by mcpDeck, whatever a tool says.",
  },
  {
    path: "/activity",
    spot: "nav-activity",
    title: "Activity shows every call",
    body: "Each call is recorded with its time and tokens, so you can see what a tool costs and make it smaller. Changes are kept in an audit log.",
  },
  {
    path: "/",
    title: "That's the whole loop",
    body: "Applications → toolkits → agents → activity. Leave the demo to set up your own: start by creating an application.",
  },
];

const KEY = "mcpdeck-tour";
function readStep(): number | null {
  try {
    const v = sessionStorage.getItem(KEY);
    return v === null ? null : Number(v);
  } catch {
    return null;
  }
}
function writeStep(n: number | null) {
  try {
    if (n === null) sessionStorage.removeItem(KEY);
    else sessionStorage.setItem(KEY, String(n));
  } catch {
    /* storage off */
  }
}

export async function leaveDemo(returnUrl: string) {
  writeStep(null);
  await post("/api/shutdown").catch(() => {});
  window.location.href = `${returnUrl}${returnUrl.includes("?") ? "&" : "?"}from=demo`;
}

/** startTour can be called from a button (About, Home in the demo). */
export function startTour() {
  writeStep(0);
  window.dispatchEvent(new Event("mcpdeck-tour"));
  navigate("/");
}

export function Tour() {
  const info = useInfo();
  const apis = useAPIs();
  const { path, search } = useLocation();
  const [i, setI] = useState<number | null>(() => (search.get("tour") === "1" ? 0 : readStep()));
  useEffect(() => {
    const on = () => setI(readStep());
    window.addEventListener("mcpdeck-tour", on);
    return () => window.removeEventListener("mcpdeck-tour", on);
  }, []);
  useEffect(() => {
    if (search.get("tour") === "1") {
      writeStep(0);
      navigate("/", true);
    }
  }, []);
  const ids: Record<string, string> = {};
  for (const a of apis.data ?? []) ids[a.name] = a.id;
  const step = i === null ? null : tour[i];
  const target = step ? (typeof step.path === "function" ? step.path(ids) : step.path) : "";
  useEffect(() => {
    if (step && path !== target) navigate(target);
  }, [i, target]);
  useEffect(() => {
    if (!step?.spot) return;
    let el: Element | null = null;
    const t = setTimeout(() => {
      el = document.querySelector(`[data-tour="${step.spot}"]`);
      el?.classList.add("spot");
    }, 250);
    return () => {
      clearTimeout(t);
      el?.classList.remove("spot");
    };
  }, [i, path]);
  if (!step || i === null || !info.data?.demo) return null;
  const go = (n: number | null) => {
    writeStep(n);
    setI(n);
  };
  const last = i === tour.length - 1;
  return (
    <div className="coach" role="dialog" aria-label="Demo tour" aria-live="polite">
      <div className="bar">
        <span style={{ width: `${((i + 1) / tour.length) * 100}%` }} />
      </div>
      <div className="p-4">
        <div className="mb-1 flex items-center gap-2">
          <span className="muted text-[11.5px] font-medium">
            Step {i + 1} of {tour.length}
          </span>
          <button
            type="button"
            className="btn btn-ghost btn-sm btn-icon ml-auto"
            aria-label="Close the tour"
            onClick={() => go(null)}
          >
            <X size={14} />
          </button>
        </div>
        <h3 className="mb-1.5 text-[15px] font-semibold">{step.title}</h3>
        <p className="text-2 m-0 text-[13px] leading-relaxed">{step.body}</p>
        <div className="mt-4 flex items-center gap-2">
          {i > 0 && (
            <Button size="sm" kind="ghost" icon={<ArrowLeft size={13} />} onClick={() => go(i - 1)}>
              Back
            </Button>
          )}
          <span className="ml-auto" />
          {last ? (
            <>
              <Button size="sm" onClick={() => go(null)}>
                Keep exploring
              </Button>
              {info.data.returnUrl && (
                <Button size="sm" kind="ink" onClick={() => leaveDemo(info.data!.returnUrl)}>
                  Leave demo and start
                </Button>
              )}
            </>
          ) : (
            <Button size="sm" kind="ink" onClick={() => go(i + 1)}>
              Next <ArrowRight size={13} />
            </Button>
          )}
        </div>
      </div>
    </div>
  );
}
