import {
  AppWindow,
  Bot,
  ChartNoAxesColumn,
  House,
  Info,
  Moon,
  Search,
  Settings,
  Star,
  Sun,
  ToolCase,
} from "lucide-react";
import { useEffect, useState } from "react";
import { mod } from "../lib/format";
import { useHome, useInfo } from "../lib/hooks";
import { Link, useLocation } from "../lib/router";
import { SetupGuide } from "./Guide";

export const REPO = "https://github.com/abhijeet-oxide/mcpDeck";

const nav = [
  { to: "/", label: "Home", icon: House, tour: "nav-home" },
  { to: "/apps", label: "Applications", icon: AppWindow, tour: "nav-apps" },
  { to: "/toolkits", label: "Toolkits", icon: ToolCase, tour: "nav-toolkits" },
  { to: "/agents", label: "Agents", icon: Bot, tour: "nav-agents" },
  { to: "/activity", label: "Activity", icon: ChartNoAxesColumn, tour: "nav-activity" },
];

export function useTheme() {
  const [theme, setTheme] = useState(() => document.documentElement.dataset.theme || "system");
  const dark = theme === "dark" || (theme === "system" && window.matchMedia?.("(prefers-color-scheme: dark)").matches);
  useEffect(() => {
    const on = () => setTheme(document.documentElement.dataset.theme || "system");
    window.addEventListener("mcpdeck-theme", on);
    return () => window.removeEventListener("mcpdeck-theme", on);
  }, []);
  const set = (t: string) => {
    if (t === "system") delete document.documentElement.dataset.theme;
    else document.documentElement.dataset.theme = t;
    try {
      localStorage.setItem("mcpdeck-theme", t);
    } catch {
      /* storage off */
    }
    setTheme(t);
    window.dispatchEvent(new Event("mcpdeck-theme"));
  };
  return { theme, dark, set };
}

export function Logo({ size = 22 }: { size?: number }) {
  return <img src="/favicon.svg" width={size} height={size} alt="" />;
}

export function Sidebar({ onSearch }: { onSearch: () => void }) {
  const { path } = useLocation();
  const home = useHome();
  const info = useInfo();
  const { dark, set } = useTheme();
  const [, force] = useState(0);
  useEffect(() => {
    const m = window.matchMedia?.("(prefers-color-scheme: dark)");
    const f = () => force((n) => n + 1);
    m?.addEventListener?.("change", f);
    return () => m?.removeEventListener?.("change", f);
  }, []);
  const needs = home.data?.needsYou.length ?? 0;
  const active = (to: string) =>
    to === "/"
      ? path === "/"
      : path === to || path.startsWith(`${to}/`) || (to === "/apps" && path.startsWith("/apis"));
  return (
    <nav className="side" aria-label="Main">
      <div className="brand">
        <Logo />
        <span>mcpDeck</span>
        <button
          type="button"
          className="btn btn-ghost btn-sm btn-icon ml-auto"
          aria-label={dark ? "Switch to light mode" : "Switch to dark mode"}
          title={dark ? "Light mode" : "Dark mode"}
          onClick={() => set(dark ? "light" : "dark")}
        >
          {dark ? <Sun size={15} /> : <Moon size={15} />}
        </button>
      </div>
      <button type="button" className="search-btn" onClick={onSearch}>
        <Search size={14} /> Search or run <kbd>{mod()} K</kbd>
      </button>
      {nav.map(({ to, label, icon: Icon, tour }) => (
        <Link key={to} to={to} className="nav-item" data-tour={tour} aria-current={active(to) ? "page" : undefined}>
          <Icon size={17} strokeWidth={1.75} />
          {label}
          {to === "/" && needs > 0 && (
            <span className="nav-count" title={`${needs} waiting for you`}>
              {needs}
            </span>
          )}
        </Link>
      ))}
      <div className="mt-auto flex flex-col gap-px pt-4">
        <SetupGuide />
        <a className="nav-item" href={REPO} target="_blank" rel="noreferrer" title="Star mcpDeck on GitHub">
          <Star size={17} strokeWidth={1.75} /> Star on GitHub
        </a>
        <Link to="/about" className="nav-item" aria-current={active("/about") ? "page" : undefined}>
          <Info size={17} strokeWidth={1.75} /> About
        </Link>
        <Link to="/settings" className="nav-item" aria-current={active("/settings") ? "page" : undefined}>
          <Settings size={17} strokeWidth={1.75} /> Settings
        </Link>
        <div
          className="muted mt-2 flex items-center gap-2 px-2.5 pt-3 text-[11.5px]"
          style={{ borderTop: "1px solid var(--line)" }}
          title="mcpDeck only listens on this computer"
        >
          <span className="dot on" /> Running on 127.0.0.1:{info.data?.port ?? "…"}
          <span className="faint ml-auto mono">{info.data?.version}</span>
        </div>
      </div>
    </nav>
  );
}
