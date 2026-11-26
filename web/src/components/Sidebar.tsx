import {
  AppWindow,
  BookOpen,
  Bot,
  ChartNoAxesColumn,
  CircleHelp,
  CirclePlay,
  House,
  Info,
  Moon,
  Palette,
  Search,
  Settings,
  Star,
  Sun,
  ToolCase,
} from "lucide-react";
import { useEffect, useState } from "react";
import { mod } from "../lib/format";
import { useHome, useInfo } from "../lib/hooks";
import { Link, navigate, useLocation } from "../lib/router";
import { SetupGuide } from "./Guide";
import { Menu } from "./ui";

export const REPO = "https://github.com/abhijeet-oxide/mcpDeck";

const nav = [
  { to: "/", label: "Home", icon: House, tour: "nav-home" },
  { to: "/apps", label: "Applications", icon: AppWindow, tour: "nav-apps" },
  { to: "/toolkits", label: "Toolkits", icon: ToolCase, tour: "nav-toolkits" },
  { to: "/agents", label: "Agents", icon: Bot, tour: "nav-agents" },
  { to: "/activity", label: "Activity", icon: ChartNoAxesColumn, tour: "nav-activity" },
];

export const ACCENTS = [
  { id: "blue", label: "Blue", color: "#2563eb" },
  { id: "teal", label: "Teal", color: "#0d9488" },
  { id: "green", label: "Green", color: "#16a34a" },
  { id: "orange", label: "Orange", color: "#ea580c" },
  { id: "rose", label: "Rose", color: "#e11d48" },
  { id: "graphite", label: "Graphite", color: "#3f3f46" },
] as const;

function store(k: string, v: string) {
  try {
    localStorage.setItem(k, v);
  } catch {
    /* storage off */
  }
}

/** Theme and accent live in this browser (UI-8), applied on <html>. */
export function useTheme() {
  const read = () => ({
    theme: document.documentElement.dataset.theme || "system",
    accent: document.documentElement.dataset.accent || "blue",
  });
  const [st, setSt] = useState(read);
  const dark =
    st.theme === "dark" || (st.theme === "system" && !!window.matchMedia?.("(prefers-color-scheme: dark)").matches);
  useEffect(() => {
    const on = () => setSt(read());
    window.addEventListener("mcpdeck-theme", on);
    const m = window.matchMedia?.("(prefers-color-scheme: dark)");
    m?.addEventListener?.("change", on);
    return () => {
      window.removeEventListener("mcpdeck-theme", on);
      m?.removeEventListener?.("change", on);
    };
  }, []);
  const set = (t: string) => {
    if (t === "system") delete document.documentElement.dataset.theme;
    else document.documentElement.dataset.theme = t;
    store("mcpdeck-theme", t);
    window.dispatchEvent(new Event("mcpdeck-theme"));
  };
  const setAccent = (a: string) => {
    if (a === "blue") delete document.documentElement.dataset.accent;
    else document.documentElement.dataset.accent = a;
    store("mcpdeck-accent", a);
    window.dispatchEvent(new Event("mcpdeck-theme"));
  };
  // Light or dark. Until you choose, it follows the computer (UI-8).
  const theme = dark ? "dark" : "light";
  const toggle = () => set(dark ? "light" : "dark");
  return { theme, chosen: st.theme !== "system", accent: st.accent, dark, set, toggle, setAccent };
}

export function Logo({ size = 22 }: { size?: number }) {
  return <img src="/favicon.svg" width={size} height={size} alt="" />;
}

export function Sidebar() {
  const { path } = useLocation();
  const home = useHome();
  const info = useInfo();
  const needs = home.data?.needsYou.length ?? 0;
  const active = (to: string) =>
    to === "/"
      ? path === "/"
      : path === to || path.startsWith(`${to}/`) || (to === "/apps" && path.startsWith("/apis"));
  const host = info.data?.baseUrl ? info.data.baseUrl.replace(/^https?:\/\//, "") : "";
  return (
    <nav className="side" aria-label="Main">
      <Link to="/" className="brand" style={{ color: "inherit", textDecoration: "none" }}>
        <Logo />
        <span>mcpDeck</span>
      </Link>
      <div className="nav-group">
        {nav.map(({ to, label, icon: Icon, tour }) => (
          <Link
            key={to}
            to={to}
            className="nav-item"
            data-tour={tour}
            aria-current={active(to) ? "page" : undefined}
            title={label}
          >
            <Icon size={17} strokeWidth={1.75} />
            <span className="label">{label}</span>
            {to === "/" && needs > 0 && (
              <span className="nav-count" title={`${needs} waiting for you`}>
                {needs}
              </span>
            )}
          </Link>
        ))}
      </div>
      <div className="mt-auto flex flex-col gap-px pt-4">
        <SetupGuide />
        <Link to="/about" className="nav-item" aria-current={active("/about") ? "page" : undefined} title="About">
          <Info size={17} strokeWidth={1.75} /> <span className="label">About</span>
        </Link>
        <Link
          to="/settings"
          className="nav-item"
          aria-current={active("/settings") ? "page" : undefined}
          title="Settings"
        >
          <Settings size={17} strokeWidth={1.75} /> <span className="label">Settings</span>
        </Link>
        <div className="side-status" title={host ? `Serving at ${host}` : undefined}>
          <span className="dot on" />
          <span className="label min-w-0 truncate mono">{host || "…"}</span>
          <span className="label faint mono ml-auto">{info.data?.version}</span>
        </div>
      </div>
    </nav>
  );
}

const titles: [RegExp, string][] = [
  [/^\/$/, "Home"],
  [/^\/(apps|apis)/, "Applications"],
  [/^\/toolkits/, "Toolkits"],
  [/^\/review/, "Review"],
  [/^\/agents/, "Agents"],
  [/^\/activity/, "Activity"],
  [/^\/settings/, "Settings"],
  [/^\/about/, "About"],
  [/^\/welcome/, "Welcome"],
];

/** TopBar: where you are, search, theme and help, always visible. */
export function TopBar({ onSearch, onTour }: { onSearch: () => void; onTour?: () => void }) {
  const { path } = useLocation();
  const { dark, toggle } = useTheme();
  const title = titles.find(([re]) => re.test(path))?.[1] ?? "";
  return (
    <header className="topbar">
      <div className="crumb">{title}</div>
      <button type="button" className="top-search" onClick={onSearch} aria-label="Search or run a command">
        <Search size={15} />
        <span className="truncate">Search or run a command</span>
        <kbd>{mod()} K</kbd>
      </button>
      <div className="top-actions">
        <button
          type="button"
          className="icon-btn"
          onClick={toggle}
          aria-label={dark ? "Switch to light theme" : "Switch to dark theme"}
          title={dark ? "Switch to light theme" : "Switch to dark theme"}
        >
          {dark ? <Sun size={16} /> : <Moon size={16} />}
        </button>
        <Menu
          label="Help"
          trigger={
            <button type="button" className="icon-btn" aria-label="Help" title="Help">
              <CircleHelp size={16} />
            </button>
          }
          items={[
            ...(onTour ? [{ label: "Take the tour", icon: <CirclePlay size={15} />, onClick: onTour }] : []),
            { label: "MCP guidelines", icon: <BookOpen size={15} />, onClick: () => navigate("/guidelines") },
            { label: "Keyboard: search", icon: <Search size={15} />, onClick: onSearch, hint: `${mod()} K` },
            { label: "Appearance", icon: <Palette size={15} />, onClick: () => navigate("/settings/appearance") },
            "sep",
            {
              label: "Star on GitHub",
              icon: <Star size={15} />,
              onClick: () => window.open(REPO, "_blank", "noreferrer"),
            },
            { label: "About mcpDeck", icon: <Info size={15} />, onClick: () => navigate("/about") },
          ]}
        />
      </div>
    </header>
  );
}
