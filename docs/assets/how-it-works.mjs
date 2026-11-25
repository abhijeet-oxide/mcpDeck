// Generates docs/assets/how-it-works-{light,dark}.svg. Run: node docs/assets/how-it-works.mjs
import { writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const themes = {
  light: {
    bg: "#F2F4F8", panel: "#FFFFFF", panelLine: "#E2E5EB", card: "#F6F7F9", cardLine: "#E4E7EC",
    text: "#1D1D1F", muted: "#6E6E73", accent: "#0071E3", line: "#9AA0AA",
    ink: "#1D1D1F", inkText: "#FFFFFF", inkMuted: "#B8B8BE", inkPill: "rgba(255,255,255,.13)",
    get: ["#1F7A35", "rgba(52,199,89,.16)"], post: ["#0058B0", "rgba(0,113,227,.12)"],
    tip: "#FFFFFF", tipLine: "#E2E5EB",
  },
  dark: {
    bg: "#0D1117", panel: "#161B22", panelLine: "#30363D", card: "#1C2128", cardLine: "#30363D",
    text: "#E6EDF3", muted: "#8B949E", accent: "#4493F8", line: "#6E7681",
    ink: "#E6EDF3", inkText: "#0D1117", inkMuted: "#57606A", inkPill: "rgba(13,17,23,.09)",
    get: ["#56D364", "rgba(46,160,67,.18)"], post: ["#79C0FF", "rgba(56,139,253,.18)"],
    tip: "#1C2128", tipLine: "#30363D",
  },
};

const font = `font-family="Inter, -apple-system, 'Segoe UI', system-ui, sans-serif"`;
const mono = `font-family="'JetBrains Mono', ui-monospace, 'SF Mono', Consolas, monospace"`;

function svg(t) {
  const txt = (x, y, s, { size = 13, w = 400, c = t.text, f = font, a = "start" } = {}) =>
    `<text x="${x}" y="${y}" ${f} font-size="${size}" font-weight="${w}" fill="${c}" text-anchor="${a}">${s}</text>`;
  const card = (x, y, w, h, r = 14) =>
    `<rect x="${x}" y="${y}" width="${w}" height="${h}" rx="${r}" fill="${t.card}" stroke="${t.cardLine}"/>`;
  const chip = (x, y, label, [fg, bg]) => {
    const w = label.length * 7 + 12;
    return `<rect x="${x}" y="${y - 12}" width="${w}" height="17" rx="5" fill="${bg}"/>` +
      txt(x + w / 2, y, label, { size: 10.5, w: 600, c: fg, f: mono, a: "middle" });
  };
  const arrow = (d) => `<path d="${d}" fill="none" stroke="${t.line}" stroke-width="1.5" marker-end="url(#a)"/>`;
  const line = (d) => `<path d="${d}" fill="none" stroke="${t.line}" stroke-width="1.5"/>`;

  const systems = [
    ["Swagger page", "or an OpenAPI file"],
    ["Postman collection", "folders, auth, examples"],
    ["curl", "paste one or many"],
    ["Legacy service", "SOAP, XML, cookie login"],
  ];
  const agents = [
    ["Claude Code", "claude mcp add …"],
    ["VS Code + Copilot", ".vscode/mcp.json"],
    ["Cursor", "~/.cursor/mcp.json"],
    ["Claude Desktop", "through the stdio bridge"],
  ];
  const rowY = (i) => 96 + i * 80;

  const out = [];
  out.push(`<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="560" viewBox="0 0 1200 560" role="img" aria-label="How mcpDeck works: import your systems as APIs, write tools in TypeScript, group them into toolkits that agents add as MCP servers. Every call goes through one proxy and is recorded.">`);
  out.push(`<defs><marker id="a" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M1 1L9 5L1 9" fill="none" stroke="${t.line}" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/></marker></defs>`);
  out.push(`<rect width="1200" height="560" rx="24" fill="${t.bg}"/>`);

  out.push(`<rect x="316" y="40" width="560" height="480" rx="24" fill="${t.panel}" stroke="${t.panelLine}"/>`);

  // Left: your systems
  out.push(txt(32, 70, "YOUR SYSTEMS", { size: 11, w: 600, c: t.muted }));
  systems.forEach(([a, b], i) => {
    const y = rowY(i);
    out.push(card(32, y, 232, 64), txt(50, y + 28, a, { w: 600 }), txt(50, y + 47, b, { size: 12, c: t.muted }));
    out.push(line(`M264 ${y + 32}H290`));
  });
  out.push(line(`M290 ${rowY(0) + 32}V${rowY(3) + 32}`));
  out.push(arrow(`M290 180H336`), txt(297, 172, "import", { size: 11, c: t.muted }));

  // Middle: mcpDeck
  out.push(txt(338, 72, "mcpDeck", { size: 15, w: 700 }), txt(414, 72, "one binary · runs on your laptop · SQLite", { size: 12, c: t.muted }));

  // Stage 1: APIs
  out.push(card(338, 96, 160, 166), txt(354, 122, "APIs", { size: 14, w: 700 }), txt(354, 140, "like Postman collections", { size: 11, c: t.muted }));
  out.push(chip(354, 168, "GET", t.get), txt(392, 168, "/accounts/{id}", { size: 10.5, f: mono }));
  out.push(chip(354, 192, "POST", t.post), txt(399, 192, "/search", { size: 10.5, f: mono }));
  out.push(txt(354, 220, "{{tenantId}}", { size: 10.5, f: mono, c: t.accent }));
  out.push(txt(354, 244, "keys encrypted", { size: 11, c: t.muted }));
  out.push(arrow(`M498 180H508`));

  // Stage 2: Tools
  out.push(card(512, 96, 160, 166), txt(528, 122, "Tools", { size: 14, w: 700 }), txt(528, 140, "TypeScript, versioned", { size: 11, c: t.muted }));
  [["await", " crm.getAccount()", t.accent], ["await", " jira.search()", t.accent], ["return", " { name, … }", t.text]].forEach(([k, r, c], i) => {
    out.push(`<text x="528" y="${168 + i * 22}" ${mono} font-size="10.5" fill="${t.muted}">${k}<tspan fill="${c}">${r}</tspan></text>`);
  });
  out.push(txt(528, 244, "v3 · live", { size: 11, c: t.muted }));
  out.push(arrow(`M672 180H682`));

  // Stage 3: Toolkits
  out.push(card(686, 96, 160, 166), txt(702, 122, "Toolkits", { size: 14, w: 700 }), txt(702, 140, "each is an MCP server", { size: 11, c: t.muted }));
  [["support", "/mcp/support"], ["finance", "/mcp/finance"]].forEach(([n, p], i) => {
    const y = 156 + i * 40;
    out.push(`<rect x="702" y="${y}" width="128" height="32" rx="9" fill="${t.panel}" stroke="${t.cardLine}"/>`);
    out.push(txt(712, y + 14, n, { size: 11.5, w: 600 }), txt(712, y + 27, p, { size: 10, f: mono, c: t.muted }));
  });
  out.push(txt(702, 244, "only the tools you pick", { size: 11, c: t.muted }));

  // Calls and suggestions
  out.push(card(338, 290, 508, 98, 16));
  out.push(txt(354, 316, "Calls and suggestions", { size: 14, w: 700 }), txt(510, 316, "who called what, when, how long, how many tokens", { size: 11.5, c: t.muted }));
  let cx = 354;
  for (const s of ["merge repeated calls", "trim large outputs", "run calls in parallel"]) {
    const w = s.length * 6.6 + 22;
    out.push(`<rect x="${cx}" y="338" width="${w}" height="28" rx="14" fill="${t.panel}" stroke="${t.cardLine}"/>`, txt(cx + w / 2, 356, s, { size: 11, a: "middle" }));
    cx += w + 8;
  }
  out.push(txt(830, 380, "rules, no LLM", { size: 10.5, c: t.muted, a: "end" }));
  out.push(arrow(`M592 290V266`), txt(600, 281, "improve", { size: 11, c: t.muted }));

  // Proxy
  out.push(`<rect x="338" y="408" width="508" height="90" rx="16" fill="${t.ink}"/>`);
  out.push(txt(354, 434, "Proxy", { size: 14, w: 700, c: t.inkText }), txt(402, 434, "the only way out · allow-listed hosts only", { size: 11.5, c: t.inkMuted }));
  cx = 354;
  for (const s of ["read-only can't write", "keys added here", "every call logged"]) {
    const w = s.length * 6.6 + 24;
    out.push(`<rect x="${cx}" y="452" width="${w}" height="28" rx="14" fill="${t.inkPill}"/>`, txt(cx + w / 2, 470, s, { size: 11, c: t.inkText, a: "middle" }));
    cx += w + 8;
  }
  out.push(arrow(`M338 470H148V406`), txt(156, 492, "real calls to your systems", { size: 11, c: t.muted }));

  // Right: agents
  out.push(txt(928, 70, "YOUR AGENTS", { size: 11, w: 600, c: t.muted }));
  agents.forEach(([a, b], i) => {
    const y = rowY(i);
    out.push(card(928, y, 240, 64), txt(946, y + 28, a, { w: 600 }), txt(946, y + 47, b, { size: 11, c: t.muted, f: mono }));
    out.push(line(`M887 ${y + 32}H928`));
  });
  out.push(line(`M887 ${rowY(0) + 32}V${rowY(3) + 32}`));
  out.push(arrow(`M887 180H850`), txt(893, 172, "MCP", { size: 11, w: 600, c: t.muted }));

  out.push(`</svg>`);
  return out.join("\n");
}

const dir = dirname(fileURLToPath(import.meta.url));
for (const [name, t] of Object.entries(themes)) {
  writeFileSync(join(dir, `how-it-works-${name}.svg`), svg(t) + "\n");
}
