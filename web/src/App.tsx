import { CirclePlay, FlaskConical, LogOut } from "lucide-react";
import { createContext, lazy, type ReactNode, Suspense, useContext, useEffect, useState } from "react";
import { ConnectDialog } from "./components/ConnectDialog";
import { leaveDemo, startTour, Tour } from "./components/Guide";
import { Palette } from "./components/Palette";
import { Logo, Sidebar } from "./components/Sidebar";
import { Button, EmptyState, Loading } from "./components/ui";
import { useHome, useInfo } from "./lib/hooks";
import { match, navigate, useLocation } from "./lib/router";
import { About } from "./routes/About";
import { Activity } from "./routes/Activity";
import { AddAPI } from "./routes/AddAPI";
import { AgentPage, Agents } from "./routes/Agents";
import { APIs } from "./routes/APIs";
import { APIWorkspace } from "./routes/APIWorkspace";
import { Home } from "./routes/Home";
import { Review } from "./routes/Review";
import { Settings } from "./routes/Settings";
import { ToolkitPage } from "./routes/Toolkit";
import { Toolkits } from "./routes/Toolkits";
import { Welcome } from "./routes/Welcome";

const ToolEditor = lazy(() => import("./routes/ToolEditor"));

const ConnectCtx = createContext<(toolkitId?: string, agentId?: string) => void>(() => {});
export const useConnect = () => useContext(ConnectCtx);

// Applications used to be called APIs; old links still work.
const table: [string, (p: Record<string, string>) => ReactNode][] = [
  ["/", () => <Home />],
  ["/welcome", () => <Welcome />],
  ["/apps", () => <APIs />],
  ["/apps/new", () => <AddAPI />],
  ["/apps/:id/import", (m) => <AddAPI apiId={m.id} />],
  ["/apps/:id", (m) => <APIWorkspace key={m.id} id={m.id} />],
  ["/apis", () => <APIs />],
  ["/apis/new", () => <AddAPI />],
  ["/apis/:id/import", (m) => <AddAPI apiId={m.id} />],
  ["/apis/:id", (m) => <APIWorkspace key={m.id} id={m.id} />],
  ["/toolkits", () => <Toolkits />],
  [
    "/toolkits/:tk/tools/:tool",
    (m) => (
      <Suspense fallback={<Loading />}>
        <ToolEditor key={`${m.tk}/${m.tool}`} toolkit={m.tk} tool={m.tool === "new" ? "" : m.tool} />
      </Suspense>
    ),
  ],
  ["/toolkits/:tk", (m) => <ToolkitPage key={m.tk} name={m.tk} />],
  ["/review/:id", (m) => <Review key={m.id} id={m.id} />],
  ["/agents", () => <Agents />],
  ["/agents/:id", (m) => <AgentPage key={m.id} id={m.id} />],
  ["/activity", () => <Activity />],
  ["/settings", () => <Settings />],
  ["/about", () => <About />],
];

function Routes(): ReactNode {
  const { path } = useLocation();
  for (const [pattern, render] of table) {
    const m = pattern === "/" ? (path === "/" ? {} : null) : match(pattern, path);
    if (m) return render(m);
  }
  return (
    <div className="page">
      <EmptyState
        icon={<Logo size={24} />}
        title="There's no page here"
        actions={
          <Button kind="ink" onClick={() => navigate("/")}>
            Go to Home
          </Button>
        }
      >
        The link may be from an older version of mcpDeck.
      </EmptyState>
    </div>
  );
}

function SignedOut() {
  return (
    <div className="grid h-full place-items-center p-6">
      <div className="panel max-w-md p-8 text-center">
        <div className="mb-4 flex justify-center">
          <Logo size={40} />
        </div>
        <h1 className="mb-2 text-[18px] font-semibold">Open mcpDeck from its launch link</h1>
        <p className="muted m-0">
          For safety, the UI only opens through the one-time link mcpDeck prints when it starts. Run{" "}
          <code className="inline-code">mcpdeck</code> again and it opens this page signed in.
        </p>
      </div>
    </div>
  );
}

function DemoBar({ returnUrl }: { returnUrl: string }) {
  return (
    <div className="demo-bar" role="status">
      <FlaskConical size={15} />
      <strong>You're in the demo</strong>
      <span className="hidden opacity-85 md:inline">
        Sample applications and temporary data. Your own mcpDeck is untouched.
      </span>
      <span className="ml-auto flex gap-2">
        <Button size="sm" icon={<CirclePlay size={13} />} onClick={startTour}>
          Tour
        </Button>
        {returnUrl && (
          <Button size="sm" icon={<LogOut size={13} />} onClick={() => leaveDemo(returnUrl)}>
            Leave demo
          </Button>
        )}
      </span>
    </div>
  );
}

/** First run: show what mcpDeck is before anything else (UI-11). */
function useFirstRun() {
  const home = useHome();
  const { path, search } = useLocation();
  useEffect(() => {
    const h = home.data;
    if (!h || h.demo) return;
    if (path === "/" && !search.get("from") && !h.welcomeSeen && h.apis === 0) navigate("/welcome", true);
  }, [home.data, path]);
}

export function App() {
  const info = useInfo();
  const [palette, setPalette] = useState(false);
  const [connect, setConnect] = useState<{ toolkitId?: string; agentId?: string } | null>(null);
  useFirstRun();
  useEffect(() => {
    const k = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setPalette((p) => !p);
      }
    };
    window.addEventListener("keydown", k);
    return () => window.removeEventListener("keydown", k);
  }, []);
  if (info.error && (info.error as any).status === 401) return <SignedOut />;
  const openConnect = (toolkitId?: string, agentId?: string) => setConnect({ toolkitId, agentId });
  return (
    <ConnectCtx.Provider value={openConnect}>
      <div className="flex h-full flex-col">
        {info.data?.demo && <DemoBar returnUrl={info.data.returnUrl} />}
        <div className="app min-h-0 flex-1">
          <Sidebar onSearch={() => setPalette(true)} />
          <main className="main">
            <Routes />
          </main>
        </div>
      </div>
      <Tour />
      {palette && <Palette onClose={() => setPalette(false)} onConnect={(t) => openConnect(t)} />}
      {connect && (
        <ConnectDialog toolkitId={connect.toolkitId} agentId={connect.agentId} onClose={() => setConnect(null)} />
      )}
    </ConnectCtx.Provider>
  );
}
