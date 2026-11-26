import { useQueries } from "@tanstack/react-query";
import { AppWindow, Bot, Code2, Moon, Search, Send, Settings, ToolCase, Zap } from "lucide-react";
import { type ReactNode, useEffect, useMemo, useState } from "react";
import { get } from "../lib/api";
import { useAgents, useAPIs, useLibrary, useToolkits } from "../lib/hooks";
import { navigate } from "../lib/router";
import type { APIDetail } from "../lib/types";
import { useTheme } from "./Sidebar";

type Item = { label: string; hint: string; icon: ReactNode; go: () => void };

// Command palette (UI-6): places and actions, fuzzy by words.
export function Palette({ onClose, onConnect }: { onClose: () => void; onConnect: (toolkit?: string) => void }) {
  const [q, setQ] = useState("");
  const [sel, setSel] = useState(0);
  const apis = useAPIs();
  const tks = useToolkits();
  const tools = useLibrary();
  const agents = useAgents();
  const { dark, toggle } = useTheme();
  // Requests are searchable too; each application's detail is one query.
  const details = useQueries({
    queries: (apis.data ?? []).slice(0, 40).map((a) => ({
      queryKey: ["api", a.id],
      queryFn: () => get<APIDetail>(`/api/apis/${a.id}`),
      staleTime: 30_000,
    })),
  });
  const requests = details.flatMap((d) => (d.data ? d.data.requests.map((r) => ({ api: d.data, r })) : []));
  const items = useMemo<Item[]>(() => {
    const out: Item[] = [
      { label: "New application", hint: "Action", icon: <Zap size={15} />, go: () => navigate("/apps/new") },
      { label: "New toolkit", hint: "Action", icon: <Zap size={15} />, go: () => navigate("/toolkits?new=1") },
      { label: "Connect agent", hint: "Action", icon: <Zap size={15} />, go: () => onConnect() },
      { label: "Activity: calls", hint: "Page", icon: <Search size={15} />, go: () => navigate("/activity") },
      { label: "Activity: usage", hint: "Page", icon: <Search size={15} />, go: () => navigate("/activity?tab=usage") },
      {
        label: "Activity: audit log",
        hint: "Page",
        icon: <Search size={15} />,
        go: () => navigate("/activity?tab=audit"),
      },
      { label: "About mcpDeck", hint: "Page", icon: <Search size={15} />, go: () => navigate("/about") },
      { label: "Settings", hint: "Page", icon: <Settings size={15} />, go: () => navigate("/settings") },
      {
        label: dark ? "Switch to light theme" : "Switch to dark theme",
        hint: "Action",
        icon: <Moon size={15} />,
        go: toggle,
      },
      {
        label: "Settings: requests and proxy",
        hint: "Page",
        icon: <Settings size={15} />,
        go: () => navigate("/settings/network"),
      },
      {
        label: "Settings: security",
        hint: "Page",
        icon: <Settings size={15} />,
        go: () => navigate("/settings/security"),
      },
    ];
    for (const a of apis.data ?? [])
      out.push({
        label: a.name,
        hint: `Application · ${a.title}`,
        icon: <AppWindow size={15} />,
        go: () => navigate(`/apps/${a.id}`),
      });
    for (const t of tks.data ?? []) {
      out.push({
        label: t.name,
        hint: "Toolkit",
        icon: <ToolCase size={15} />,
        go: () => navigate(`/toolkits/${t.name}`),
      });
      out.push({
        label: `Connect an agent to ${t.name}`,
        hint: "Action",
        icon: <Zap size={15} />,
        go: () => onConnect(t.id),
      });
    }
    for (const { api, r } of requests)
      out.push({
        label: r.name,
        hint: `Request · ${r.method} · ${api.name}`,
        icon: <Send size={15} />,
        go: () => navigate(`/apps/${api.id}?req=${r.id}`),
      });
    for (const t of tools.data ?? [])
      out.push({
        label: t.name,
        hint: `Tool · ${t.toolkits.join(", ") || "library"}`,
        icon: <Code2 size={15} />,
        go: () => navigate(`/toolkits/${t.toolkits[0] ?? "_"}/tools/${t.name}`),
      });
    for (const a of agents.data ?? [])
      out.push({
        label: a.name,
        hint: `Agent · ${a.clientLabel}`,
        icon: <Bot size={15} />,
        go: () => navigate(`/agents/${a.id}`),
      });
    return out;
  }, [apis.data, tks.data, tools.data, agents.data, requests, dark]);
  const words = q.toLowerCase().split(/\s+/).filter(Boolean);
  const shown = items.filter((i) => words.every((w) => `${i.label} ${i.hint}`.toLowerCase().includes(w))).slice(0, 12);
  useEffect(() => setSel(0), [q]);
  const run = (i: Item | undefined) => {
    if (!i) return;
    onClose();
    i.go();
  };
  return (
    <div className="overlay" onMouseDown={(e) => e.target === e.currentTarget && onClose()}>
      <div className="dialog" role="dialog" aria-modal="true" aria-label="Search or run">
        <div className="flex items-center gap-2 px-3" style={{ borderBottom: "1px solid var(--line)" }}>
          <Search size={15} className="muted" />
          <input
            autoFocus
            className="input"
            style={{ border: 0, boxShadow: "none", height: 44 }}
            placeholder="Search applications, requests, tools, agents or actions"
            aria-label="Search"
            value={q}
            onChange={(e) => setQ(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Escape") onClose();
              if (e.key === "ArrowDown") {
                e.preventDefault();
                setSel((s) => Math.min(s + 1, shown.length - 1));
              }
              if (e.key === "ArrowUp") {
                e.preventDefault();
                setSel((s) => Math.max(s - 1, 0));
              }
              if (e.key === "Enter") run(shown[sel]);
            }}
          />
        </div>
        <ul className="m-0 list-none overflow-auto p-1.5" style={{ maxHeight: 380 }}>
          {shown.length === 0 && <li className="empty">Nothing matches.</li>}
          {shown.map((i, n) => (
            <li key={`${i.hint}${i.label}`}>
              <button
                type="button"
                className="nav-item"
                style={n === sel ? { background: "var(--nav-on)" } : undefined}
                onMouseEnter={() => setSel(n)}
                onClick={() => run(i)}
              >
                {i.icon}
                <span className="truncate">{i.label}</span>
                <span className="muted ml-auto text-[12px]">{i.hint}</span>
              </button>
            </li>
          ))}
        </ul>
      </div>
    </div>
  );
}
