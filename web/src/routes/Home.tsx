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
  Scissors,
  ShieldCheck,
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

function Spark({ days }: { days: DayPoint[] }) {
  const max = Math.max(1, ...days.map((d) => d.calls));
  return (
    <div className="mt-2 flex h-7 items-end gap-1" role="img" aria-label="Calls per day, last 7 days">
      {days.map((d) => (
        <div
          key={d.day}
          className="flex-1 rounded-sm"
          title={`${d.day}: ${d.calls} calls, ${d.errors} errors`}
          style={{
            height: `${Math.max(10, (d.calls / max) * 100)}%`,
            background: d.calls ? "var(--accent)" : "var(--line)",
            opacity: d.calls ? 0.85 : 1,
          }}
        />
      ))}
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
  const fetched = h.fetched ?? 0;
  const returned = h.returned ?? 0;
  const saved = fetched > 0 ? Math.max(0, Math.round((1 - returned / fetched) * 100)) : 0;
  const calls7 = h.calls7d ?? 0;
  const errRate = calls7 ? Math.round(((h.errors7d ?? 0) / calls7) * 1000) / 10 : 0;
  const health = h.health ?? [];
  const heaviest = h.heaviest ?? [];
  const maxTok = Math.max(1, ...heaviest.map((r) => r.avgTokens));
  return (
    <div className="page">
      <PageHead
        title="Home"
        sub="How your toolkits are doing, and anything that needs you."
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
      {h.needsYou.length > 0 && (
        <Panel
          className="mb-5"
          title="Needs you"
          icon={<Inbox size={15} className="muted" />}
          right={<Chip tone="warn">{h.needsYou.length} waiting</Chip>}
        >
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
      )}
      <div className="mb-5 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <StatCard
          icon={<ChartNoAxesColumn size={14} />}
          label="Agent calls, 7 days"
          value={num(calls7)}
          foot={
            <span>
              {num(h.callsToday)} today
              {errRate > 0 && <span style={{ color: "var(--bad)" }}> · {errRate}% errors</span>}
            </span>
          }
          onClick={() => navigate("/activity")}
        >
          <Spark days={h.daily} />
        </StatCard>
        <StatCard
          icon={<Scissors size={14} />}
          label="Tokens kept out of context"
          value={fetched ? `${saved}%` : "–"}
          foot={
            fetched ? (
              <span>
                {num(returned)} returned of {num(fetched)} fetched
              </span>
            ) : (
              "Shows once agents call tools"
            )
          }
          onClick={() => navigate("/activity?tab=usage")}
        />
        <StatCard
          icon={<ToolCase size={14} />}
          label="Toolkits"
          value={h.toolkits}
          foot={`${num(h.tools)} tools · ${num(h.apis)} applications`}
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
      </div>
      <div className="grid items-start gap-5 xl:grid-cols-[minmax(0,1.55fr)_minmax(320px,1fr)]">
        <div className="flex min-w-0 flex-col gap-5">
          <Panel
            title="Toolkit health"
            icon={<ShieldCheck size={15} className="muted" />}
            right={
              <Link to="/guidelines" className="text-[12.5px]">
                Guidelines
              </Link>
            }
          >
            {health.length === 0 ? (
              <Empty title="No toolkits yet">
                A toolkit is one MCP endpoint. Create one to see how it scores against the MCP guidelines.
              </Empty>
            ) : (
              <table className="table">
                <thead>
                  <tr>
                    <th>Toolkit</th>
                    <th>Score</th>
                    <th className="num">Tools</th>
                    <th className="num">tools/list</th>
                    <th>Top finding</th>
                  </tr>
                </thead>
                <tbody>
                  {health.map((t) => (
                    <tr
                      key={t.name}
                      className="clickable"
                      onClick={() => navigate(`/toolkits/${t.name}?tab=guidelines`)}
                    >
                      <td>
                        <Link
                          to={`/toolkits/${t.name}`}
                          className="mono font-medium"
                          onClick={(e) => e.stopPropagation()}
                        >
                          {t.name}
                        </Link>
                        <div className="muted text-[12px]">
                          {t.agents} {t.agents === 1 ? "agent" : "agents"}
                        </div>
                      </td>
                      <td style={{ width: 150 }}>
                        <div className="flex items-center gap-2">
                          <span className="w-7 text-right font-semibold">{t.score}</span>
                          <div className="meter flex-1">
                            <span
                              style={{
                                width: `${t.score}%`,
                                background:
                                  t.score >= 85
                                    ? "var(--ok-dot)"
                                    : t.score >= 60
                                      ? "var(--warn-dot)"
                                      : "var(--bad-dot)",
                              }}
                            />
                          </div>
                        </div>
                      </td>
                      <td className="num">{t.tools}</td>
                      <td className="num">{num(t.listTokens)} tok</td>
                      <td>
                        {t.top ? (
                          <span className="flex items-center gap-2">
                            <Chip tone={t.block ? "bad" : "warn"}>
                              {t.block ? `${t.block} blocking` : `${t.warn} ${t.warn === 1 ? "warning" : "warnings"}`}
                            </Chip>
                            <span className="text-2 truncate text-[12.5px]">{t.top}</span>
                          </span>
                        ) : (
                          <Chip tone="ok">Looks good</Chip>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </Panel>
          <Panel
            title="Heaviest tools this week"
            icon={<Scissors size={15} className="muted" />}
            right={<span className="muted text-[12px]">Average output per call</span>}
          >
            {heaviest.length === 0 ? (
              <Empty title="No tool calls this week">
                When agents call tools, the ones with the biggest outputs show here, so you know what to trim first.
              </Empty>
            ) : (
              <div className="px-4 py-3">
                {heaviest.map((r) => (
                  <div key={r.key} className="flex items-center gap-3 py-1.5">
                    <span className="mono w-56 truncate text-[12.5px]">{r.label}</span>
                    <div className="meter flex-1">
                      <span
                        style={{
                          width: `${(r.avgTokens / maxTok) * 100}%`,
                          background: r.avgTokens > 2000 ? "var(--warn-dot)" : "var(--accent)",
                        }}
                      />
                    </div>
                    <span className="w-20 text-right text-[12.5px]">{num(r.avgTokens)} tok</span>
                    <span className="muted w-16 text-right text-[12px]">{r.calls} calls</span>
                  </div>
                ))}
              </div>
            )}
          </Panel>
        </div>
        <div className="flex min-w-0 flex-col gap-5">
          {h.needsYou.length === 0 && (
            <div className="panel flex items-center gap-3 px-4 py-3.5">
              <span className="icon-tile ok">
                <Inbox size={15} />
              </span>
              <div>
                <div className="font-semibold">You're all caught up</div>
                <div className="muted text-[12.5px]">Reviews, tool requests and blocked writes show up here.</div>
              </div>
            </div>
          )}
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
                <span className="muted text-[12px]">{num(c.tokens)} tok</span>
                <span className="muted truncate text-[12px]" style={{ maxWidth: 110 }}>
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
