// Tiny static renderer for the .dc.html mockups: runs renderVals(), expands <sc-for> and {{holes}}, screenshots.
const fs = require('fs'), path = require('path');
const { chromium } = require('playwright');
const [src, out] = process.argv.slice(2);
const get = (o, p) => p.trim().split('.').reduce((a, k) => (a == null ? a : a[k]), o);
function fill(tpl, scope) {
  tpl = tpl.replace(/<sc-for list="\{\{\s*([\w.]+)\s*\}\}" as="(\w+)"[^>]*>([\s\S]*?)<\/sc-for>/g,
    (_, list, as, body) => (get(scope, list) || []).map((it, i) => fill(body, { ...scope, [as]: it, $index: i })).join(''));
  tpl = tpl.replace(/(\w+)="\{\{\s*([\w.$]+)\s*\}\}"/g, (m, attr, p) => {
    const v = get(scope, p);
    if (typeof v === 'boolean') return v ? attr : '';
    return `${attr}="${v ?? ''}"`;
  });
  return tpl.replace(/\{\{\s*([\w.$]+)\s*\}\}/g, (_, p) => get(scope, p) ?? '');
}
(async () => {
  const html = fs.readFileSync(src, 'utf8');
  const body = html.match(/<x-dc>([\s\S]*)<\/x-dc>/)[1];
  const helmet = (body.match(/<helmet>([\s\S]*?)<\/helmet>/) || [, ''])[1];
  const tpl = body.replace(/<helmet>[\s\S]*?<\/helmet>/, '');
  const code = html.match(/data-dc-script[^>]*>([\s\S]*?)<\/script>/)[1];
  const C = new Function('DCLogic', code + '; return Component;')(class { constructor() { this.props = {}; this.state = {}; } setState() {} });
  const vals = new C().renderVals();
  const page = `<!doctype html><html><head><meta charset="utf-8">${helmet}</head><body>${fill(tpl, vals)}</body></html>`;
  const b = await chromium.launch();
  const p = await b.newPage({ viewport: { width: 1440, height: 920 }, deviceScaleFactor: 2 });
  await p.setContent(page, { waitUntil: 'networkidle', timeout: 15000 }).catch(() => {});
  await p.waitForTimeout(300);
  await p.screenshot({ path: out, fullPage: true });
  await b.close();
})();
