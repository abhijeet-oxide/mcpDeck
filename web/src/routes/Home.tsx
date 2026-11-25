import {
  AppWindow,
  ArrowRight,
  Ban,
  Bot,
  ChartNoAxesColumn,
  CirclePlay,
  Code2,
  FlaskConical,
  Inbox,
  MessageSquarePlus,
  Plus,
  ToolCase,
} from "lucide-react";
import { useState } from "react";
import { useConnect } from "../App";
import { AskAgentButton, type Prompt, toolRequestPrompt } from "../components/AskAgent";
import { HowItWorks } from "../components/Explainer";
import { GettingStarted, startTour } from "../components/Guide";
import {
  Button,
  Chip,
  CopyButton,
  Empty,
  ErrorBox,
  Loading,
  PageHead,
  Panel,
  StatCard,
  StatusChip,
  useToast,
} from "../components/ui";
import { post } from "../lib/api";
import { ago, num, time } from "../lib/format";
import { useAction, useHome, useInfo, useToolkits } from "../lib/hooks";
import { openDemo } from "../lib/journey";
import { Link, navigate, useLocation } from "../lib/router";
import type { DayPoint } from "../lib/types";

export function DemoButton() {
  const info = useInfo();
  const toast = useToast();
  const [busy, setBusy] = useState(false);
  if (info.data?.demo)
    return (
      <Button icon={<CirclePlay size={14} />} onClick={startTour}>
        Tour
      </Button>
    );
  return (
    <Button
      icon={<FlaskConical size={14} />}
      loading={busy}
      title="Open the demo with sample applications in a new tab"
      onClick={async () => {
        setBusy(true);
        try {
          await openDemo(true);
        } catch (e) {
          toast((e as Error).message, true);
        } finally {
          setBusy(false);
        }
      }}
    >
      Demo
    </Button>
  );
}

function Bars({ days }: { days: DayPoint[] }) {
  const max = Math.max(1, ...days.map((d) => d.calls));
  const total = days.reduce((n, d) => n + d.calls, 0);
  return (
    <div className="px-4 pt-3 pb-3">
      <div className="mb-3 flex items-baseline gap-2">
        <span className="text-[20px] font-semibold tracking-tight">{num(total)}</span>
        <span className="muted text-[12.5px]">calls in the last 7 days</span>
      </div>
      <div className="flex items-end gap-2" role="img" aria-label="Calls per day, last 7 days">
        {days.map((d) => (
          <div
            key={d.day}
            className="flex flex-1 flex-col items-center gap-1.5"
            title={`${d.day}: ${d.calls} calls, ${d.errors} errors`}
          >
            <div className="flex w-full flex-col justify-end" style={{ height: 64 }}>
              <div
                style={{
                  height: Math.max(3, (d.calls / max) * 64),
                  background: d.calls ? "var(--accent)" : "var(--line)",
                  borderRadius: 4,
                }}
              />
            </div>
            <span className="muted text-[11px]">
              {new Date(`${d.day}T00:00:00Z`).toLocaleDateString([], { weekday: "short" })}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}

/** Home with nothing added yet: the journey, the tour, and the three ideas. */
function FirstRun() {
  const { search } = useLocation();
  const back = search.get("from") === "demo";
  return (
    <div className="page" style={{ maxWidth: 1040 }}>
      <PageHead
        title={back ? "Welcome back. Let's set up your own." : "Let's set up mcpDeck"}
        sub="Start with an application: the system you want agents to use. The guide shows the next step as you go, and you can do them in any order."
        actions={<DemoButton />}
      />
      <div className="grid items-start gap-5 lg:grid-cols-[minmax(0,1.5fr)_minmax(280px,1fr)]">
        <GettingStarted />
        <div className="flex flex-col gap-4">
          <div className="panel p-5">
            <div className="icon-tile mb-3">
              <AppWindow size={16} />
            </div>
            <h3 className="mb-1 text-[15px] font-semibold">Start with an application</h3>
            <p className="muted mt-0 mb-4 text-[13px]">
              Drop in a Postman collection or an OpenAPI file. You'll see every request before anything is saved.
            </p>
            <Button kind="ink" icon={<Plus size={14} />} onClick={() => navigate("/apps/new")}>
              Create application
            </Button>
          </div>
          {!back && (
            <div className="panel p-5">
              <h3 className="mb-1 text-[14px] font-semibold">Not sure yet?</h3>
              <p className="muted mt-0 mb-3 text-[13px]">
                Take a two-minute tour of a demo with sample applications. It opens in a new tab.
              </p>
              <DemoButton />
            </div>
          )}
        </div>
      </div>
      <h2 className="mt-10 mb-4 text-[15px] font-semibold">How it works</h2>
      <HowItWorks />
    </div>
  );
}

export function Home() {
  const home = useHome();
  const tks = useToolkits();
  const connect = useConnect();
  const toast = useToast();
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
  if (h.apis === 0) return <FirstRun />;
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
  const showSteps = !h.hideOnboarding && !h.demo && h.steps.some((s) => !s.done);
  const hasToolkit = (tks.data?.length ?? 0) > 0;
  return (
    <div className="page">
      <PageHead
        title="Home"
        sub="What needs you, and what your agents did today."
        actions={
          <>
            <DemoButton />
            {hasToolkit && <AskAgentButton prompts={prompts} toolkit={tk} />}
            {hasToolkit ? (
              <Button kind="ink" icon={<Bot size={14} />} onClick={() => connect()}>
                Connect agent
              </Button>
            ) : (
              <Button kind="ink" icon={<Plus size={14} />} onClick={() => navigate("/toolkits?new=1")}>
                Create toolkit
              </Button>
            )}
          </>
        }
      />
      {showSteps && (
        <div className="mb-5">
          <GettingStarted compact />
        </div>
      )}
      <div className="mb-5 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <StatCard
          icon={<AppWindow size={14} />}
          label="Applications"
          value={h.apis}
          foot={`${num(h.requests)} requests`}
          onClick={() => navigate("/apps")}
        />
        <StatCard
          icon={<ToolCase size={14} />}
          label="Toolkits"
          value={h.toolkits}
          foot={`${num(h.tools)} tools in the library`}
          onClick={() => navigate("/toolkits")}
        />
        <StatCard
          icon={<Bot size={14} />}
          label="Agents"
          value={h.agents}
          foot={
            <span className="flex items-center gap-1.5">
              {h.activeAgents > 0 && <span className="dot on" />}
              {h.activeAgents} connected now
            </span>
          }
          onClick={() => navigate("/agents")}
        />
        <StatCard
          icon={<ChartNoAxesColumn size={14} />}
          label="Calls today"
          value={num(h.callsToday)}
          foot={
            <span>
              {num(h.tokensToday)} tokens returned
              {h.errorsToday + h.blockedToday > 0 && (
                <span style={{ color: "var(--bad)" }}>
                  {" "}
                  · {h.errorsToday} errors, {h.blockedToday} blocked
                </span>
              )}
            </span>
          }
          onClick={() => navigate("/activity")}
        />
      </div>
      <div className="grid items-start gap-5 lg:grid-cols-[minmax(0,1.6fr)_minmax(300px,1fr)]">
        <Panel
          title="Needs you"
          icon={<Inbox size={15} className="muted" />}
          right={h.needsYou.length > 0 && <Chip tone="warn">{h.needsYou.length} waiting</Chip>}
        >
          {h.needsYou.length === 0 && (
            <Empty title="You're all caught up">
              Tool versions to review, tool requests from agents and blocked writes show up here.
            </Empty>
          )}
          {h.needsYou.map((n) => (
            <div key={n.kind + n.id} className="list-row items-start">
              <div className={`icon-tile ${n.kind === "review" ? "warn" : n.kind === "blocked" ? "bad" : ""}`}>
                {n.kind === "review" ? (
                  <Code2 size={15} />
                ) : n.kind === "blocked" ? (
                  <Ban size={15} />
                ) : (
                  <MessageSquarePlus size={15} />
                )}
              </div>
              <div className="min-w-0 flex-1">
                <div className="flex flex-wrap items-center gap-2">
                  <span className={n.kind === "review" ? "mono font-semibold" : "font-semibold"}>{n.title}</span>
                  {n.kind === "review" && <StatusChip status="pending" />}
                  {n.toolkit && <span className="tag">{n.toolkit}</span>}
                </div>
                <div className="text-2 line-clamp-2 text-[13px]">{n.detail}</div>
                <div className="muted mt-0.5 text-[12px]">
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
                    <Button size="sm" kind="ghost" onClick={() => close.mutate(n.id)}>
                      Close
                    </Button>
                    <CopyButton
                      text={toolRequestPrompt({ id: n.id, description: n.detail }, n.toolkit)}
                      label="Copy prompt"
                      what="Prompt copied"
                    />
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
        <div className="flex flex-col gap-5">
          <Panel
            title="Last 7 days"
            icon={<ChartNoAxesColumn size={15} className="muted" />}
            right={
              <Link to="/activity?tab=usage" className="text-[12.5px]">
                Usage
              </Link>
            }
          >
            <Bars days={h.daily} />
          </Panel>
          <Panel
            title="Latest calls"
            right={
              <Link to="/activity" className="flex items-center gap-1 text-[12.5px]">
                All activity <ArrowRight size={12} />
              </Link>
            }
          >
            {h.recent.length === 0 && (
              <Empty title="No calls yet">Calls from agents, and requests you send, show up here.</Empty>
            )}
            {h.recent.map((c) => (
              <Link
                key={c.id}
                to={`/activity?call=${c.id}`}
                className="list-row py-2.5"
                style={{ color: "inherit", textDecoration: "none" }}
              >
                <span className={`dot ${c.status === "ok" ? "on" : c.status === "blocked" ? "warn" : "bad"}`} />
                <span className="mono min-w-0 flex-1 truncate text-[12.5px]">{c.name}</span>
                <span className="muted truncate text-[12px]" style={{ maxWidth: 120 }}>
                  {c.actorName}
                </span>
                <span className="faint text-[12px]">{time(c.startedAt)}</span>
              </Link>
            ))}
          </Panel>
        </div>
      </div>
    </div>
  );
}
