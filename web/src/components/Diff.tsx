// Side-by-side line diff, like a code review (VER-4).
type Line = { a?: string; b?: string; kind: "same" | "del" | "add" | "mod" };

export function diffLines(left: string, right: string): Line[] {
  const a = left.split("\n");
  const b = right.split("\n");
  const n = a.length;
  const m = b.length;
  const dp: number[][] = Array.from({ length: n + 1 }, () => new Array(m + 1).fill(0));
  for (let i = n - 1; i >= 0; i--)
    for (let j = m - 1; j >= 0; j--)
      dp[i][j] = a[i] === b[j] ? dp[i + 1][j + 1] + 1 : Math.max(dp[i + 1][j], dp[i][j + 1]);
  const out: Line[] = [];
  let i = 0;
  let j = 0;
  while (i < n || j < m) {
    if (i < n && j < m && a[i] === b[j]) {
      out.push({ a: a[i], b: b[j], kind: "same" });
      i++;
      j++;
    } else if (j < m && (i >= n || dp[i][j + 1] >= dp[i + 1][j])) {
      out.push({ b: b[j], kind: "add" });
      j++;
    } else {
      out.push({ a: a[i], kind: "del" });
      i++;
    }
  }
  return out;
}

export function Diff({ left, right }: { left: string; right: string }) {
  const lines = diffLines(left, right);
  const cell = (text: string | undefined, kind: string) => ({
    background: kind === "del" ? "var(--bad-bg)" : kind === "add" ? "var(--ok-bg)" : "transparent",
    text: text ?? "",
  });
  return (
    <div className="code" style={{ padding: 0, display: "grid", gridTemplateColumns: "1fr 1fr" }}>
      {lines.map((l, k) => {
        const L = cell(l.a, l.kind === "del" ? "del" : "");
        const R = cell(l.b, l.kind === "add" ? "add" : "");
        return [
          <div
            key={`l${k}`}
            style={{ background: L.background, padding: "0 10px", minHeight: 18, borderRight: "1px solid var(--line)" }}
          >
            {L.text}
          </div>,
          <div key={`r${k}`} style={{ background: R.background, padding: "0 10px", minHeight: 18 }}>
            {R.text}
          </div>,
        ];
      })}
    </div>
  );
}
