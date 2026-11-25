import { CircleAlert, CircleX, Lightbulb } from "lucide-react";
import type { Finding } from "../lib/types";
import { Chip } from "./ui";

export const LEVELS = {
  must: { label: "Must fix", tone: "bad" as const, icon: CircleX, color: "var(--bad)" },
  avoid: { label: "Avoid", tone: "warn" as const, icon: CircleAlert, color: "var(--warn)" },
  tip: { label: "Tip", tone: "info" as const, icon: Lightbulb, color: "var(--info)" },
};

const order = { must: 0, avoid: 1, tip: 2 };

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
    must: findings.filter((f) => f.level === "must").length,
    avoid: findings.filter((f) => f.level === "avoid").length,
    tip: findings.filter((f) => f.level === "tip").length,
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
