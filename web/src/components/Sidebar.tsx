import { Activity, Bot, Home, Info, Layers, Moon, Plug, Search, Settings, Star, Sun } from "lucide-react";
import { useEffect, useState } from "react";
import { mod } from "../lib/format";
import { useHome, useInfo } from "../lib/hooks";
import { Link, useLocation } from "../lib/router";

const nav = [
  { to: "/", label: "Home", icon: Home },
  { to: "/apis", label: "APIs", icon: Plug },
  { to: "/toolkits", label: "Toolkits", icon: Layers },
  { to: "/agents", label: "Agents", icon: Bot },
  { to: "/activity", label: "Activity", icon: Activity },
];

export function useTheme() {
  const [theme, setTheme] = useState(() => document.documentElement.dataset.theme || "system");
  const dark = theme === "dark" || (theme === "system" && window.matchMedia?.("(prefers-color-scheme: dark)").matches);
  const set = (t: string) => {
    if (t === "system") delete document.documentElement.dataset.theme;
    else document.documentElement.dataset.theme = t;
    try {
      localStorage.setItem("mcpdeck-theme", t);
    } catch {
      /* storage off */
    }
    setTheme(t);
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
  const active = (to: string) => (to === "/" ? path === "/" : path === to || path.startsWith(`${to}/`));
  return (
    <nav className="side" aria-label="Main">
      <div className="brand">
        <Logo />
        <span>mcpDeck</span>
        <button
          type="button"
          className="btn btn-ghost btn-sm btn-icon ml-auto"
          aria-label={dark ? "Switch to light" : "Switch to dark"}
          onClick={() => set(dark ? "light" : "dark")}
        >
          {dark ? <Sun size={15} /> : <Moon size={15} />}
        </button>
      </div>
      <button type="button" className="search-btn" onClick={onSearch}>
        <Search size={14} /> Search or run <kbd>{mod()} K</kbd>
      </button>
      {nav.map(({ to, label, icon: Icon }) => (
        <Link key={to} to={to} className="nav-item" aria-current={active(to) ? "page" : undefined}>
          <Icon size={17} strokeWidth={1.75} />
          {label}
          {to === "/" && needs > 0 && <span className="nav-count">{needs}</span>}
        </Link>
      ))}
      <div className="mt-auto flex flex-col gap-0.5">
        <a
          className="nav-item"
          href="https://github.com/abhijeet-oxide/mcpDeck"
          target="_blank"
          rel="noreferrer"
          title="Star mcpDeck on GitHub"
        >
          <Star size={17} strokeWidth={1.75} />
          <span className="min-w-0">
            <span className="block leading-tight">Star on GitHub</span>
            <span className="muted block text-[11.5px] leading-tight">abhijeet-oxide/mcpDeck</span>
          </span>
        </a>
        <Link to="/about" className="nav-item" aria-current={active("/about") ? "page" : undefined}>
          <Info size={17} strokeWidth={1.75} /> About
        </Link>
        <Link to="/settings" className="nav-item" aria-current={active("/settings") ? "page" : undefined}>
          <Settings size={17} strokeWidth={1.75} /> Settings
        </Link>
        <div
          className="muted flex items-center gap-2 px-2 pt-2 text-[12px]"
          style={{ borderTop: "1px solid var(--line)" }}
        >
          <span className="dot on" /> 127.0.0.1:{info.data?.port ?? "…"}
        </div>
      </div>
    </nav>
  );
}
