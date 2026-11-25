import { KeyRound, Plus } from "lucide-react";
import { Button, Chip, Empty, ErrorBox, Loading, PageHead, Panel } from "../components/ui";
import { useAPIs } from "../lib/hooks";
import { Link, navigate } from "../lib/router";

export function APIs() {
  const q = useAPIs();
  return (
    <div className="page">
      <PageHead
        title="APIs"
        sub="The systems your tools call. Requests, environments and credentials live inside each API."
        actions={
          <Button kind="ink" icon={<Plus size={14} />} onClick={() => navigate("/apis/new")}>
            Add API
          </Button>
        }
      />
      <ErrorBox error={q.error} />
      {q.isLoading && <Loading />}
      {q.data && (
        <Panel>
          {q.data.length === 0 ? (
            <Empty title="No APIs yet">
              <div className="mt-2">
                <Button kind="ink" onClick={() => navigate("/apis/new")}>
                  Add an API
                </Button>
              </div>
            </Empty>
          ) : (
            <table className="table">
              <thead>
                <tr>
                  <th>API</th>
                  <th>Source</th>
                  <th className="num">Requests</th>
                  <th>Environments</th>
                  <th>Used by</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {q.data.map((a) => (
                  <tr key={a.id} className="clickable" onClick={() => navigate(`/apis/${a.id}`)}>
                    <td>
                      <Link to={`/apis/${a.id}`} className="mono font-semibold" style={{ color: "var(--text)" }}>
                        {a.name}
                      </Link>
                      <div className="muted text-[12.5px]">{a.title}</div>
                    </td>
                    <td className="muted">{a.source}</td>
                    <td className="num">{a.requests}</td>
                    <td className="muted">{a.environments.join(", ")}</td>
                    <td>
                      <div className="flex flex-wrap gap-1">
                        {(a.tools ?? []).slice(0, 3).map((t) => (
                          <span key={t} className="tag">
                            {t}
                          </span>
                        ))}
                        {(a.tools ?? []).length > 3 && (
                          <span className="muted text-[12px]">+{(a.tools ?? []).length - 3}</span>
                        )}
                        {(a.tools ?? []).length === 0 && <span className="muted">–</span>}
                      </div>
                    </td>
                    <td className="num">
                      <div className="flex justify-end gap-1">
                        {a.needsSecret && (
                          <Chip tone="warn">
                            <KeyRound size={11} /> add credential
                          </Chip>
                        )}
                        {a.flagged > 0 && <Chip tone="warn">{a.flagged} to check</Chip>}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </Panel>
      )}
    </div>
  );
}
