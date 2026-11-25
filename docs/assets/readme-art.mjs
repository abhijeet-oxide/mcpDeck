// Generates the README banner (light and dark) and feature icons.
// Run: node docs/assets/readme-art.mjs
import { mkdirSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const dir = dirname(fileURLToPath(import.meta.url));
const font = `font-family="Inter, -apple-system, 'Segoe UI', system-ui, sans-serif"`;
const mono = `font-family="'JetBrains Mono', ui-monospace, 'SF Mono', Consolas, monospace"`;

const themes = {
  light: {
    bg: "#F4F6FA", glow1: "#BFD4FF", glow2: "#D5E3EC", glow3: "#FFDCC8",
    text: "#1D1D1F", muted: "#5F6368", card: "#FFFFFF", cardLine: "#E3E6EC", shadow: "rgba(20,30,60,.10)",
    tile: "#1D1D1F", front: "#FFFFFF", edge: "#1D1D1F", accent: "#0071E3",
    get: ["#1F7A35", "rgba(52,199,89,.16)"], post: ["#0058B0", "rgba(0,113,227,.12)"], ok: ["#1F7A35", "rgba(52,199,89,.16)"],
  },
  dark: {
    bg: "#0D1117", glow1: "#1B3A6B", glow2: "#16323A", glow3: "#4A2A1A",
    text: "#E6EDF3", muted: "#9BA6B2", card: "#161B22", cardLine: "#30363D", shadow: "rgba(0,0,0,.45)",
    tile: "#E6EDF3", front: "#0D1117", edge: "#0D1117", accent: "#4493F8",
    get: ["#56D364", "rgba(46,160,67,.18)"], post: ["#79C0FF", "rgba(56,139,253,.18)"], ok: ["#56D364", "rgba(46,160,67,.18)"],
  },
};

function chip(x, y, label, [fg, bg], size = 11) {
  const w = label.length * size * 0.62 + 14;
  return `<rect x="${x}" y="${y - size - 3}" width="${w}" height="${size + 9}" rx="6" fill="${bg}"/>` +
    `<text x="${x + w / 2}" y="${y}" ${mono} font-size="${size}" font-weight="600" fill="${fg}" text-anchor="middle">${label}</text>`;
}

function toolCard(t, { x, y, rot, name, steps, status }) {
  const w = 300;
  const h = 170;
  const rows = steps.map(([m, api, path], i) => {
    const yy = y + 78 + i * 26;
    const colors = m === "GET" ? t.get : t.post;
    return chip(x + 20, yy, m, colors) +
      `<text x="${x + 76}" y="${yy}" ${mono} font-size="12" fill="${t.text}">${api}<tspan fill="${t.muted}"> ${path}</tspan></text>`;
  }).join("");
  return `<g transform="rotate(${rot} ${x + w / 2} ${y + h / 2})" filter="url(#shadow)">
    <rect x="${x}" y="${y}" width="${w}" height="${h}" rx="18" fill="${t.card}" stroke="${t.cardLine}"/>
    <text x="${x + 20}" y="${y + 36}" ${mono} font-size="16" font-weight="700" fill="${t.text}">${name}</text>
    ${chip(x + w - 20 - (status.length * 11 * 0.62 + 14), y + 35, status, t.ok)}
    <line x1="${x + 20}" y1="${y + 52}" x2="${x + w - 20}" y2="${y + 52}" stroke="${t.cardLine}"/>
    ${rows}
  </g>`;
}

function banner(t) {
  const W = 1280;
  const H = 420;
  return `<svg xmlns="http://www.w3.org/2000/svg" width="${W}" height="${H}" viewBox="0 0 ${W} ${H}" role="img" aria-label="mcpDeck: build MCP tools from the APIs you already have, and control what every agent can do.">
  <defs>
    <filter id="blur" x="-50%" y="-50%" width="200%" height="200%"><feGaussianBlur stdDeviation="70"/></filter>
    <filter id="shadow" x="-20%" y="-20%" width="140%" height="160%"><feDropShadow dx="0" dy="14" stdDeviation="16" flood-color="${t.shadow}"/></filter>
    <clipPath id="clip"><rect width="${W}" height="${H}" rx="28"/></clipPath>
  </defs>
  <g clip-path="url(#clip)">
    <rect width="${W}" height="${H}" fill="${t.bg}"/>
    <g filter="url(#blur)" opacity=".9">
      <circle cx="1000" cy="120" r="190" fill="${t.glow1}"/>
      <circle cx="760" cy="380" r="170" fill="${t.glow2}"/>
      <circle cx="1210" cy="400" r="150" fill="${t.glow3}"/>
    </g>
    <svg x="72" y="92" width="72" height="72" viewBox="8 8 56 56">
      <rect x="14.6" y="19.1" width="24.8" height="33.8" rx="5" fill="#0071E3" transform="rotate(-12 27 36)"/>
      <g transform="rotate(9 42.8 33.8)">
        <rect x="30.4" y="16.9" width="24.8" height="33.8" rx="5" fill="${t.edge}"/>
        <rect x="32.1" y="18.6" width="21.4" height="30.4" rx="4" fill="#FFFFFF"/>
        <g fill="none" stroke="${t.edge}" stroke-width="2.9" stroke-linecap="round" stroke-linejoin="round"><path d="M38.3 28.8l-3.6 5 3.6 5"/><path d="M47.3 28.8l3.6 5-3.6 5"/></g>
      </g>
    </svg>
    <text x="164" y="146" ${font} font-size="54" font-weight="700" letter-spacing="-1.5" fill="${t.text}">mcpDeck</text>
    <text x="72" y="236" ${font} font-size="30" font-weight="600" letter-spacing="-.6" fill="${t.text}">Build MCP tools from the APIs</text>
    <text x="72" y="274" ${font} font-size="30" font-weight="600" letter-spacing="-.6" fill="${t.text}">you already have.</text>
    <text x="72" y="320" ${font} font-size="20" fill="${t.muted}">Control what every agent can do.</text>
    <text x="72" y="350" ${font} font-size="20" fill="${t.muted}">Watch every call. Make tools smaller.</text>
    ${toolCard(t, { x: 690, y: 150, rot: -9, name: "open_tickets", steps: [["POST", "jira", "/search"]], status: "read" })}
    ${toolCard(t, { x: 820, y: 112, rot: 4, name: "overdue_invoices", steps: [["GET", "crm", "/accounts/{id}"], ["GET", "billing", "/invoices"]], status: "read" })}
    ${toolCard(t, { x: 905, y: 182, rot: -2, name: "customer_360", steps: [["GET", "crm", "/accounts/{id}"], ["GET", "billing", "/summary"], ["POST", "jira", "/search"]], status: "v3 · live" })}
  </g>
</svg>
`;
}

// Feature icons: 24px strokes on a 44px tinted tile. Mid blue reads on light and dark.
const icons = {
  api: '<path d="M9 2v5M15 2v5"/><path d="M6 7h12v4a6 6 0 0 1-12 0z"/><path d="M12 17v5"/>',
  code: '<path d="M8 6l-6 6 6 6"/><path d="M16 6l6 6-6 6"/><path d="M14 4l-4 16"/>',
  bolt: '<path d="M13 2L4 14h7l-1 8 9-12h-7z"/>',
  layers: '<path d="M12 2l10 5-10 5L2 7z"/><path d="M2 12l10 5 10-5"/><path d="M2 17l10 5 10-5"/>',
  key: '<circle cx="7.5" cy="15.5" r="4.5"/><path d="M10.7 12.3L21 2"/><path d="M16 7l3 3"/><path d="M18.5 4.5l2 2"/>',
  shield: '<path d="M12 2l8 3v6c0 5-3.4 9.3-8 11-4.6-1.7-8-6-8-11V5z"/><path d="M9 12l2 2 4-4"/>',
  share: '<circle cx="18" cy="5" r="3"/><circle cx="6" cy="12" r="3"/><circle cx="18" cy="19" r="3"/><path d="M8.6 13.5l6.8 4"/><path d="M15.4 6.5l-6.8 4"/>',
  activity: '<path d="M3 12h4l3-8 4 16 3-8h4"/>',
};
const icon = (body) => `<svg xmlns="http://www.w3.org/2000/svg" width="44" height="44" viewBox="0 0 44 44" role="img" aria-hidden="true">
  <rect width="44" height="44" rx="12" fill="#2F7FEA" fill-opacity=".14"/>
  <g transform="translate(10 10)" fill="none" stroke="#2F7FEA" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">${body}</g>
</svg>
`;

for (const [name, t] of Object.entries(themes)) writeFileSync(join(dir, `banner-${name}.svg`), banner(t));
mkdirSync(join(dir, "icons"), { recursive: true });
for (const [name, body] of Object.entries(icons)) writeFileSync(join(dir, "icons", `${name}.svg`), icon(body));
