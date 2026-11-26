import { useQuery } from "@tanstack/react-query";
import { BookOpen, ExternalLink } from "lucide-react";
import { LEVELS } from "../components/Findings";
import { Chip, PageHead } from "../components/ui";
import { get } from "../lib/api";
import type { LintRules } from "../lib/types";

/** GuidelinesBody is the rule catalog the linter runs, from GET /api/lint/rules (GUIDE-1, LINT-1). */
export function GuidelinesBody() {
  const { data, error } = useQuery({
    queryKey: ["lint-rules"],
    queryFn: () => get<LintRules>("/api/lint/rules"),
    staleTime: Infinity,
  });
  if (error) return <div className="panel p-5 text-[13px]">Couldn't load the rules: {String(error)}</div>;
  if (!data) return <div className="muted p-5 text-[13px]">Loading…</div>;
  const sourceOf = Object.fromEntries(data.sources.map((s) => [s.key, s]));
  return (
    <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_300px]">
      <div className="flex min-w-0 flex-col gap-5">
        {data.sections.map((sec, i) => (
          <section key={sec} className="panel">
            <div className="px-5 pt-4 pb-3">
              <div className="muted mb-1 text-[12px] font-semibold">{i + 1}</div>
              <h2 className="text-[15px] font-semibold">{sec}</h2>
            </div>
            <table className="table">
              <tbody>
                {data.rules
                  .filter((r) => r.section === sec)
                  .map((r) => {
                    const L = LEVELS[r.severity];
                    return (
                      <tr key={r.id}>
                        <td style={{ width: 120 }}>
                          <Chip tone={L.tone}>{L.label}</Chip>
                        </td>
                        <td>
                          <div className="font-medium">
                            {r.title} <span className="muted mono text-[11.5px]">{r.id}</span>
                          </div>
                          <div className="muted text-[12.5px]">{r.why}</div>
                        </td>
                        <td className="text-2 text-[12.5px]" style={{ width: "34%" }}>
                          {r.fix}
                          <div className="mt-1 flex flex-wrap gap-2">
                            {r.sources.map((k) => (
                              <a
                                key={k}
                                href={sourceOf[k]?.url}
                                target="_blank"
                                rel="noreferrer"
                                className="text-[11.5px]"
                              >
                                {k}
                              </a>
                            ))}
                          </div>
                        </td>
                      </tr>
                    );
                  })}
              </tbody>
            </table>
          </section>
        ))}
      </div>
      <aside className="flex flex-col gap-4">
        <div className="panel p-5">
          <h3 className="mb-2 text-[13.5px] font-semibold">How mcpDeck uses these</h3>
          <ul className="text-2 m-0 pl-4 text-[13px]">
            <li className="mb-1.5">The tool editor checks the draft as you type.</li>
            <li className="mb-1.5">Saving runs the same checks; a blocking finding stops the save.</li>
            <li className="mb-1.5">
              <code>mcpdeck lint</code> and the admin MCP <code>lint</code> tool give the same findings, for CI and
              agents.
            </li>
            <li>No model is involved; the same input always gives the same findings.</li>
          </ul>
        </div>
        <div className="panel p-5">
          <h3 className="mb-2 text-[13.5px] font-semibold">Sources</h3>
          <ul className="m-0 flex list-none flex-col gap-2 p-0 text-[13px]">
            {data.sources.map((s) => (
              <li key={s.key}>
                <a href={s.url} target="_blank" rel="noreferrer" className="inline-flex items-start gap-1.5">
                  {s.title} <ExternalLink size={12} className="mt-1 flex-none" />
                </a>
              </li>
            ))}
          </ul>
        </div>
      </aside>
    </div>
  );
}

export function Guidelines() {
  return (
    <div className="page">
      <PageHead
        icon={
          <span className="head-icon">
            <BookOpen size={18} />
          </span>
        }
        title="MCP guidelines"
        sub="What makes a toolkit good for agents. mcpDeck checks these as you build and when you save. A blocking rule stops the save; warnings and hints only advise."
      />
      <GuidelinesBody />
    </div>
  );
}
