import {
  Activity as ActivityIcon,
  ArrowRight,
  Ban,
  Bot,
  Check,
  CircleAlert,
  Code2,
  Inbox,
  Layers,
  MessageSquarePlus,
  Plug,
  Plus,
  Sparkles,
} from "lucide-react";
import { useConnect } from "../App";
import { AskAgentButton, type Prompt, toolRequestPrompt } from "../components/AskAgent";
import {
  Button,
  Chip,
  CopyButton,
  Empty,
  ErrorBox,
  Loading,
  PageHead,
  Panel,
  StatusChip,
  useToast,
} from "../components/ui";
import { post, put } from "../lib/api";
import { ago, num, time } from "../lib/format";
import { useAction, useHome, useToolkits } from "../lib/hooks";
import { Link, navigate } from "../lib/router";
import type { DayPoint, Home as HomeT } from "../lib/types";

const stepText: Record<string, { label: string; to: string; action: string }> = {
  api: { label: "Add an API", to: "/apis/new", action: "Add API" },
  toolkit: { label: "Create a toolkit", to: "/toolkits?new=1", action: "New toolkit" },
  agent: { label: "Connect an agent", to: "connect", action: "Connect agent" },
  tool: { label: "Build the first tool", to: "/toolkits", action: "Open toolkits" },
};

function Steps({ h, compact, onHide }: { h: HomeT; compact?: boolean; onHide?: () => void }) {
  const connect = useConnect();
  const done = h.steps.filter((s) => s.done).length;
  const next = h.steps.find((s) => !s.done);
  const go = (to: string) => (to === "connect" ? connect() : navigate(to));
  if (compact) {
    return (
      <div className="panel mb-4 flex flex-wrap items-center gap-x-5 gap-y-2 px-4 py-2.5">
        <strong className="text-[13px]">Getting started · {done} of 4</strong>
        {h.steps.map((s, i) => (
          <span key={s.key} className={`flex items-center gap-1.5 text-[13px] ${s.done ? "muted" : ""}`}>
            <span
              className="grid place-items-center rounded-full text-[10.5px] font-semibold"
              style={{
                width: 17,
                height: 17,
                background: s.done ? "var(--ok-bg)" : s === next ? "var(--accent)" : "var(--neutral-bg)",
                color: s.done ? "var(--ok)" : s === next ? "#fff" : "var(--muted)",
              }}
            >
              {s.done ? <Check size={11} /> : i + 1}
            </span>
            {stepText[s.key].label}
          </span>
        ))}
        <span className="ml-auto flex gap-2">
          {next && (
            <Button size="sm" kind="ink" onClick={() => go(stepText[next.key].to)}>
              {stepText[next.key].action}
            </Button>
          )}
          <Button size="sm" kind="ghost" onClick={onHide}>
            Hide
          </Button>
        </span>
      </div>
    );
  }
  return (
    <ol className="m-0 flex list-none flex-col p-0">
      {h.steps.map((s, i) => {
        const enabled = !s.done && !s.reason;
        return (
          <li key={s.key} className="list-row">
            <span
              className="grid place-items-center rounded-full text-[12px] font-semibold"
              style={{
                width: 22,
                height: 22,
                background: s.done ? "var(--ok-bg)" : enabled ? "var(--accent)" : "var(--neutral-bg)",
                color: s.done ? "var(--ok)" : enabled ? "#fff" : "var(--muted)",
              }}
            >
              {s.done ? <Check size={13} /> : i + 1}
            </span>
            <span className={s.done ? "muted" : "font-medium"}>{stepText[s.key].label}</span>
            {s.reason && <span className="muted text-[12.5px]">· {s.reason}</span>}
            <span className="ml-auto">
              {enabled && (
                <Button size="sm" kind={i === 0 ? "ink" : "default"} onClick={() => go(stepText[s.key].to)}>
                  {stepText[s.key].action}
                </Button>
              )}
            </span>
          </li>
        );
      })}
    </ol>
  );
}

function FirstRun({ h }: { h: HomeT }) {
  const toast = useToast();
  const demo = async () => {
    try {
      const r = await post<{ url: string }>("/api/demo");
      window.open(r.url, "_blank", "noopener");
    } catch (e) {
      toast((e as Error).message, true);
    }
  };
  return (
    <div className="page" style={{ maxWidth: 760 }}>
      <PageHead
        title="Welcome to mcpDeck"
        sub="Turn the APIs you already have into small, fast MCP tools your agents can use safely."
      />
      <div className="mb-4 grid gap-3 sm:grid-cols-3">
        {[
          {
            icon: Plug,
            t: "Import APIs",
            d: "OpenAPI, Swagger, Postman or curl. Keys stay encrypted on this machine.",
          },
          {
            icon: Code2,
            t: "Write tools",
            d: "Short TypeScript that calls one or more APIs and returns only what's needed.",
          },
          { icon: Bot, t: "Connect agents", d: "Each toolkit is one MCP server for Claude Code, VS Code or Cursor." },
        ].map(({ icon: I, t, d }) => (
          <div key={t} className="panel panel-body">
            <div className="icon-tile mb-2">
              <I size={15} />
            </div>
            <div className="font-semibold">{t}</div>
            <div className="muted text-[12.5px]">{d}</div>
          </div>
        ))}
      </div>
      <Panel title="Four steps">
        <Steps h={h} />
      </Panel>
      <div className="muted mt-4 flex items-center gap-3 text-[13px]">
        Want to look around first?
        <Button size="sm" icon={<Sparkles size={13} />} onClick={demo}>
          Open the demo world
        </Button>
        <span>Mock APIs in their own throwaway data.</span>
      </div>
    </div>
  );
}

function Bars({ days }: { days: DayPoint[] }) {
  const max = Math.max(1, ...days.map((d) => d.calls));
  return (
    <div className="flex items-end gap-1.5 px-3.5 pt-3 pb-2" role="img" aria-label="Calls per day, last 7 days">
      {days.map((d) => (
        <div
          key={d.day}
          className="flex flex-1 flex-col items-center gap-1"
          title={`${d.day}: ${d.calls} calls, ${d.errors} errors`}
        >
          <div className="mono muted text-[10.5px]">{d.calls || ""}</div>
          <div className="flex w-full flex-col justify-end" style={{ height: 72 }}>
            <div
              style={{
                height: Math.max(2, (d.calls / max) * 72),
                background: d.calls ? "var(--accent)" : "var(--line)",
                borderRadius: 3,
                opacity: 0.85,
              }}
            />
          </div>
          <span className="muted text-[10.5px]">
            {new Date(`${d.day}T00:00:00Z`).toLocaleDateString([], { weekday: "narrow" })}
          </span>
        </div>
      ))}
    </div>
  );
}

export function Home() {
  const home = useHome();
  const tks = useToolkits();
  const connect = useConnect();
  const toast = useToast();
  const hide = useAction(() => put("/api/settings", { hideOnboarding: "true" }));
  const close = useAction((id: string) => post(`/api/tool-requests/${id}/close`), {
    onSuccess: () => toast("Request closed"),
  });
  if (home.isLoading) return <Loading />;
  if (home.error)
    return (
      <div className="page">
        <ErrorBox error={home.error} />
      </div>
    );
  const h = home.data!;
  if (h.apis === 0) return <FirstRun h={h} />;
  const tk = tks.data?.[0]?.name ?? "support";
  const prompts: Prompt[] = [
    {
      title: "Answer a question with mcpDeck tools",
      text: `Using only the mcpdeck-${tk} tools, answer: "What does our biggest customer owe, and what's open for them?"\nIf no tool fits, hand it to the mcpdeck-${tk}-builder subagent instead of exploring APIs here.\nCheck: you used one or two tool calls, not raw API calls.`,
    },
    {
      title: "Build a tool for a question you keep asking",
      needsBuild: true,
      text: `Ask the mcpdeck-${tk}-builder subagent for a tool that returns <the fields you need> for <one account / one invoice>.\nIt should search_tools first, then search_apis, describe_api, test_tool and save_tool with a test.\nCheck: the tool is live and calling it returns only those fields.`,
    },
    {
      title: "Report which fields you used",
      text: `After your next call to a mcpdeck-${tk} tool, call report_field_usage with the fields you actually used and the ones you didn't.\nCheck: mcpDeck answers "recorded".`,
    },
  ];
  const showSteps = !h.hideOnboarding && h.steps.some((s) => !s.done);
  const stats = [
    { to: "/apis", icon: Plug, label: "APIs", value: h.apis, foot: `${h.requests} requests` },
    { to: "/toolkits", icon: Layers, label: "Toolkits", value: h.toolkits, foot: `${h.tools} tools in the library` },
    { to: "/agents", icon: Bot, label: "Agents", value: h.agents, foot: `${h.activeAgents} connected now` },
    {
      to: "/activity",
      icon: ActivityIcon,
      label: "Calls today",
      value: num(h.callsToday),
      foot: "tool and build calls",
    },
    {
      to: "/activity?tab=usage",
      icon: Sparkles,
      label: "Tokens returned",
      value: num(h.tokensToday),
      foot: "today, counted by mcpDeck",
    },
    {
      to: "/activity?status=error",
      icon: CircleAlert,
      label: "Errors · blocked",
      value: `${h.errorsToday} · ${h.blockedToday}`,
      foot: "today",
    },
  ];
  return (
    <div className="page">
      <PageHead
        title="Home"
        sub="What needs you and what agents did today."
        actions={
          <>
            <AskAgentButton prompts={prompts} toolkit={tk} />
            <Button icon={<Plus size={14} />} onClick={() => navigate("/apis/new")}>
              Add API
            </Button>
            <Button kind="ink" icon={<Bot size={14} />} onClick={() => connect()}>
              Connect agent
            </Button>
          </>
        }
      />
      {showSteps && <Steps h={h} compact onHide={() => hide.mutate(undefined)} />}
      <div className="panel stats mb-4">
        {stats.map(({ to, icon: I, label, value, foot }) => (
          <Link key={label} to={to} className="stat">
            <div className="label">
              <I size={13} />
              {label}
            </div>
            <div className="value">{value}</div>
            <div className="foot">{foot}</div>
          </Link>
        ))}
      </div>
      <div className="grid items-start gap-4" style={{ gridTemplateColumns: "minmax(0, 1.65fr) minmax(280px, 1fr)" }}>
        <Panel
          title={
            <>
              Needs you <span className="chip ml-1">{h.needsYou.length}</span>
            </>
          }
          icon={<Inbox size={15} className="muted" />}
          right={<span className="muted text-[12px]">Reviews, tool requests and blocked writes</span>}
        >
          {h.needsYou.length === 0 && (
            <Empty title="Nothing waits for you">Reviews, tool requests and blocked writes show up here.</Empty>
          )}
          {h.needsYou.map((n) => (
            <div key={n.kind + n.id} className="list-row items-start">
              <div className={`icon-tile ${n.kind === "review" ? "warn" : n.kind === "blocked" ? "bad" : ""}`}>
                {n.kind === "review" ? (
                  <Code2 size={14} />
                ) : n.kind === "blocked" ? (
                  <Ban size={14} />
                ) : (
                  <MessageSquarePlus size={14} />
                )}
              </div>
              <div className="min-w-0 flex-1">
                <div className="flex flex-wrap items-center gap-2">
                  <span className={n.kind === "review" ? "mono font-semibold" : "font-semibold"}>{n.title}</span>
                  {n.kind === "review" && <StatusChip status="pending" />}
                  {n.toolkit && <Chip>{n.toolkit}</Chip>}
                </div>
                <div className="text-2 line-clamp-2 text-[12.5px]">{n.detail}</div>
                <div className="muted text-[12px]">
                  {n.agent} · {ago(n.at)}
                </div>
              </div>
              <div className="flex flex-none gap-1.5">
                {n.kind === "review" && (
                  <Button size="sm" kind="blue" onClick={() => navigate(`/review/${n.id}`)}>
                    Review
                  </Button>
                )}
                {n.kind === "request" && (
                  <>
                    <CopyButton
                      text={toolRequestPrompt({ id: n.id, description: n.detail }, n.toolkit)}
                      label="Copy prompt"
                      what="Prompt copied"
                    />
                    <Button size="sm" onClick={() => close.mutate(n.id)}>
                      Close
                    </Button>
                  </>
                )}
                {n.kind === "blocked" && (
                  <Button size="sm" onClick={() => navigate(`/toolkits/${n.toolkit}?tab=agents`)}>
                    Review access
                  </Button>
                )}
              </div>
            </div>
          ))}
        </Panel>
        <div className="flex flex-col gap-4">
          <Panel
            title="Last 7 days"
            icon={<ActivityIcon size={15} className="muted" />}
            right={<Link to="/activity?tab=usage">Usage</Link>}
          >
            <Bars days={h.daily} />
          </Panel>
          <Panel
            title="Latest calls"
            right={
              <Link to="/activity">
                All activity <ArrowRight size={12} className="inline" />
              </Link>
            }
          >
            {h.recent.length === 0 && <Empty title="No calls yet">Calls from agents and from you show up here.</Empty>}
            {h.recent.map((c) => (
              <Link
                key={c.id}
                to={`/activity?call=${c.id}`}
                className="list-row"
                style={{ color: "inherit", textDecoration: "none" }}
              >
                <span
                  className={`dot ${c.status === "ok" ? "on" : c.status === "blocked" ? "warn" : ""}`}
                  style={c.status === "error" ? { background: "var(--bad)" } : undefined}
                />
                <span className="mono min-w-0 flex-1 truncate">{c.name}</span>
                <span className="muted truncate text-[12px]" style={{ maxWidth: 110 }}>
                  {c.actorName}
                </span>
                <span className="muted text-[12px]">{time(c.startedAt)}</span>
              </Link>
            ))}
          </Panel>
        </div>
      </div>
    </div>
  );
}
