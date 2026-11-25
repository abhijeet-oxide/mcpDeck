import { createContext, lazy, type ReactNode, Suspense, useContext, useEffect, useState } from "react";
import { ConnectDialog } from "./components/ConnectDialog";
import { Palette } from "./components/Palette";
import { Logo, Sidebar } from "./components/Sidebar";
import { Button, Loading } from "./components/ui";
import { post } from "./lib/api";
import { useInfo } from "./lib/hooks";
import { match, useLocation } from "./lib/router";
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

const ToolEditor = lazy(() => import("./routes/ToolEditor"));

const ConnectCtx = createContext<(toolkitId?: string, agentId?: string) => void>(() => {});
export const useConnect = () => useContext(ConnectCtx);

const table: [string, (p: Record<string, string>) => ReactNode][] = [
  ["/", () => <Home />],
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
      <div className="empty">
        <strong>No page here</strong>
        <a href="/">Go to Home</a>
      </div>
    </div>
  );
}

function SignedOut() {
  return (
    <div className="grid h-full place-items-center p-6">
      <div className="panel panel-body max-w-md text-center">
        <div className="mb-3 flex justify-center">
          <Logo size={36} />
        </div>
        <h1 className="mb-1 text-[17px] font-semibold">Open mcpDeck from its launch link</h1>
        <p className="muted m-0">
          For safety the UI only opens through the one-time link mcpDeck prints when it starts. Run <code>mcpdeck</code>{" "}
          again and it opens this page signed in.
        </p>
      </div>
    </div>
  );
}

function DemoBar({ returnUrl }: { returnUrl: string }) {
  return (
    <div className="demo-bar" role="status">
      <strong>Demo world</strong>
      <span style={{ opacity: 0.8 }}>Mock APIs and throwaway data. Nothing here touches your own mcpDeck.</span>
      {returnUrl && (
        <Button
          size="sm"
          onClick={async () => {
            await post("/api/shutdown").catch(() => {});
            window.location.href = returnUrl;
          }}
        >
          Leave demo
        </Button>
      )}
    </div>
  );
}

export function App() {
  const info = useInfo();
  const [palette, setPalette] = useState(false);
  const [connect, setConnect] = useState<{ toolkitId?: string; agentId?: string } | null>(null);
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
      {palette && <Palette onClose={() => setPalette(false)} onConnect={(t) => openConnect(t)} />}
      {connect && (
        <ConnectDialog toolkitId={connect.toolkitId} agentId={connect.agentId} onClose={() => setConnect(null)} />
      )}
    </ConnectCtx.Provider>
  );
}
