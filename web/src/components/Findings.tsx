import { CircleAlert, CircleX, ExternalLink, Lightbulb } from "lucide-react";
import type { Finding } from "../lib/types";
import { Chip } from "./ui";

export const LEVELS = {
  block: { label: "Blocks save", tone: "bad" as const, icon: CircleX, color: "var(--bad)" },
  warn: { label: "Warning", tone: "warn" as const, icon: CircleAlert, color: "var(--warn)" },
  hint: { label: "Hint", tone: "info" as const, icon: Lightbulb, color: "var(--info)" },
};

const order = { block: 0, warn: 1, hint: 2 };

/** FindingList shows guideline findings, worst first (GUIDE-2, GUIDE-3). */
export function FindingList({
  findings,
  showTool,
  empty,
}: {
  findings: Finding[];
  showTool?: boolean;
  empty?: string;
}) {
  if (findings.length === 0)
    return <div className="muted px-4 py-5 text-center text-[13px]">{empty ?? "Nothing to fix."}</div>;
  const sorted = [...findings].sort((a, b) => order[a.level] - order[b.level]);
  return (
    <div>
      {sorted.map((f, i) => {
        const L = LEVELS[f.level];
        return (
          <div className="finding" key={`${f.rule}-${f.tool ?? ""}-${f.path ?? ""}-${i}`}>
            <L.icon size={16} className="lvl" style={{ color: L.color }} aria-label={L.label} />
            <div className="min-w-0 flex-1">
              <div className="flex flex-wrap items-center gap-2">
                <span className="t">{f.title}</span>
                {showTool && f.tool && <span className="tag mono">{f.tool}</span>}
                <Chip tone={L.tone}>{L.label}</Chip>
              </div>
              <div className="d">{f.detail}</div>
              <div className="fix">
                <strong className="text-2">Fix:</strong> {f.fix}
                {f.source && (
                  <a href={f.source} target="_blank" rel="noreferrer" className="ml-2 inline-flex items-center gap-1">
                    Why <ExternalLink size={11} />
                  </a>
                )}
              </div>
            </div>
          </div>
        );
      })}
    </div>
  );
}

export function counts(findings: Finding[]) {
  return {
    block: findings.filter((f) => f.level === "block").length,
    warn: findings.filter((f) => f.level === "warn").length,
    hint: findings.filter((f) => f.level === "hint").length,
  };
}

export function ScoreRing({ score }: { score: number }) {
  const color = score >= 85 ? "var(--ok-dot)" : score >= 60 ? "var(--warn-dot)" : "var(--bad-dot)";
  return (
    <div
      className="score-ring"
      style={{ ["--p" as string]: score, ["--score-color" as string]: color }}
      role="img"
      aria-label={`Score ${score} of 100`}
    >
      {score}
    </div>
  );
}
