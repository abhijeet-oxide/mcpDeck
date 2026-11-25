import { AppWindow, KeyRound, Plus, Search, TriangleAlert } from "lucide-react";
import { useState } from "react";
import { Avatar, Button, Chip, EmptyState, ErrorBox, Loading, PageHead, Panel } from "../components/ui";
import { useAPIs } from "../lib/hooks";
import { Link, navigate } from "../lib/router";

export function APIs() {
  const q = useAPIs();
  const [filter, setFilter] = useState("");
  const list = (q.data ?? []).filter((a) =>
    `${a.name} ${a.title} ${a.description}`.toLowerCase().includes(filter.toLowerCase()),
  );
  return (
    <div className="page">
      <PageHead
        title="Applications"
        sub="The systems your tools call, like a CRM or a ticket desk. Each holds its requests, environments and credentials."
        actions={
          q.data?.length ? (
            <Button kind="ink" icon={<Plus size={14} />} onClick={() => navigate("/apps/new")}>
              New application
            </Button>
          ) : null
        }
      />
      <ErrorBox error={q.error} />
      {q.isLoading && <Loading />}
      {q.data?.length === 0 && (
        <Panel>
          <EmptyState
            icon={<AppWindow size={22} />}
            title="Add your first application"
            actions={
              <Button kind="ink" icon={<Plus size={14} />} onClick={() => navigate("/apps/new")}>
                Create application
              </Button>
            }
          >
            An application is one system you want agents to use. Drop in a Postman collection or an OpenAPI file, paste
            curl, or add requests manually.
          </EmptyState>
        </Panel>
      )}
      {!!q.data?.length && (
        <>
          {q.data.length > 6 && (
            <div className="relative mb-4 max-w-sm">
              <Search size={14} className="muted absolute top-1/2 left-3 -translate-y-1/2" />
              <input
                className="input"
                style={{ paddingLeft: 32 }}
                placeholder="Filter applications"
                aria-label="Filter applications"
                value={filter}
                onChange={(e) => setFilter(e.target.value)}
              />
            </div>
          )}
          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
            {list.map((a) => (
              <Link key={a.id} to={`/apps/${a.id}`} className="panel card-link flex flex-col p-4">
                <div className="flex items-start gap-3">
                  <Avatar name={a.name} />
                  <div className="min-w-0 flex-1">
                    <div className="truncate font-semibold" style={{ color: "var(--text)" }}>
                      {a.title || a.name}
                    </div>
                    <div className="mono muted truncate text-[12px]">{a.name}</div>
                  </div>
                </div>
                <p className="muted mt-3 mb-0 line-clamp-2 min-h-[38px] text-[12.5px]">
                  {a.description || `Imported from ${a.source}.`}
                </p>
                <div className="mt-3 flex flex-wrap items-center gap-1.5">
                  {a.needsSecret && (
                    <Chip tone="warn">
                      <KeyRound size={11} /> Needs a credential
                    </Chip>
                  )}
                  {a.flagged > 0 && (
                    <Chip tone="warn">
                      <TriangleAlert size={11} /> {a.flagged} to check
                    </Chip>
                  )}
                  {(a.tools ?? []).length > 0 && <Chip tone="ok">Used by {(a.tools ?? []).length} tools</Chip>}
                </div>
                <div
                  className="muted mt-4 flex items-center gap-4 pt-3 text-[12px]"
                  style={{ borderTop: "1px solid var(--line)" }}
                >
                  <span>
                    <strong className="text-2 font-semibold">{a.requests}</strong> requests
                  </span>
                  <span>
                    <strong className="text-2 font-semibold">{a.environments.length}</strong>{" "}
                    {a.environments.length === 1 ? "environment" : "environments"}
                  </span>
                  <span className="ml-auto">{a.source}</span>
                </div>
              </Link>
            ))}
            <button
              type="button"
              className="dropzone min-h-[180px] justify-center"
              onClick={() => navigate("/apps/new")}
            >
              <Plus size={20} className="muted" />
              <span className="font-medium">New application</span>
              <span className="muted text-[12.5px]">Import a collection or spec</span>
            </button>
          </div>
        </>
      )}
    </div>
  );
}
