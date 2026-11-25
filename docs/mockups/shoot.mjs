// Screenshots of every mockup screen, and a smoke test of the main flows.
// Needs Node and Playwright with Chromium.
//
//   node docs/mockups/shoot.mjs            # writes docs/mockups/screens/*.png
//   node docs/mockups/shoot.mjs --check    # flows and console errors only
//
// Fails if any page logs a console error or a flow doesn't reach its end state.
import { mkdirSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const { chromium } = require("playwright");
const dir = dirname(fileURLToPath(import.meta.url));
const url = pathToFileURL(join(dir, "index.html")).href;
const check = process.argv.includes("--check");
const out = join(dir, "screens");
mkdirSync(out, { recursive: true });

const browser = await chromium.launch();
const page = await browser.newPage({ viewport: { width: 1440, height: 900 }, deviceScaleFactor: check ? 1 : 2 });
const errors = [];
page.on("console", (m) => m.type() === "error" && !/fonts\.g/.test(m.text()) && errors.push(m.text()));
page.on("pageerror", (e) => errors.push(e.message));

const go = async (hash) => { await page.goto(url + hash); await page.waitForTimeout(150); };
const shot = async (name, full = true) => {
  if (check) return;
  const tag = await page.addStyleTag({ content: ".mockbar,.toasts{display:none!important}" });
  await page.screenshot({ path: join(out, name + ".jpg"), fullPage: full, type: "jpeg", quality: 82 });
  await tag.evaluate((el) => el.remove());
};
const expect = async (sel, what) => { try { await page.locator(sel).first().waitFor({ state: "visible", timeout: 4000 }); } catch { throw new Error(`flow "${what}": ${sel} not visible`); } };

const fx = (f) => join(dir, "fixtures", f);

// 1. New user: first run, and the demo world kept apart.
await go("#home");
await page.click('[data-a=mockstate][data-v=new]');
await shot("01-home-first-run");
for (const h of ["apis", "toolkits", "agents", "activity"]) { await go("#" + h); }
await go("#home");
await page.click("[data-a=loadDemo]");
await expect(".demobar", "open demo");
await shot("08-demo-world", false);
await page.click("[data-a=leaveDemo]");
await expect("text=Turn the APIs you already have", "leave demo");

// 2. Add an API: every source.
await page.click("text=Add API >> nth=0");
await expect("text=Import from", "add api page");
await page.fill("#imp-url", "https://petstore3.swagger.io/");
await page.click("[data-a=impLoad]");
await expect("text=19 requests found", "swagger page");
await shot("02a-import-swagger");
await page.click("[data-a=impSrc][data-v=openapi]");
await page.setInputFiles("#imp-file", fx("acme-crm.openapi.json"));
await expect("text=5 requests found", "openapi file");
await shot("02b-import-openapi");
await page.click("[data-a=impSrc][data-v=postman]");
await page.setInputFiles("#imp-file", fx("acme-support.postman_collection.json"));
await expect("text=5 requests found", "postman collection");
await page.setInputFiles("#imp-envfile", fx("acme-support.postman_environment.json"));
await expect("text=become encrypted credentials", "postman environment");
await shot("02c-import-postman");
await page.click("[data-a=impSrc][data-v=blank]");
await shot("02e-import-blank");
await page.click("[data-a=impSrc][data-v=curl]");
await page.fill("#imp-curl", "curl 'https://erp.internal.example/api/orders/1042?api_key=abc' -H 'Accept: application/json'\ncurl -X POST https://erp.internal.example/api/orders -H 'Authorization: Bearer xyz' -d '{\"sku\":\"A1\"}'");
await expect("text=2 requests found", "curl live preview");
await page.fill("#imp-name", "erp");
await shot("02d-import-curl");
await page.click("[data-a=impGo]");
await expect("text=with 2 requests from pasted curl", "import curl");
await shot("03-api-after-import");

// 3. Getting started, new toolkit, connect an agent.
await go("#home");
await expect("text=Getting started", "getting started card");
await shot("04-home-getting-started");
await page.click(".setup [data-a=newToolkit]");
await page.fill("#tk-name", "orders");
await page.fill("#tk-desc", "Order lookups for the support team.");
await shot("05-new-toolkit", false);
await page.click("[data-a=tkCreate]");
await expect("text=MCP endpoint", "create toolkit");
await page.click("[data-a=connect] >> nth=0");
await shot("06-connect-pick-client", false);
await page.click("[data-a=cCreate]");
await expect("text=This token is shown once", "connect");
await page.click(".modal >> text=Write config");
await page.waitForTimeout(2700);
await expect("text=Connected.", "connect waits for client");
await shot("07-connect-done", false);
await page.click(".modal >> text=Done");
await go("#home");
await expect("text=3 of 4 done", "getting started progress");
await shot("09-home-getting-started-3");

// 4. Demo world: every page.
await page.click('[data-a=mockstate][data-v=used]');
const pages = [
  ["10-home", "#home"], ["11-apis", "#apis"], ["12-api-requests", "#api-support"], ["16-toolkits", "#toolkits"], ["17-all-tools", "#tools"],
  ["18-toolkit", "#toolkit-support"], ["22-tool-editor", "#tool-customer_360"], ["26-review", "#review-reassign_ticket"],
  ["27-agents", "#agents"], ["28-agent", "#agent-claude-code"], ["29-activity-calls", "#activity"], ["31-activity-usage", "#usage"], ["32-activity-audit", "#audit"], ["33-settings", "#preferences"], ["38-about", "#about"],
];
for (const [name, hash] of pages) { await go(hash); await shot(name); }

// API page tabs and a send.
await go("#api-legacy");
await shot("13-api-legacy-check-access");
await go("#api-billing");
await page.click("[data-a=send]");
await page.waitForTimeout(500);
await expect("text=Save as example", "send request");
await page.click('[data-a=tab][data-v=credentials]');
await shot("14-api-credentials");
await page.click('[data-a=tab][data-v=environments]');
await shot("15-api-environments");

// Toolkit tabs.
await go("#toolkit-support");
await page.click('[data-a=tab][data-v=agents]');
await shot("19-toolkit-agents");
await page.click('[data-a=tab][data-v=requests]');
await shot("20-toolkit-requests");
await page.click('[data-a=tab][data-v=settings]');
await shot("21-toolkit-settings");

// Tool editor: autocomplete, run, versions, fields.
await go("#tool-customer_360");
await page.click("[data-a=runTool] >> nth=1");
await expect("text=Output to the agent", "test run");
await shot("23-tool-test-run");
await page.click('[data-a=tab][data-v=versions]');
await shot("24-tool-versions");
await page.click('[data-a=tab][data-v=fields]');
await shot("25-tool-fields");
await page.click('[data-a=tab][data-v=code]');
await page.click("#code");
await page.keyboard.press("Control+End");
await page.keyboard.type("\ncrm.");
await expect(".complete", "autocomplete");
await shot("22b-tool-autocomplete", false);
await page.keyboard.press("Escape");

// New tool, ask popover, suggestion, palette, call detail.
await go("#toolkit-finance");
await page.click("[data-a=newTool] >> nth=0");
await shot("34-new-tool-ask-agent", false);
await page.keyboard.press("Escape");
await page.click("[data-a=ask]");
await shot("35-ask-an-agent", false);
await go("#home");
await page.click("[data-a=suggestion] >> nth=0");
await shot("36-suggestion", false);
await page.keyboard.press("Escape");
await page.keyboard.press("Control+k");
await page.keyboard.type("custom");
await shot("37-command-palette", false);
await page.keyboard.press("Escape");
await go("#activity");
await page.click("text=run_api billing.createCredit");
await shot("30-activity-blocked-call");

// Approve the pending write tool.
await go("#review-reassign_ticket");
await page.click("[data-a=approve]");
await expect("text=reassign_ticket", "approve");

// Dark mode and phone width.
await page.emulateMedia({ colorScheme: "dark" });
await go("#home");
await page.reload();
await shot("40-home-dark");
await go("#tool-customer_360");
await page.click("[data-a=runTool] >> nth=1");
await shot("41-tool-editor-dark");
await page.emulateMedia({ colorScheme: "light" });
await page.setViewportSize({ width: 400, height: 860 });
await go("#home");
await shot("42-home-phone");
const wide = await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth + 1);
if (wide) errors.push("page scrolls sideways at 400px");

await browser.close();
if (errors.length) { console.error(errors.join("\n")); process.exit(1); }
console.log(check ? "flows ok, no console errors" : `screens written to ${out}`);
