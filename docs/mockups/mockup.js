/* mcpDeck clickable mockup. Plain JS, no build step.
   Every screen reads from one state object (S). Data is the demo world (MOCK-3). */
"use strict";

/* ---------- helpers ---------- */
const $ = (s, r = document) => r.querySelector(s);
const $$ = (s, r = document) => [...r.querySelectorAll(s)];
const esc = (s) => String(s ?? "").replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
const isMac = /Mac|iPhone|iPad/.test(navigator.platform || navigator.userAgent);
const MOD = isMac ? "⌘" : "Ctrl";
const HOST = "http://127.0.0.1:47800";
const fmt = (n) => Number(n).toLocaleString("en-US");
const kb = (b) => (b >= 1024 ? (b / 1024).toFixed(1) + " KB" : b + " B");
const tok = (b) => Math.max(1, Math.round(b / 4)); // stands in for the named offline encoder (SES-3)
const plural = (n, w, p = w + "s") => `${fmt(n)} ${n === 1 ? w : p}`;
const deep = (o) => JSON.parse(JSON.stringify(o));
const src = (id) => ($("#src-" + id)?.textContent || "").trimEnd() + "\n";

const ICON = {
  home: '<path d="M3 10.5L12 3l9 7.5"/><path d="M5 9.5V21h14V9.5"/>',
  api: '<path d="M9 2v5M15 2v5"/><path d="M6 7h12v4a6 6 0 0 1-12 0z"/><path d="M12 17v5"/>',
  layers: '<path d="M12 2l10 5-10 5L2 7z"/><path d="M2 12l10 5 10-5"/><path d="M2 17l10 5 10-5"/>',
  agent: '<rect x="4" y="7" width="16" height="12" rx="3"/><path d="M12 3v4"/><path d="M9 12.5v1.5M15 12.5v1.5"/>',
  activity: '<path d="M3 12h4l3-8 4 16 3-8h4"/>',
  sliders: '<path d="M4 6h9M17 6h3M4 12h3M11 12h9M4 18h11M19 18h1"/><circle cx="15" cy="6" r="2"/><circle cx="9" cy="12" r="2"/><circle cx="17" cy="18" r="2"/>',
  search: '<circle cx="11" cy="11" r="7"/><path d="M20 20l-3.5-3.5"/>',
  copy: '<rect x="8" y="8" width="12" height="12" rx="2"/><path d="M16 8V6a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h2"/>',
  check: '<path d="M5 12.5l4.5 4.5L19 7.5"/>',
  x: '<path d="M6 6l12 12M18 6L6 18"/>',
  plus: '<path d="M12 5v14M5 12h14"/>',
  chat: '<path d="M4 5h16v11H10l-5 4v-4H4z"/><path d="M8 9.5h8M8 12.5h5"/>',
  play: '<path d="M7 5l12 7-12 7z"/>',
  lock: '<rect x="5" y="11" width="14" height="10" rx="2"/><path d="M8 11V8a4 4 0 0 1 8 0v3"/>',
  key: '<circle cx="7.5" cy="15.5" r="4.5"/><path d="M10.7 12.3L21 2"/><path d="M16 7l3 3"/>',
  shield: '<path d="M12 2l8 3v6c0 5-3.4 9.3-8 11-4.6-1.7-8-6-8-11V5z"/><path d="M9 12l2 2 4-4"/>',
  import: '<path d="M12 3v12"/><path d="M7 10l5 5 5-5"/><path d="M5 21h14"/>',
  alert: '<path d="M12 3l10 18H2z"/><path d="M12 10v4.5"/><path d="M12 17.5v.5"/>',
  info: '<circle cx="12" cy="12" r="9"/><path d="M12 11v5"/><path d="M12 7.5v.5"/>',
  bulb: '<path d="M9 18h6M10 21h4"/><path d="M12 3a6 6 0 0 0-4 10.5c.8.8 1 1.5 1 2.5h6c0-1 .2-1.7 1-2.5A6 6 0 0 0 12 3z"/>',
  eye: '<path d="M2 12s3.6-7 10-7 10 7 10 7-3.6 7-10 7S2 12 2 12z"/><circle cx="12" cy="12" r="3"/>',
  clock: '<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>',
  folder: '<path d="M3 6a1 1 0 0 1 1-1h5l2 2h9a1 1 0 0 1 1 1v10a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1z"/>',
  code: '<path d="M8 6l-6 6 6 6"/><path d="M16 6l6 6-6 6"/>',
  terminal: '<path d="M5 7l5 5-5 5"/><path d="M12 18h7"/>',
  link: '<path d="M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1 1"/><path d="M14 10a4 4 0 0 0-5.7 0l-3 3a4 4 0 0 0 5.7 5.7l1-1"/>',
  ban: '<circle cx="12" cy="12" r="9"/><path d="M5.6 5.6l12.8 12.8"/>',
  chev: '<path d="M9 6l6 6-6 6"/>',
  file: '<path d="M14 3H6v18h12V7z"/><path d="M14 3v4h4"/>',
};
const ic = (n, s = 16) => `<svg width="${s}" height="${s}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${ICON[n]}</svg>`;
const LOGO = `<svg width="30" height="30" viewBox="0 0 72 72" aria-hidden="true"><rect width="72" height="72" rx="17" fill="#1D1D1F"/><rect x="14.6" y="19.1" width="24.8" height="33.8" rx="5" fill="#0071E3" transform="rotate(-12 27 36)"/><rect x="30.4" y="16.9" width="24.8" height="33.8" rx="5" fill="#1D1D1F" transform="rotate(9 42.8 33.8)"/><rect x="32.1" y="18.6" width="21.4" height="30.4" rx="4" fill="#fff" transform="rotate(9 42.8 33.8)"/><g transform="rotate(9 42.8 33.8)" fill="none" stroke="#1D1D1F" stroke-width="2.9" stroke-linecap="round" stroke-linejoin="round"><path d="M38.3 28.8l-3.6 5 3.6 5"/><path d="M47.3 28.8l3.6 5-3.6 5"/></g></svg>`;

const CLIENTS = {
  "claude-code": { label: "Claude Code", short: "CC", color: "#C2603E" },
  vscode: { label: "VS Code (Copilot)", short: "VS", color: "#0A6CC2" },
  cursor: { label: "Cursor", short: "Cu", color: "#3A3A3C" },
  "claude-desktop": { label: "Claude Desktop", short: "CD", color: "#8A5A44" },
  other: { label: "Other MCP client", short: "MC", color: "#5F6B7A" },
};

/* ---------- demo world responses ---------- */
const LOREM_FREE_DESC = [
  "Customer reports that the invoice PDF for October shows the old VAT number GB 412 7789 21. Their finance team cannot book it until it is corrected. Steps: open INV-88213 in the portal, download the PDF, check the header block. Expected the VAT number from the account record. Attached: screenshot of the PDF header and the account page. Priority raised by the account owner because payment is blocked.",
  "Two users at Northwind cannot sign in to the portal since the SSO certificate rotation on Monday. Error shown: SAML response signature invalid. Other customers on the same identity provider are fine. Their IT contact is available 9 to 5 Pacific and has asked for a call before any change is made on their side.",
  "Request to raise the API rate limit for the warehouse integration from 600 to 1,200 requests per minute during the holiday peak, from 1 December to 10 January. They will send the expected traffic pattern by email. Needs approval from the account owner and the platform team.",
];
const RESP = {
  account: { id: "001Ab00000XyZ", name: "Northwind Traders", tier: "Enterprise", billingId: "CUS-20417", jiraOrg: "NWT", owner: { id: "005Q1", name: "Priya Raman", email: "priya@acme.example" }, industry: "Wholesale", employees: 820, createdAt: "2019-03-14T09:12:00Z", address: { street: "12 Harbour Rd", city: "Seattle", region: "WA", country: "US" }, links: { self: "http://127.0.0.1:9001/accounts/001Ab00000XyZ", contacts: "http://127.0.0.1:9001/accounts/001Ab00000XyZ/contacts" } },
  contacts: { items: [{ id: "003A1", name: "Lena Fischer", role: "Billing", email: "lena.fischer@northwind.example", phone: "+1 206 555 0141" }, { id: "003A2", name: "Omar Haddad", role: "IT", email: "omar.haddad@northwind.example", phone: "+1 206 555 0187" }], total: 2 },
  summary: { customerId: "CUS-20417", currency: "USD", balance: 18240.5, paymentTerms: "NET30", invoices: [{ number: "INV-88213", total: 9120.25, dueDate: "2026-10-31", overdue: true, lines: 14, pdf: "http://127.0.0.1:9002/invoices/INV-88213.pdf" }, { number: "INV-88507", total: 9120.25, dueDate: "2026-11-30", overdue: false, lines: 14, pdf: "http://127.0.0.1:9002/invoices/INV-88507.pdf" }], dunningLevel: 1, lastPaymentAt: "2026-09-29T16:40:00Z", taxId: "US-91-2245117", links: { self: "http://127.0.0.1:9002/customers/CUS-20417/summary" } },
  invoice: { number: "INV-88213", customer: "CUS-20417", status: "OVERDUE", total: { amount: 9120.25, currency: "USD" }, amountPaid: { amount: 0, currency: "USD" }, issuedAt: "2026-10-01", dueDate: "2026-10-31", lines: Array.from({ length: 6 }, (_, i) => ({ sku: "WH-" + (2100 + i), description: ["Pallet storage, per week", "Pick and pack, per order", "Cross-dock handling", "Returns processing", "Cold storage, per week", "Label printing"][i], quantity: [40, 1250, 18, 64, 12, 1250][i], unitPrice: [61.5, 2.1, 140, 7.25, 88, 0.45][i] })), links: { self: "http://127.0.0.1:9002/invoices/INV-88213", pdf: "http://127.0.0.1:9002/invoices/INV-88213.pdf" } },
  invoices: { items: [{ number: "INV-88213", status: "OVERDUE", total: { amount: 9120.25, currency: "USD" }, dueDate: "2026-10-31", issuedAt: "2026-10-01", links: { self: "http://127.0.0.1:9002/invoices/INV-88213" } }], total: 1, page: 1 },
  search: { total: 3, maxResults: 20, issues: [
    { key: "NWT-482", self: "http://127.0.0.1:9003/rest/api/2/issue/48211", fields: { summary: "Invoice PDF shows wrong VAT number", status: { name: "In Progress", id: "3" }, priority: { name: "High", id: "2" }, assignee: { displayName: "Sam Ortiz", email: "sam@acme.example" }, reporter: { displayName: "Lena Fischer" }, created: "2026-11-21T08:14:00Z", description: LOREM_FREE_DESC[0] } },
    { key: "NWT-479", self: "http://127.0.0.1:9003/rest/api/2/issue/47930", fields: { summary: "SSO sign-in fails after certificate rotation", status: { name: "Open", id: "1" }, priority: { name: "Highest", id: "1" }, assignee: null, reporter: { displayName: "Omar Haddad" }, created: "2026-11-18T15:02:00Z", description: LOREM_FREE_DESC[1] } },
    { key: "NWT-471", self: "http://127.0.0.1:9003/rest/api/2/issue/47105", fields: { summary: "Raise API rate limit for holiday peak", status: { name: "Waiting for customer", id: "10" }, priority: { name: "Medium", id: "3" }, assignee: { displayName: "Priya Raman", email: "priya@acme.example" }, reporter: { displayName: "Omar Haddad" }, created: "2026-11-10T11:47:00Z", description: LOREM_FREE_DESC[2] } },
  ] },
  issue: { key: "NWT-482", fields: { summary: "Invoice PDF shows wrong VAT number", status: { name: "In Progress" }, assignee: { displayName: "Sam Ortiz", email: "sam@acme.example" } } },
  owner: { id: "005Q1", name: "Priya Raman", email: "priya@acme.example", team: "Enterprise West" },
  accounts: { items: [{ id: "001Ab00000XyZ", name: "Northwind Traders", tier: "Enterprise" }, { id: "001Ab00000Q7k", name: "Northwind Logistics GmbH", tier: "Growth" }], total: 2 },
  hold: '<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">\n  <soap:Body>\n    <GetCreditHoldResponse xmlns="urn:acme:customer">\n      <Hold>true</Hold>\n      <Reason>Invoice INV-88213 overdue more than 21 days</Reason>\n      <Since>2026-11-21</Since>\n    </GetCreditHoldResponse>\n  </soap:Body>\n</soap:Envelope>',
  notes: '<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">\n  <soap:Body>\n    <GetCustomerNotesResponse xmlns="urn:acme:customer">\n      <Note date="2026-11-02">Agreed to split INV-88213 into two payments.</Note>\n    </GetCustomerNotesResponse>\n  </soap:Body>\n</soap:Envelope>',
  login: "HTTP/1.1 302 Found\nSet-Cookie: ASPXAUTH=[captured, stored as a credential]; HttpOnly\nLocation: /home",
  created: { ok: true },
};

/* ---------- demo world ---------- */
function R(id, folder, method, path, access, extra = {}) { return { id, folder, method, path, access, ...extra }; }
function demoWorld() {
  return {
    apis: [
      { id: "crm", title: "Acme CRM", kind: "OpenAPI 3.0", from: "http://127.0.0.1:9001/openapi.json", authType: "Bearer token", envs: ["Demo", "Staging"],
        vars: [["baseUrl", "http://127.0.0.1:9001", "https://crm.staging.acme.example", "text"], ["token", "secret", "secret", "secret"]],
        creds: { Demo: "Nov 18", Staging: null }, hosts: ["127.0.0.1:9001", "crm.staging.acme.example"], scripts: null,
        requests: [
          R("getAccount", "accounts", "GET", "/accounts/{id}", "read", { params: [["id", "001Ab00000XyZ"]], resp: "account", example: true }),
          R("searchAccounts", "accounts", "GET", "/accounts", "read", { query: [["q", "Northwind"]], resp: "accounts", example: true }),
          R("updateAccount", "accounts", "PATCH", "/accounts/{id}", "write", { params: [["id", "001Ab00000XyZ"]], body: '{\n  "tier": "Enterprise"\n}', resp: "created" }),
          R("listContacts", "contacts", "GET", "/accounts/{id}/contacts", "read", { params: [["id", "001Ab00000XyZ"]], resp: "contacts", example: true }),
          R("getOwner", "owners", "GET", "/owners/{id}", "read", { params: [["id", "005Q1"]], resp: "owner" }),
        ] },
      { id: "billing", title: "Acme Billing", kind: "Swagger 2.0", from: "http://127.0.0.1:9002/swagger.json", authType: "API key (header X-Api-Key)", envs: ["Demo", "Staging"],
        vars: [["baseUrl", "http://127.0.0.1:9002", "https://billing.staging.acme.example", "text"], ["apiKey", "secret", "secret", "secret"]],
        creds: { Demo: "Nov 18", Staging: "Nov 20" }, hosts: ["127.0.0.1:9002", "billing.staging.acme.example"], scripts: null,
        requests: [
          R("getCustomerSummary", "customers", "GET", "/customers/{id}/summary", "read", { params: [["id", "CUS-20417"]], resp: "summary", example: true }),
          R("listInvoices", "invoices", "GET", "/invoices", "read", { query: [["customer", "CUS-20417"], ["status", "OVERDUE"]], resp: "invoices", example: true }),
          R("getInvoice", "invoices", "GET", "/invoices/{number}", "read", { params: [["number", "INV-88213"]], resp: "invoice", example: true }),
          R("createCredit", "credits", "POST", "/credits", "write", { body: '{\n  "invoice": "INV-88213",\n  "amount": 250.00,\n  "reason": "Late delivery"\n}', resp: "created" }),
        ] },
      { id: "support", title: "Acme Support", kind: "Postman collection", from: "acme-support.postman_collection.json", authType: "Bearer token", envs: ["Demo", "Staging"],
        vars: [["baseUrl", "http://127.0.0.1:9003", "https://support.staging.acme.example", "text"], ["token", "secret", "secret", "secret"]],
        creds: { Demo: "Nov 18", Staging: "Nov 18" }, hosts: ["127.0.0.1:9003", "support.staging.acme.example"], scripts: { count: 3, trusted: false },
        requests: [
          R("searchIssues", "issues", "POST", "/rest/api/2/search", "read", { override: "POST, marked read: it only searches", body: '{\n  "jql": "org = \\"NWT\\" AND status != Done",\n  "maxResults": 20\n}', resp: "search", example: true }),
          R("getIssue", "issues", "GET", "/rest/api/2/issue/{key}", "read", { params: [["key", "NWT-482"]], resp: "issue", example: true }),
          R("createIssue", "issues", "POST", "/rest/api/2/issue", "write", { body: '{\n  "fields": { "project": "NWT", "summary": "…" }\n}', resp: "created" }),
          R("assignIssue", "issues", "PUT", "/rest/api/2/issue/{key}/assignee", "write", { params: [["key", "NWT-482"]], body: '{\n  "email": "dana@acme.example"\n}', resp: "created" }),
          R("addComment", "comments", "POST", "/rest/api/2/issue/{key}/comment", "write", { params: [["key", "NWT-482"]], body: '{\n  "body": "Moving to finance support"\n}', resp: "created" }),
        ] },
      { id: "legacy", title: "Acme Orders (SOAP)", kind: "curl + by hand", from: "pasted curl", authType: "Login request (cookie)", envs: ["Demo"],
        vars: [["baseUrl", "http://127.0.0.1:9004", "", "text"], ["username", "svc-mcpdeck", "", "text"], ["password", "secret", "", "secret"]],
        creds: { Demo: "Nov 19" }, hosts: ["127.0.0.1:9004"], scripts: null,
        login: { request: "login", capture: "cookie ASPXAUTH", lasts: "20 min" },
        requests: [
          R("login", "auth", "POST", "/login", "write", { body: "username={{username}}&password={{password}}", bodyType: "x-www-form-urlencoded", resp: "login" }),
          R("getCreditHold", "CustomerService", "POST", "/CustomerService.asmx", "read", { override: "SOAP call, confirmed read on Nov 19", soap: "GetCreditHold", body: '<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">\n  <soap:Body>\n    <GetCreditHold xmlns="urn:acme:customer"><AccountId>001Ab00000XyZ</AccountId></GetCreditHold>\n  </soap:Body>\n</soap:Envelope>', bodyType: "XML", resp: "hold", example: true }),
          R("getCustomerNotes", "CustomerService", "POST", "/CustomerService.asmx", "check", { soap: "GetCustomerNotes", body: '<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">\n  <soap:Body>\n    <GetCustomerNotes xmlns="urn:acme:customer"><AccountId>001Ab00000XyZ</AccountId></GetCustomerNotes>\n  </soap:Body>\n</soap:Envelope>', bodyType: "XML", resp: "notes" }),
          R("setCreditHold", "CustomerService", "POST", "/CustomerService.asmx", "write", { soap: "SetCreditHold", body: "<soap:Envelope>…</soap:Envelope>", bodyType: "XML", resp: "created" }),
        ] },
    ],
    tools: [
      T("customer_360", ["crm", "billing", "support"], "read", 3, "you", 412, 0.5, 182, 410, [{ v: 3, by: "you", when: "2 days ago", note: "Run the billing and support calls together" }, { v: 2, by: "claude-code", when: "5 days ago", note: "Return ticket keys and titles, not whole tickets" }, { v: 1, by: "you", when: "Nov 12", note: "First version" }], [["returns only overdue invoices", true], ["unknown account says what to pass", true]]),
      T("open_tickets_for_account", ["crm", "support"], "read", 2, "claude-code", 288, 1.0, 96, 220, [{ v: 2, by: "claude-code", when: "4 days ago", note: "Add priority and assignee", rules: "auto-approved: 6 of 6 rules passed" }, { v: 1, by: "claude-code", when: "6 days ago", note: "First version", rules: "auto-approved" }], [["newest first", true], ["limit is respected", true]]),
      T("invoice_status", ["billing"], "read", 1, "you", 301, 0, 64, 140, [{ v: 1, by: "you", when: "Nov 12", note: "First version" }], [["overdue invoice reads as overdue", true]]),
      T("overdue_invoices", ["crm", "billing"], "read", 1, "cursor", 97, 2.1, 344, 690, [{ v: 1, by: "cursor", when: "3 days ago", note: "First version", rules: "auto-approved: 6 of 6 rules passed" }], [["only overdue invoices", true]]),
      T("credit_hold", ["legacy"], "read", 1, "you", 41, 4.9, 512, 1240, [{ v: 1, by: "you", when: "Nov 19", note: "First version" }], [["hold is a boolean", true]]),
      T("reassign_ticket", ["support"], "write", 1, "claude-code", 0, 0, 0, 0, [{ v: 1, by: "claude-code", when: "25 min ago", note: "First version", status: "pending" }], [["moves the ticket and comments", true], ["unknown ticket says so", true]], "pending"),
    ],
    toolkits: [
      { id: "support", desc: "Customer support: one customer's account, invoices and tickets.", tools: ["customer_360", "open_tickets_for_account", "invoice_status", "reassign_ticket"], scope: ["crm", "billing", "support"], hints: true, limit: 2000,
        rules: { tests: true, desc: true, output: true, dup: true }, vars: [["tenantId", "acme-eu", "literal"]] },
      { id: "finance", desc: "Collections: overdue invoices, credit holds and who to contact.", tools: ["customer_360", "overdue_invoices", "invoice_status", "credit_hold"], scope: ["crm", "billing", "legacy"], hints: true, limit: 2000,
        rules: { tests: true, desc: true, output: true, dup: true }, vars: [] },
    ],
    agents: [
      { id: "claude-code", name: "Claude Code on this laptop", client: "claude-code", token: "mcpd_…4f2a", since: "Nov 18", where: "~/.claude.json", live: true, version: "Claude Code 2.1", today: 64, errors: 0, blocked: 0,
        grants: [{ tk: "support", access: "read_write", build: true, env: "Demo", pin: "latest" }, { tk: "finance", access: "read", build: false, env: "Demo", pin: "latest" }], drops: [] },
      { id: "vscode", name: "VS Code (Copilot)", client: "vscode", token: "mcpd_…91c0", since: "Nov 19", where: ".vscode/mcp.json in acme-portal", live: true, version: "Visual Studio Code 1.106", today: 71, errors: 1, blocked: 0,
        grants: [{ tk: "support", access: "read", build: false, env: "Demo", pin: "latest" }], drops: [] },
      { id: "cursor", name: "Cursor", client: "cursor", token: "mcpd_…7d3e", since: "Nov 20", where: "~/.cursor/mcp.json", live: false, lastSeen: "38 min ago", version: "Cursor 2.0", today: 51, errors: 2, blocked: 1,
        grants: [{ tk: "finance", access: "read", build: true, env: "Demo", pin: "latest" }], drops: [] },
    ],
    requests: [
      { id: 7, tk: "support", agent: "vscode", when: "1 h ago", text: "Refund history for an invoice. invoice_status doesn't include refunds and I can't find a tool that does.", tried: ["invoice_status ×2"], state: "open" },
      { id: 8, tk: "finance", agent: "cursor", when: "40 min ago", text: "Accounts whose credit hold changed this week. Today I can only check one account at a time with credit_hold.", tried: ["credit_hold ×6", "run_api legacy.getCreditHold ×2"], state: "open" },
    ],
    suggestions: [
      { id: "s1", rule: "INS-12", tool: "open_tickets_for_account", kind: "fields", title: "Remove issues[].fields.description from open_tickets_for_account",
        why: "Claude Code and VS Code (Copilot) reported this field unused in 23 sessions. No agent reported using it or was seen using it.", save: "≈ 1,040 B and 260 tokens per call, 288 calls in 7 days",
        evidence: [["claude-code", "unused", "14 sessions"], ["vscode", "unused", "9 sessions"]],
        diff: [["hunk", "@@ return @@"], ["", "        assignee: t.fields.assignee?.displayName ?? null,"], ["del", "        description: t.fields.description,"], ["", "      },"]] },
      { id: "s2", rule: "INS-5", tool: "overdue_invoices", kind: "parallel", title: "Run two calls in overdue_invoices at the same time",
        why: "billing.listInvoices and crm.listContacts run one after the other, and the second doesn't use the first's result.", save: "≈ 140 ms at p50 (344 ms → about 205 ms)",
        evidence: [["cursor", "97 calls", "7 days"]],
        diff: [["hunk", "@@ export default @@"], ["del", "  const invoices = await billing.listInvoices({ customer: account.billingId, status: \"OVERDUE\" });"], ["del", "  const contacts = await crm.listContacts({ id: input.accountId });"], ["add", "  const [invoices, contacts] = await Promise.all(["], ["add", "    billing.listInvoices({ customer: account.billingId, status: \"OVERDUE\" }),"], ["add", "    crm.listContacts({ id: input.accountId }),"], ["add", "  ]);"]] },
      { id: "s3", rule: "INS-12", tool: "customer_360", kind: "peragent", agent: "cursor", title: "Leave openTickets[].title out of customer_360 for Cursor only",
        why: "Cursor reported openTickets[].title unused in 6 sessions. Claude Code and VS Code use it, so the tool stays the same for them.", save: "≈ 120 B per call for Cursor",
        evidence: [["cursor", "unused", "6 sessions"], ["claude-code", "used", "11 sessions"], ["vscode", "used", "seen in next call input"]], diff: null },
      { id: "s4", rule: "INS-11", tool: null, kind: "blocked", agent: "cursor", title: "Cursor tried to write to billing with read access",
        why: "run_api billing.createCredit was blocked once in session s-3d4e. Cursor has read access to finance.", save: "Decide on access, or ask for a read tool",
        evidence: [["cursor", "blocked", "POST /credits, 10:02"]], diff: null },
    ],
    history: [
      { api: "support", req: "searchIssues", status: 200, ms: 84, when: "10:33", who: "claude-code" },
      { api: "support", req: "getIssue", status: 200, ms: 31, when: "10:33", who: "claude-code" },
      { api: "crm", req: "getAccount", status: 200, ms: 38, when: "09:41", who: "you" },
      { api: "billing", req: "getCustomerSummary", status: 200, ms: 72, when: "09:41", who: "you" },
      { api: "billing", req: "listInvoices", status: 200, ms: 66, when: "Yesterday", who: "you" },
    ],
    calls: CALLS(),
    audit: [
      ["10:35", "claude-code", "Saved tool version", "reassign_ticket v1", "pending: writes to support (AB-3)"],
      ["10:02", "cursor", "Write blocked", "billing.createCredit", "read grant on finance (ACC-2)"],
      ["09:41", "you", "Replaced credential", "billing · Staging · apiKey", "value not recorded"],
      ["Yesterday", "you", "Changed access", "Claude Code → support", "read → read+write, may build"],
      ["Nov 22", "cursor", "Saved tool version", "overdue_invoices v1", "auto-approved: 6 of 6 rules passed"],
      ["Nov 21", "you", "Approved", "open_tickets_for_account v2", "by rules"],
      ["Nov 20", "you", "Connected agent", "Cursor", "finance · read · may build"],
      ["Nov 19", "you", "Confirmed access", "legacy.getCreditHold", "check access → read"],
      ["Nov 19", "you", "Connected agent", "VS Code (Copilot)", "support · read"],
      ["Nov 18", "you", "Connected agent", "Claude Code on this laptop", "support · read"],
    ],
  };
}
function T(id, apis, access, version, by, calls7, err, p50, p95, versions, tests, status = "live") {
  return { id, apis, access, version, by, calls7, err, p50, p95, versions: versions.map((v) => ({ status: "live", ...v })), tests: tests.map(([name, pass]) => ({ name, pass })), status };
}
function CALLS() {
  // [time, agent, session, toolkit, kind, name, status, ms, outBytes, extra]
  const rows = [
    ["10:36:02", "claude-code", "s-91c2", "support", "feedback", "report_field_usage", "ok", 4, 41, { input: { tool: "open_tickets_for_account", version: 2, used: ["issues[].key", "issues[].fields.summary", "issues[].fields.priority"], unused: ["issues[].fields.description"] } }],
    ["10:35:40", "claude-code", "s-91c2", "support", "build", "save_tool", "ok", 212, 310, { input: { file: "reassign_ticket.ts" }, output: { version: 1, status: "pending", reason: "writes to support: a person approves write tools", rules: { tests: "2 of 2 pass", description: "ok", scope: "ok", output: "180 tokens", duplicate: "none" } } }],
    ["10:35:12", "claude-code", "s-91c2", "support", "build", "test_tool", "ok", 96, 520, { input: { file: "reassign_ticket.ts", against: "examples" } }],
    ["10:33:41", "claude-code", "s-91c2", "support", "build", "describe_api", "ok", 3, 690, { input: { request: "support.assignIssue" } }],
    ["10:33:20", "claude-code", "s-91c2", "support", "build", "search_apis", "ok", 5, 280, { input: { query: "assign issue to person" }, output: "support.assignIssue({ key, body }) → void · PUT /rest/api/2/issue/{key}/assignee · write\nsupport.addComment({ key, body }) → Comment · POST /rest/api/2/issue/{key}/comment · write\nsupport.getIssue({ key }) → Issue · GET /rest/api/2/issue/{key} · read" }],
    ["10:33:01", "claude-code", "s-91c2", "support", "build", "search_tools", "ok", 2, 96, { input: { query: "reassign ticket" }, output: "No tool in support matches. Closest: open_tickets_for_account (reads tickets)." }],
    ["10:32:15", "claude-code", "s-91c2", "support", "tool", "open_tickets_for_account", "ok", 101, 0, { tool: "open_tickets_for_account", input: { accountId: "001Ab00000XyZ" } }],
    ["10:31:40", "claude-code", "s-91c2", "support", "tool", "customer_360", "ok", 176, 0, { tool: "customer_360", input: { accountId: "001Ab00000XyZ" } }],
    ["10:31:02", "claude-code", "s-91c2", "support", "list", "tools/list", "ok", 2, 4720, { output: "4 tools" }],
    ["10:18:44", "vscode", "s-77b0", "support", "feedback", "request_tool", "ok", 3, 60, { input: { description: "Refund history for an invoice. invoice_status doesn't include refunds and I can't find a tool that does.", tried: ["invoice_status"] }, output: { request: 7, state: "open" } }],
    ["10:16:09", "vscode", "s-77b0", "support", "tool", "invoice_status", "ok", 61, 0, { tool: "invoice_status", input: { number: "INV-88213" } }],
    ["10:15:30", "vscode", "s-77b0", "support", "tool", "customer_360", "ok", 188, 0, { tool: "customer_360", input: { accountId: "001Ab00000XyZ" } }],
    ["10:15:02", "vscode", "s-77b0", "support", "tool", "customer_360", "error", 41, 210, { input: { accountId: "Northwind" }, error: "crm.getAccount returned 404: no account with id \"Northwind\". accountId is a CRM id like 001Ab00000XyZ. To find one by name, ask for a tool that searches accounts (request_tool)." }],
    ["10:14:40", "vscode", "s-77b0", "support", "list", "tools/list", "ok", 2, 3910, { output: "3 tools (reassign_ticket is a write tool and this agent has read access)" }],
    ["10:04:18", "cursor", "s-3d4e", "finance", "tool", "credit_hold", "ok", 498, 0, { tool: "credit_hold", input: { accountId: "001Ab00000XyZ" } }],
    ["10:02:51", "cursor", "s-3d4e", "finance", "blocked", "run_api billing.createCredit", "blocked", 1, 190, { input: { request: "billing.createCredit", body: { invoice: "INV-88213", amount: 250, reason: "Late delivery" } }, error: "write blocked: this agent has read access to the finance toolkit. Use a read tool, or ask the user to give this agent read+write in mcpDeck." }],
    ["10:01:37", "cursor", "s-3d4e", "finance", "tool", "overdue_invoices", "ok", 351, 0, { tool: "overdue_invoices", input: { accountId: "001Ab00000XyZ" } }],
    ["10:01:05", "cursor", "s-3d4e", "finance", "list", "tools/list", "ok", 2, 4080, { output: "4 tools + 7 build tools" }],
  ];
  return rows.map(([t, agent, session, tk, kind, name, status, ms, bytes, x], i) => ({ id: "c" + (rows.length - i), t, agent, session, tk, kind, name, status, ms, bytes, ...x }));
}

/* Tool implementations mirroring the TypeScript files, so test runs show real outputs and sizes. */
const SIZE = (o) => (typeof o === "string" ? o.length : JSON.stringify(o).length);
const RUN = {
  customer_360: (inp) => {
    if (inp.accountId !== "001Ab00000XyZ") throw { call: ["crm.getAccount", "GET", "/accounts/" + inp.accountId, 404, 32], msg: `crm.getAccount returned 404: no account with id "${inp.accountId}". accountId is a CRM id like 001Ab00000XyZ.` };
    const a = RESP.account, s = RESP.summary, t = RESP.search;
    return { calls: [["crm.getAccount", "GET", "/accounts/{id}", 200, 38, 0, a], ["billing.getCustomerSummary", "GET", "/customers/{id}/summary", 200, 74, 40, s], ["support.searchIssues", "POST", "/rest/api/2/search", 200, 86, 40, t]],
      out: { name: a.name, tier: a.tier, balance: s.balance, overdueInvoices: s.invoices.filter((i) => i.overdue), openTickets: t.issues.map((x) => ({ key: x.key, title: x.fields.summary })) } };
  },
  open_tickets_for_account: (inp) => {
    const a = RESP.account, t = RESP.search;
    return { calls: [["crm.getAccount", "GET", "/accounts/{id}", 200, 36, 0, a], ["support.searchIssues", "POST", "/rest/api/2/search", 200, 61, 38, t]],
      out: { total: t.total, issues: t.issues.slice(0, inp.limit || 20).map((x) => ({ key: x.key, fields: { summary: x.fields.summary, status: x.fields.status.name, priority: x.fields.priority.name, assignee: x.fields.assignee?.displayName ?? null, description: x.fields.description } })) } };
  },
  invoice_status: () => { const i = RESP.invoice; return { calls: [["billing.getInvoice", "GET", "/invoices/{number}", 200, 58, 0, i]], out: { number: i.number, amount: i.total.amount, paid: i.amountPaid.amount, currency: i.total.currency, dueDate: i.dueDate, overdue: true } }; },
  overdue_invoices: () => {
    const a = RESP.account, inv = RESP.invoices, c = RESP.contacts;
    return { calls: [["crm.getAccount", "GET", "/accounts/{id}", 200, 37, 0, a], ["billing.listInvoices", "GET", "/invoices", 200, 151, 39, inv], ["crm.listContacts", "GET", "/accounts/{id}/contacts", 200, 140, 192, c]],
      out: { account: a.name, contact: c.items.find((x) => x.role === "Billing").email, invoices: inv.items.map((i) => ({ number: i.number, amount: i.total.amount, dueDate: i.dueDate })) } };
  },
  credit_hold: () => ({ calls: [["legacy.login", "POST", "/login", 302, 92, 0, RESP.login], ["legacy.getCreditHold", "POST", "/CustomerService.asmx", 200, 371, 95, RESP.hold]], out: { onHold: true, reason: "Invoice INV-88213 overdue more than 21 days", since: "2026-11-21" } }),
  reassign_ticket: (inp) => ({ calls: [["support.getIssue", "GET", "/rest/api/2/issue/{key}", 200, 30, 0, RESP.issue], ["support.assignIssue", "PUT", "/rest/api/2/issue/{key}/assignee", 204, 44, 32, ""], ["support.addComment", "POST", "/rest/api/2/issue/{key}/comment", 201, 52, 78, RESP.created]], out: { key: inp.key || "NWT-482", assignee: inp.assignee || "dana@acme.example", previous: "Sam Ortiz" } }),
};
function inputsOf(source) {
  // Read inputSchema.properties from the tool file (the same literal check-tool.mjs reads).
  const m = /properties:\s*\{([\s\S]*?)\n\s*\},\n\s*required/.exec(source) || /properties:\s*\{(.*)\}\s*,\s*\n?\s*required/.exec(source);
  const req = (/required:\s*\[([^\]]*)\]/.exec(source)?.[1] || "").replace(/["'\s]/g, "").split(",").filter(Boolean);
  const out = [];
  if (!m) return out;
  for (const p of m[1].matchAll(/(\w+):\s*\{\s*type:\s*"(\w+)"(?:[^}]*?examples:\s*\[([^\]]*)\])?/g)) {
    out.push({ name: p[1], type: p[2], example: (p[3] || "").replace(/^["']|["']$/g, "").split(/["'],\s*["']/)[0], required: req.includes(p[1]) });
  }
  return out;
}

/* ---------- state ---------- */
const PETSTORE = [
  ["pet", "addPet", "POST", "/pet"], ["pet", "updatePet", "PUT", "/pet"], ["pet", "findPetsByStatus", "GET", "/pet/findByStatus"], ["pet", "findPetsByTags", "GET", "/pet/findByTags"],
  ["pet", "getPetById", "GET", "/pet/{petId}"], ["pet", "updatePetWithForm", "POST", "/pet/{petId}"], ["pet", "deletePet", "DELETE", "/pet/{petId}"], ["pet", "uploadFile", "POST", "/pet/{petId}/uploadImage"],
  ["store", "getInventory", "GET", "/store/inventory"], ["store", "placeOrder", "POST", "/store/order"], ["store", "getOrderById", "GET", "/store/order/{orderId}"], ["store", "deleteOrder", "DELETE", "/store/order/{orderId}"],
  ["user", "createUser", "POST", "/user"], ["user", "createUsersWithListInput", "POST", "/user/createWithList"], ["user", "loginUser", "GET", "/user/login"], ["user", "logoutUser", "GET", "/user/logout"],
  ["user", "getUserByName", "GET", "/user/{username}"], ["user", "updateUser", "PUT", "/user/{username}"], ["user", "deleteUser", "DELETE", "/user/{username}"],
];
function emptyWorld() { return { apis: [], tools: [], toolkits: [], agents: [], requests: [], suggestions: [], history: [], calls: [], audit: [] }; }
let S = { mode: "demo", theme: "system", ...demoWorld(), ui: { tabs: {}, open: {}, sel: {}, drafts: {}, runs: {}, env: {}, filters: { agent: "", tk: "", kind: "all", session: "" }, usage: { range: "7d", by: "tools" }, showFirstRun: false } };
const api = (id) => S.apis.find((a) => a.id === id);
const tool = (id) => S.tools.find((t) => t.id === id);
const kit = (id) => S.toolkits.find((t) => t.id === id);
const agent = (id) => S.agents.find((a) => a.id === id);
const reqOf = (a, r) => api(a)?.requests.find((x) => x.id === r);
const grantsFor = (tk) => S.agents.flatMap((a) => a.grants.filter((g) => g.tk === tk).map((g) => ({ ...g, agent: a })));
const builders = (tk) => grantsFor(tk).filter((g) => g.build).map((g) => g.agent);
const pendingVersions = () => S.tools.filter((t) => t.status === "pending");
const openRequests = () => S.requests.filter((r) => r.state === "open");
const inboxCount = () => pendingVersions().length + openRequests().length + S.suggestions.length;
const tab = (key, def) => S.ui.tabs[key] || def;
const avatar = (clientId, size = 26) => { const c = CLIENTS[clientId] || CLIENTS.other; return `<span class="avatar" style="background:${c.color};width:${size}px;height:${size}px" title="${esc(c.label)}">${c.short}</span>`; };
const agentName = (id) => (id === "you" ? "You" : agent(id)?.name || CLIENTS[id]?.label || id);
const who = (id) => (id === "you" ? `<span class="avatar" style="background:var(--muted)">You</span>` : avatar(agent(id)?.client || id));
const accessChip = (a) => (a === "read_write" ? `<span class="chip warn">read+write</span>` : a === "write" ? `<span class="chip warn">writes</span>` : a === "check" ? `<span class="chip warn">check access</span>` : `<span class="chip info">read</span>`);
const statusChip = (s) => ({ live: `<span class="chip ok">live</span>`, pending: `<span class="chip warn">needs review</span>`, rejected: `<span class="chip bad">rejected</span>` }[s] || "");
const m = (method) => `<span class="m ${method}">${method}</span>`;
const toolsListTokens = (tkId) => { const k = kit(tkId); return k ? k.tools.filter((t) => tool(t)?.status === "live").reduce((n, t) => n + tok(src(t).length * 0.55 | 0), 0) : 0; };

/* ---------- prompts (agent-first) ---------- */
function prompts(ctx) {
  const out = [];
  const tkWithApi = (apiId) => S.toolkits.find((k) => k.scope.includes(apiId));
  const buildNote = (tkId) => { const b = builders(tkId); return b.length ? `Paste into ${b.map((a) => a.name).join(" or ")}: they can build in ${tkId}.` : `No agent can build in ${tkId} yet. Turn on "May build" for one on the toolkit's Agents tab.`; };
  if (ctx.kind === "home") {
    const k = S.toolkits[0];
    if (k) out.push({ title: "Try a toolkit", text: `Use the mcpDeck ${k.id} tools to tell me everything about account 001Ab00000XyZ: balance, overdue invoices and open tickets.`, note: `Works for any agent connected to ${k.id}.` });
    out.push({ title: "Fulfil tool requests", text: "Call list_tool_requests in mcpDeck and build any requested tool you can. Check search_tools first so you don't duplicate one. Test each with test_tool before save_tool, and pass fulfils with the request id.", note: "For agents that may build." });
    out.push({ title: "Report what you use", text: "For the rest of this session, after each mcpDeck tool call, call report_field_usage with the output fields you used and the ones you didn't.", note: "Feeds field suggestions. Nothing changes until you approve." });
  }
  if (ctx.kind === "api") {
    const a = ctx.api; const k = tkWithApi(a.id);
    const goal = { crm: "finds accounts by name and returns id, name, tier and owner", billing: "lists one customer's overdue invoices with number, amount and due date", support: "lists one account's open tickets with priority Highest or High", legacy: "returns the notes on a customer from the order system" }[a.id];
    const first = a.requests.find((r) => r.access === "read") || a.requests[0];
    const text = goal ? `Build a read-only tool in the mcpDeck ${k ? k.id : "<toolkit>"} toolkit that ${goal}. Use search_apis and describe_api on the ${a.id} API first, return only the fields a person would ask about, test it with test_tool against saved examples, then save_tool.`
      : first ? `Build a read-only tool in the mcpDeck ${k ? k.id : "<toolkit>"} toolkit around ${a.id}.${first.id}. Use describe_api to see its response, return only the fields a person would ask about, test it with test_tool, then save_tool.` : null;
    if (text) out.push({ title: `Build a tool from ${a.id}`, text, note: k ? buildNote(k.id) : `Add ${a.id} to a toolkit's build scope first, so agents can explore it.` });
    if (first) out.push({ title: "Explore the API", text: `Use mcpDeck run_api to call ${a.id}.${first.id} with real values and summarise what the response contains. Don't save anything.`, note: k ? buildNote(k.id) : `Needs a toolkit with ${a.id} in its build scope.` });
  }
  if (ctx.kind === "toolkit") {
    const k = ctx.tk; const t = k.tools.find((x) => tool(x)?.status === "live");
    if (t) out.push({ title: "Use the toolkit", text: `Use the mcpDeck ${k.id} tools to answer: which of account 001Ab00000XyZ's invoices are overdue, and are there open tickets about them?`, note: `Any agent connected to ${k.id}.` });
    out.push({ title: "Fulfil requests", text: `Call list_tool_requests for the mcpDeck ${k.id} toolkit and build the tools people asked for. search_tools first, test_tool before save_tool, and pass fulfils with the request id.`, note: buildNote(k.id) });
    out.push({ title: "Build a new tool", text: `In the mcpDeck ${k.id} toolkit, build a read-only tool that gives one account's billing contact and payment terms. Use search_apis and describe_api, return only those fields, test it, then save it.`, note: buildNote(k.id) });
  }
  if (ctx.kind === "tool") {
    const t = ctx.tool; const ins = inputsOf(src(t.id) || S.ui.drafts[t.id] || "");
    const arg = ins.map((i) => `${i.name} ${i.example || "…"}`).join(", ");
    out.push({ title: "Call it", text: `Call the mcpDeck tool ${t.id}${arg ? " with " + arg : ""}. Then call report_field_usage with the fields you used and didn't use.`, note: "Any agent whose toolkit includes this tool." });
    out.push({ title: "Improve it", text: `Read the mcpDeck tool ${t.id} (search_tools with detail: true), then save a new version that returns only what's needed to answer questions about ${t.apis.join(", ")}. Keep the input schema the same. Run test_tool first.`, note: "Needs an agent that may build in a toolkit with this tool." });
  }
  if (ctx.kind === "request") {
    const r = ctx.req;
    out.push({ title: `Build request #${r.id}`, text: `In the mcpDeck ${r.tk} toolkit, tool request #${r.id} says: "${r.text}" Build a tool for it. Check search_tools first, then use search_apis and describe_api, test with test_tool, and save_tool with fulfils: ${r.id}.`, note: buildNote(r.tk) });
  }
  if (ctx.kind === "suggestion") {
    const s = ctx.s;
    const text = s.kind === "parallel" ? `Save a new version of the mcpDeck tool overdue_invoices that runs billing.listInvoices and crm.listContacts with Promise.all. Keep the output the same. Run test_tool before save_tool.`
      : s.kind === "fields" ? `Save a new version of the mcpDeck tool ${s.tool} that leaves out issues[].fields.description. Keep every other field. Run test_tool before save_tool.`
      : s.kind === "blocked" ? `Your call to billing.createCredit was blocked because you have read access to finance. Use request_tool to ask for a credit tool, describing the invoice, amount and reason it needs.` : null;
    if (text) out.push({ title: "Give it to an agent", text, note: s.kind === "blocked" ? "Paste into Cursor." : "A version saved by an agent still goes through your approval rules." });
  }
  if (ctx.kind === "call") {
    const c = ctx.c;
    out.push({ title: c.status === "ok" ? "Explain this call" : "Fix this call", text: c.status === "ok" ? `In mcpDeck, my agent called ${c.name} with ${JSON.stringify(c.input || {})}. Explain what it returned and whether a smaller output would do.` : `My mcpDeck call to ${c.name} failed: ${c.error} Input was ${JSON.stringify(c.input || {})}. Fix the input and try again, or call request_tool if no tool fits.`, note: "Paste into the agent that made the call." });
  }
  return out;
}

/* ---------- copy, toast ---------- */
function toast(msg) { const t = document.createElement("div"); t.className = "toast"; t.textContent = msg; $("#toasts").append(t); setTimeout(() => t.remove(), 3200); }
function copy(text, msg = "Copied") {
  const done = () => toast(msg);
  const fallback = () => { const ta = document.createElement("textarea"); ta.value = text; ta.style.position = "fixed"; ta.style.opacity = "0"; document.body.append(ta); ta.select(); try { document.execCommand("copy"); done(); } catch { toast("Select the text and copy it"); } ta.remove(); };
  try { navigator.clipboard.writeText(text).then(done, fallback); } catch { fallback(); }
}
const CLIP = new Map(); let clipN = 0;
const clip = (text) => { const k = "k" + ++clipN; CLIP.set(k, text); return k; };
const copyBtn = (text, label = "Copy", msg = "Copied", cls = "light sm") => `<button type="button" class="btn ${cls}" data-a="copy" data-k="${clip(text)}" data-msg="${esc(msg)}">${ic("copy", 14)}${label}</button>`;
const copyField = (text, msg) => `<div class="copyfield"><code>${esc(text)}</code>${copyBtn(text, "Copy", msg || "Copied")}</div>`;
const codeBlock = (text, label, msg) => `<div class="codeblock">${label ? `<div class="lab">${esc(label)}</div>` : ""}<pre>${esc(text)}</pre>${copyBtn(text, "Copy", msg || "Copied")}</div>`;
const promptCard = (p) => `<div class="prompt"><div class="row nw"><b class="grow">${esc(p.title)}</b>${copyBtn(p.text, "Copy prompt", "Prompt copied. Paste it into your agent.", "blue sm")}</div><div class="p">${esc(p.text)}</div>${p.note ? `<div class="small muted">${esc(p.note)}</div>` : ""}</div>`;
const askBtn = (ctx) => `<button type="button" class="btn light" data-a="ask" data-ctx="${esc(ctx)}">${ic("chat")}Ask an agent</button>`;

/* ---------- syntax highlight ---------- */
const KW = new Set("export const let default async function await return if else for of new type import from true false null undefined throw try catch".split(" "));
function hlTs(text) {
  const re = /(\/\/[^\n]*)|(`(?:\\.|[^`\\])*`|"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*')|\b(\d+(?:\.\d+)?)\b|\b([A-Za-z_$][\w$]*)\b(\s*\()?/g;
  let out = "", last = 0, mm;
  while ((mm = re.exec(text))) {
    out += esc(text.slice(last, mm.index));
    if (mm[1]) out += `<span class="tk-com">${esc(mm[1])}</span>`;
    else if (mm[2]) out += `<span class="tk-str">${esc(mm[2])}</span>`;
    else if (mm[3]) out += `<span class="tk-num">${mm[3]}</span>`;
    else if (KW.has(mm[4])) out += `<span class="tk-kw">${mm[4]}</span>` + esc(mm[5] || "");
    else if (mm[5]) out += `<span class="tk-fn">${esc(mm[4])}</span>${esc(mm[5])}`;
    else out += esc(mm[4]);
    last = re.lastIndex;
  }
  return out + esc(text.slice(last));
}
function hlJson(v) {
  const text = typeof v === "string" ? v : JSON.stringify(v, null, 2);
  if (typeof v === "string") return esc(text);
  return esc(text).replace(/(&quot;(?:[^&]|&(?!quot;))*?&quot;)(\s*:)?|\b(true|false|null)\b|(-?\d+(?:\.\d+)?)/g, (s, str, colon, b, n) => str ? (colon ? `<span class="j-k">${str}</span>${colon}` : `<span class="j-s">${str}</span>`) : b ? `<span class="j-b">${b}</span>` : `<span class="j-n">${n}</span>`);
}
function lineDiff(a, b) {
  const A = a.split("\n"), B = b.split("\n"), n = A.length, mm = B.length;
  const L = Array.from({ length: n + 1 }, () => new Int16Array(mm + 1));
  for (let i = n - 1; i >= 0; i--) for (let j = mm - 1; j >= 0; j--) L[i][j] = A[i] === B[j] ? L[i + 1][j + 1] + 1 : Math.max(L[i + 1][j], L[i][j + 1]);
  const out = []; let i = 0, j = 0;
  while (i < n && j < mm) { if (A[i] === B[j]) { out.push(["", A[i]]); i++; j++; } else if (L[i + 1][j] >= L[i][j + 1]) out.push(["del", A[i++]]); else out.push(["add", B[j++]]); }
  while (i < n) out.push(["del", A[i++]]); while (j < mm) out.push(["add", B[j++]]);
  return out;
}
const diffHtml = (rows) => `<div class="diff">${rows.map(([k, l]) => `<div class="${k}">${k === "add" ? "+ " : k === "del" ? "- " : k === "hunk" ? "" : "  "}${esc(l)}</div>`).join("")}</div>`;
function sourceAt(toolId, v) {
  const cur = S.ui.drafts[toolId + "@saved"] || src(toolId);
  if (toolId === "customer_360" && v === 2) return cur.replace(/  const \[summary, tickets\] = await Promise\.all\(\[\n    billing\.getCustomerSummary\(\{ id: account\.billingId \}\),\n    support\.searchIssues\((.*)\),\n  \]\);/, "  const summary = await billing.getCustomerSummary({ id: account.billingId });\n  const tickets = await support.searchIssues($1);");
  if (toolId === "customer_360" && v === 1) return sourceAt(toolId, 2).replace("openTickets: tickets.issues.map((t) => ({ key: t.key, title: t.fields.summary })),", "openTickets: tickets.issues,");
  if (toolId === "open_tickets_for_account" && v === 1) return cur.replace(/        priority: t\.fields\.priority\.name,\n        assignee: t\.fields\.assignee\?\.displayName \?\? null,\n/, "");
  return S.ui.drafts[`${toolId}@v${v}`] || cur;
}

/* ---------- shell ---------- */
const NAV = [
  ["home", "Home", "home"], ["apis", "APIs", "api", "1"], ["toolkits", "Toolkits", "layers", "2"], ["agents", "Agents", "agent", "3"], ["activity", "Activity", "activity", "4"],
];
function renderSide(route) {
  const sect = route.split("-")[0];
  const active = { api: "apis", toolkit: "toolkits", tool: "toolkits", tools: "toolkits", review: "toolkits", agent: "agents", usage: "activity", audit: "activity", preferences: "preferences" }[sect] || sect;
  const n = inboxCount();
  $("#side").innerHTML = `
    <div class="brand">${LOGO}<span>mcpDeck</span></div>
    <button type="button" class="search-btn" data-a="palette">${ic("search", 15)}<span>Search or run</span><kbd>${MOD} K</kbd></button>
    ${NAV.map(([id, label, icon, step]) => `<a class="nav ${active === id ? "on" : ""}" href="#${id}" ${active === id ? 'aria-current="page"' : ""}>${ic(icon, 18)}<span>${label}</span>${id === "home" && n ? `<span class="count" title="${n} need you">${n}</span>` : ""}</a>`).join("")}
    <div class="spacer"></div>
    <a class="nav ${active === "preferences" ? "on" : ""}" href="#preferences">${ic("sliders", 18)}<span>Settings</span></a>
    <div class="status"><span class="dot"></span>127.0.0.1:47800</div>`;
}
const head = (title, sub, actions = "", crumbs = "") => `<header class="pagehead"><div class="titles">${crumbs ? `<div class="crumbs">${crumbs}</div>` : ""}<h1>${title}</h1>${sub ? `<p class="muted">${sub}</p>` : ""}</div><div class="actions">${actions}</div></header>`;
const tabsBar = (key, items, def) => { const cur = tab(key, def); return `<div class="tabs" role="tablist">${items.map(([id, label, badge]) => `<button type="button" role="tab" aria-selected="${cur === id}" class="tab ${cur === id ? "on" : ""}" data-a="tab" data-key="${key}" data-v="${id}">${label}${badge ? ` <span class="chip neutral">${badge}</span>` : ""}</button>`).join("")}</div>`; };
const empty = (icon, title, text, actions) => `<div class="stack" style="align-items:center;text-align:center;padding:48px 16px;gap:12px">${`<span style="color:var(--muted)">${ic(icon, 32)}</span>`}<h2>${title}</h2><p class="muted" style="max-width:460px">${text}</p><div class="row" style="justify-content:center">${actions}</div></div>`;

/* ---------- pages ---------- */
const PAGES = {};

PAGES.home = () => {
  if (S.mode === "new" || !S.apis.length) return firstRun();
  const k = S.toolkits;
  const live = S.agents.filter((a) => a.live).length;
  const callsToday = S.agents.reduce((n, a) => n + a.today, 0);
  const tokensToday = 61400;
  const steps = [
    ["apis", "APIs", S.apis.length, `Systems you call · ${S.apis.reduce((n, a) => n + a.requests.length, 0)} requests`, "api"],
    ["toolkits", "Toolkits", k.length, `MCP servers · ${S.tools.length} tools in the library`, "layers"],
    ["agents", "Agents", S.agents.length, `Connected clients · ${live} active now`, "agent"],
    ["activity", "Activity", callsToday, `Calls today · ${fmt(tokensToday)} tokens returned`, "activity"],
  ];
  const inbox = [
    ...pendingVersions().map((t) => `<div class="item">${who(t.versions[0].by)}<div class="body"><div class="row"><b class="name">${t.id}</b> <span class="muted small">v${t.version}</span> ${accessChip(t.access)} ${statusChip("pending")}</div><div class="sub">Saved by ${esc(agentName(t.versions[0].by))} ${t.versions[0].when}. Tools that write always wait for a person.</div></div><div class="acts"><a class="btn blue sm" href="#review-${t.id}">Review</a></div></div>`),
    ...openRequests().map((r) => `<div class="item">${who(r.agent)}<div class="body"><div class="row"><b>Tool request #${r.id}</b><span class="chip neutral">${r.tk}</span><span class="muted small">${r.when}</span></div><div class="sub">${esc(r.text)}</div></div><div class="acts">${copyBtn(prompts({ kind: "request", req: r })[0].text, "Copy prompt", "Prompt copied. Paste it into an agent that can build.")}<a class="btn light sm" href="#toolkit-${r.tk}" data-a="tab" data-key="toolkit-${r.tk}" data-v="requests">Open</a></div></div>`),
    ...S.suggestions.map((s) => `<div class="item"><span class="avatar" style="background:var(--accent-soft);color:var(--accent)">${ic("bulb", 15)}</span><div class="body"><div class="row"><b>${esc(s.title)}</b></div><div class="sub">${esc(s.save)}</div></div><div class="acts"><button type="button" class="btn light sm" data-a="suggestion" data-id="${s.id}">Details</button></div></div>`),
  ];
  const recent = S.calls.filter((c) => c.kind !== "list").slice(0, 6);
  return head("Home", "What's connected, what needs you, and what agents did today.", `${askBtn("home")}<button type="button" class="btn light" data-a="addApi">${ic("plus")}Add API</button><button type="button" class="btn dark" data-a="connect">${ic("agent")}Connect agent</button>`) + `
  <section class="glass pipe" aria-label="How mcpDeck fits together">${steps.map(([href, label, n, desc, icon]) => `<a class="stepcard" href="#${href}" style="color:inherit;text-decoration:none"><span class="what">${ic(icon)}${label}</span><span class="n">${fmt(n)}</span><span class="desc">${desc}</span></a>`).join("")}</section>
  <div class="grid cols-2">
    <section class="glass panel"><div class="panel-head"><h2>Needs you</h2><span class="chip neutral">${inbox.length}</span><span class="grow"></span><span class="small muted">Reviews, tool requests and suggestions</span></div>
      <div class="stack">${inbox.join("") || `<p class="muted">Nothing needs you. Agents' requests and mcpDeck's suggestions show up here.</p>`}</div></section>
    <section class="glass panel"><div class="panel-head"><h2>Today</h2><span class="grow"></span><a href="#activity" class="small">All activity</a></div>
      <div class="grid" style="grid-template-columns:repeat(2,minmax(0,1fr));margin-bottom:14px">
        <div class="card stat"><div class="label">Calls</div><div class="value">${fmt(callsToday)}</div></div>
        <div class="card stat"><div class="label">Tokens returned</div><div class="value">61.4K</div><div class="hint">Counted by mcpDeck</div></div>
        <div class="card stat"><div class="label">Errors</div><div class="value">${S.agents.reduce((n, a) => n + a.errors, 0)}</div></div>
        <div class="card stat"><div class="label">Writes blocked</div><div class="value">${S.agents.reduce((n, a) => n + a.blocked, 0)}</div></div>
      </div>
      <div class="stack" style="gap:2px">${recent.map((c) => `<a class="row nw" href="#activity" data-a="pickcall" data-id="${c.id}" style="color:inherit;padding:6px 4px;border-radius:8px">${avatar(agent(c.agent)?.client || c.agent, 22)}<span class="name grow" style="font-weight:500;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">${esc(c.name)}</span>${c.status === "ok" ? "" : `<span class="chip ${c.status === "blocked" ? "bad" : "warn"}">${c.status}</span>`}<span class="small muted nowrap">${c.t.slice(0, 5)}</span></a>`).join("")}</div>
    </section>
  </div>`;
};

function firstRun() {
  const hasApi = S.apis.length > 0, hasKit = S.toolkits.length > 0, hasAgent = S.agents.length > 0;
  const newKit = S.toolkits[0], newAgent = S.agents[0];
  const step = (n, done, title, text, action) => `<div class="card ${done ? "done" : ""}"><div class="row nw"><span class="num">${done ? ic("check", 14) : n}</span><h3 class="grow">${title}</h3></div><p class="small muted">${text}</p><div class="row" style="margin-top:auto">${action}</div></div>`;
  const buildPrompt = newKit && S.apis[0] ? prompts({ kind: "api", api: S.apis[0] })[0] : null;
  return `<section class="glass hero">
      <h1>Turn the APIs you already have into tools for your agents.</h1>
      <p>Add an API, put tools in a toolkit, and connect Claude Code, VS Code, Cursor or Claude Desktop. mcpDeck keeps the keys, decides what each agent may do, and records every call.</p>
    </section>
    <section class="firststeps" aria-label="Get started">
      ${step(1, hasApi, "Add an API", hasApi ? `${S.apis.map((a) => `<code>${a.id}</code>`).join(", ")} added with ${plural(S.apis.reduce((n, a) => n + a.requests.length, 0), "request")}.` : "Paste a Swagger page URL, import OpenAPI or a Postman collection, or paste curl. One API per system, like your CRM.", hasApi ? `<a class="btn light sm" href="#api-${S.apis[0].id}">Open ${S.apis[0].id}</a>` : `<button type="button" class="btn dark sm" data-a="addApi">${ic("plus", 14)}Add API</button>`)}
      ${step(2, hasKit, "Create a toolkit", hasKit ? `<code>${newKit.id}</code> is an MCP server at <code>/mcp/${newKit.id}</code>.` : "A toolkit is one MCP server. Choose which APIs its agents may use to build tools.", hasKit ? `<a class="btn light sm" href="#toolkit-${newKit.id}">Open ${newKit.id}</a>` : `<button type="button" class="btn ${hasApi ? "dark" : "light"} sm" data-a="newToolkit" ${hasApi ? "" : "disabled"}>${ic("plus", 14)}New toolkit</button>${hasApi ? "" : `<span class="small muted">Add an API first</span>`}`)}
      ${step(3, hasAgent, "Connect an agent", hasAgent ? `${esc(newAgent.name)} can use <code>${newAgent.grants[0].tk}</code> and may build tools.` : "Add the toolkit to Claude Code, VS Code, Cursor or Claude Desktop. Let it build tools if you want it to.", hasAgent ? `<a class="btn light sm" href="#agent-${newAgent.id}">Open agent</a>` : `<button type="button" class="btn ${hasKit ? "dark" : "light"} sm" data-a="connect" ${hasKit ? "" : "disabled"}>${ic("agent", 14)}Connect agent</button>${hasKit ? "" : `<span class="small muted">Create a toolkit first</span>`}`)}
      ${step(4, S.tools.length > 0, "Build the first tool", S.tools.length ? `${S.tools.length} tool in the library.` : "Ask your agent to build it from the API, or write it yourself in the editor.", hasAgent && buildPrompt ? copyBtn(buildPrompt.text, "Copy prompt for your agent", "Prompt copied. Paste it into your agent.", "dark sm") + `<button type="button" class="btn light sm" data-a="newTool" data-how="blank" data-tk="${newKit.id}">Write it myself</button>` : `<button type="button" class="btn light sm" disabled>Copy prompt for your agent</button><span class="small muted">Connect an agent first</span>`)}
    </section>
    <section class="glass panel"><div class="row"><div class="grow stack" style="gap:4px"><h2>Try it with the demo world</h2><p class="small muted">Four mock APIs on your computer (a CRM, billing, a Jira-like support desk and a SOAP order system), two toolkits, three agents and a morning of calls. Nothing leaves this computer.</p></div><button type="button" class="btn light" data-a="mockstate" data-v="demo">Load the demo</button></div></section>`;
}

PAGES.apis = () => {
  const acts = `<button type="button" class="btn dark" data-a="addApi">${ic("plus")}Add API</button>`;
  if (!S.apis.length) return head("APIs", "Each API is one system you call, like your CRM or Jira.", acts) + `<section class="glass panel">${empty("api", "No APIs yet", "Add one from a Swagger page, an OpenAPI file, a Postman collection or curl. Its requests, environments and credentials live inside it.", acts)}</section>`;
  const usedBy = (id) => S.tools.filter((t) => t.apis.includes(id)).length;
  return head("APIs", "Each API is one system you call, like your CRM or Jira. Its requests, environments and credentials live inside it.", acts) + `
  <section class="glass panel"><div class="tablewrap"><table>
    <thead><tr><th>API</th><th>From</th><th class="num">Requests</th><th>Auth</th><th>Credentials</th><th class="num">Used by tools</th></tr></thead>
    <tbody>${S.apis.map((a) => {
      const reads = a.requests.filter((r) => r.access === "read").length, checks = a.requests.filter((r) => r.access === "check").length;
      const missing = Object.entries(a.creds || {}).filter(([, v]) => !v).map(([e]) => e);
      return `<tr class="click" data-go="#api-${a.id}"><td><div class="name">${a.id}</div><div class="sub">${esc(a.title)}</div></td><td class="small">${esc(a.kind)}</td>
      <td class="num"><b>${a.requests.length}</b><div class="sub">${reads} read · ${a.requests.length - reads - checks} write${checks ? ` · <span style="color:var(--warn)">${checks} to check</span>` : ""}</div></td>
      <td class="small">${esc(a.authType)}</td><td>${a.authType === "None" ? `<span class="muted small">Not needed</span>` : missing.length ? `<span class="chip warn">Missing in ${missing.join(", ")}</span>` : `<span class="chip ok">${ic("lock", 11)}Set</span>`}</td><td class="num">${usedBy(a.id)}</td></tr>`;
    }).join("")}</tbody></table></div></section>`;
};

PAGES.api = (id) => {
  const a = api(id);
  if (!a) return notFound("API", "#apis");
  const env = S.ui.env[id] || a.envs[0];
  const t = tab("api-" + id, "requests");
  const head1 = head(`<span class="mono">${a.id}</span>`, `${esc(a.title)} · ${esc(a.kind)} · tools call it as <code>${a.id}.${a.requests[0]?.id || "request"}()</code>`,
    `<label class="sr" for="env-${id}">Environment</label><select class="input" id="env-${id}" style="width:auto;height:34px;border-radius:999px" data-a="env" data-api="${id}">${a.envs.map((e) => `<option ${e === env ? "selected" : ""}>${e}</option>`).join("")}</select>${askBtn("api:" + id)}<button type="button" class="btn light" data-a="addApi" data-into="${id}">${ic("import")}Import</button><button type="button" class="btn dark" data-a="newRequest" data-api="${id}">${ic("plus")}New request</button>`,
    `<a href="#apis">APIs</a>${ic("chev", 12)}<span>${a.id}</span>`);
  const tabs = tabsBar("api-" + id, [["requests", "Requests", a.requests.length], ["environments", "Environments", a.envs.length], ["credentials", "Auth and credentials"], ["settings", "Settings"]], "requests");
  let body = "";
  if (t === "requests") body = apiWorkspace(a, env);
  if (t === "environments") body = apiEnvs(a);
  if (t === "credentials") body = apiCreds(a);
  if (t === "settings") body = apiSettings(a);
  return head1 + `<section class="glass panel" style="padding-top:8px">${tabs}${body}</section>`;
};

function apiWorkspace(a, env) {
  const side = tab("tree-" + a.id, "requests");
  const open = S.ui.open[a.id] || (S.ui.open[a.id] = a.requests.length ? [a.requests[0].id] : []);
  const cur = S.ui.sel[a.id] && open.includes(S.ui.sel[a.id]) ? S.ui.sel[a.id] : open[0];
  const r = a.requests.find((x) => x.id === cur);
  const folders = [...new Set(a.requests.map((x) => x.folder))];
  const tree = side === "requests"
    ? `<input class="input" type="search" placeholder="Filter requests" aria-label="Filter requests" data-filter=".treeitem" style="height:32px">` + (a.requests.length ? folders.map((f) => `<div class="folder">${ic("folder", 13)}${esc(f)}</div>` + a.requests.filter((x) => x.folder === f).map((x) => `<button type="button" class="treeitem ${x.id === cur ? "on" : ""}" data-a="openReq" data-api="${a.id}" data-id="${x.id}" data-text="${x.id} ${x.path}">${m(x.method)}<span class="lbl">${x.id}</span>${x.access === "check" ? `<span title="Check access">${ic("alert", 13)}</span>` : ""}</button>`).join("")).join("") : `<p class="small muted" style="padding:8px">No requests yet. Import some, or add one with New request.</p>`)
    : historyList(a);
  const editor = r ? requestEditor(a, r, env, open) : `<div class="card">${empty("file", "No request open", "Pick a request on the left, or import some.", `<button type="button" class="btn light" data-a="addApi" data-into="${a.id}">${ic("import")}Import</button>`)}</div>`;
  return `<div class="workspace"><div class="card tree" style="padding:10px"><div class="seg" style="margin-bottom:8px"><button type="button" class="${side === "requests" ? "on" : ""}" data-a="tab" data-key="tree-${a.id}" data-v="requests">Requests</button><button type="button" class="${side === "history" ? "on" : ""}" data-a="tab" data-key="tree-${a.id}" data-v="history">History</button></div>${tree}</div><div class="stack" style="min-width:0;gap:12px">${editor}</div></div>`;
}
function historyList(a) {
  const rows = S.history.filter((h) => h.api === a.id);
  const sel = S.ui.sel["hist-" + a.id] || [];
  if (!rows.length) return `<p class="small muted" style="padding:8px">Every send shows here, yours and agents', newest first.</p>`;
  return `<p class="small muted" style="padding:4px 6px 8px">Select sends, then create a tool from them.</p>` + rows.map((h, i) => { const r = reqOf(h.api, h.req); return `<label class="treeitem" style="cursor:pointer"><input type="checkbox" data-a="histsel" data-api="${a.id}" data-i="${i}" ${sel.includes(i) ? "checked" : ""}>${m(r?.method || "GET")}<span class="lbl">${h.req}</span><span class="small muted">${h.status}</span></label><div class="small muted" style="padding:0 8px 4px 34px">${h.when} · ${esc(agentName(h.who))} · ${h.ms} ms</div>`; }).join("") +
    `<button type="button" class="btn dark sm" style="margin-top:8px" data-a="toolFromHistory" data-api="${a.id}" ${sel.length ? "" : "disabled"}>${ic("code", 14)}Create tool from ${sel.length || ""} selected</button>`;
}
function urlHtml(a, r, env) {
  const base = `<span class="var">{{baseUrl}}</span>`;
  const path = esc(r.path).replace(/\{(\w+)\}/g, (s, p) => `<span class="var">:${p}</span>`);
  const q = (r.query || []).map(([k, v]) => `${k}=${esc(v)}`).join("&");
  return base + path + (q ? "?" + q : "");
}
function requestEditor(a, r, env, open) {
  const sub = tab("req-" + a.id + r.id, r.body ? "body" : "params");
  const run = S.ui.runs["req:" + a.id + r.id];
  const resp = run || (r.example && RESP[r.resp] !== undefined ? { status: 200, ms: null, body: RESP[r.resp], example: true } : null);
  const kvs = (rows) => rows && rows.length ? `<table class="kv">${rows.map(([k, v]) => `<tr><td>${esc(k)}</td><td>${esc(v)}</td></tr>`).join("")}</table>` : `<p class="small muted">None.</p>`;
  const headers = [["Authorization", a.authType.startsWith("Bearer") ? "Bearer {{token}} · added by mcpDeck" : "—"], ...(a.authType.startsWith("API key") ? [["X-Api-Key", "{{apiKey}} · added by mcpDeck"]] : []), ...(r.body ? [["Content-Type", r.bodyType === "XML" ? "text/xml; charset=utf-8" : r.bodyType || "application/json"]] : [])].filter(([, v]) => v !== "—");
  const subBody = {
    params: `${r.params?.length ? `<div class="small muted" style="margin-bottom:6px">Path variables</div>${kvs(r.params)}` : ""}${r.query?.length ? `<div class="small muted" style="margin:10px 0 6px">Query</div>${kvs(r.query)}` : ""}${!r.params?.length && !r.query?.length ? `<p class="small muted">No params.</p>` : ""}`,
    auth: `<div class="row"><span class="chip neutral">Inherit from API</span><span class="small">${esc(a.authType)}${a.login ? ` · runs <code>${a.login.request}</code> when needed, keeps the ${esc(a.login.capture)} for ${a.login.lasts}` : ""}</span></div><p class="small muted" style="margin-top:8px">mcpDeck adds the credential when the request leaves. Agents and tool code never see it.</p>`,
    headers: kvs(headers),
    body: r.body ? `<div class="small muted" style="margin-bottom:6px">${esc(r.bodyType || "JSON")}</div><div class="codeblock"><pre>${esc(r.body)}</pre></div>` : `<p class="small muted">No body.</p>`,
    scripts: a.scripts ? `<div class="notice warn">${ic("lock")}<div>This API came with ${a.scripts.count} Postman scripts. They don't run until you trust them in Settings.</div></div>` : `<p class="small muted">No scripts. Pre-request and post-response scripts use Postman's <code>pm</code> API.</p>`,
  };
  const accessRow = r.access === "check"
    ? `<div class="notice warn">${ic("alert")}<div class="grow">This looks like a SOAP call (${esc(r.soap)}). mcpDeck can't tell if it reads or writes, so tools using it count as writes until you say.</div><div class="row nw"><button type="button" class="btn light sm" data-a="setAccess" data-api="${a.id}" data-id="${r.id}" data-v="read">It reads</button><button type="button" class="btn light sm" data-a="setAccess" data-api="${a.id}" data-id="${r.id}" data-v="write">It writes</button></div></div>`
    : `<div class="row small"><span class="muted">Access</span><div class="seg"><button type="button" class="${r.access === "read" ? "on" : ""}" data-a="setAccess" data-api="${a.id}" data-id="${r.id}" data-v="read">Read</button><button type="button" class="${r.access === "write" ? "on" : ""}" data-a="setAccess" data-api="${a.id}" data-id="${r.id}" data-v="write">Write</button></div><span class="muted">${r.override ? esc(r.override) : r.access === "read" ? "Agents with read access can use it." : "Blocked for agents with read access."}</span></div>`;
  let respHtml;
  if (!resp) respHtml = `<div class="resp"><div class="resp-head"><span class="muted">Response</span></div><p class="muted small" style="padding:22px 14px">Send the request to see the response. <kbd>${MOD} Enter</kbd></p></div>`;
  else {
    const bytes = SIZE(resp.body);
    const rt = tab("resp-" + a.id + r.id, "pretty");
    const content = rt === "headers" ? `<pre>${esc(`content-type: ${typeof resp.body === "string" ? "text/xml; charset=utf-8" : "application/json"}\ncontent-length: ${bytes}\ndate: Wed, 25 Nov 2026 10:${resp.example ? "02" : "41"}:07 GMT`)}</pre>`
      : rt === "raw" ? `<pre>${esc(typeof resp.body === "string" ? resp.body : JSON.stringify(resp.body))}</pre>`
      : rt === "timing" ? `<pre>${esc(resp.ms ? `DNS        0 ms\nConnect    1 ms\nmcpDeck    1 ms   (proxy: grant check, credential, recording)\nWaiting    ${resp.ms - 4} ms\nDownload   2 ms\nTotal      ${resp.ms} ms` : "Saved example: no timing recorded.")}</pre>`
      : `<pre>${hlJson(resp.body)}</pre>`;
    respHtml = `<div class="resp"><div class="resp-head"><span class="chip ${resp.status < 400 ? "ok" : "bad"}">${resp.status} ${resp.status === 200 ? "OK" : resp.status === 201 ? "Created" : resp.status === 302 ? "Found" : ""}</span>${resp.example ? `<span class="chip neutral">Saved example</span>` : `<span>${resp.ms} ms</span>`}<span>${kb(bytes)}</span><span title="Tokens an agent would read">${fmt(tok(bytes))} tokens</span><span class="grow"></span>
      <div class="seg">${["pretty", "raw", "headers", "timing"].map((x) => `<button type="button" class="${rt === x ? "on" : ""}" data-a="tab" data-key="resp-${a.id}${r.id}" data-v="${x}">${x[0].toUpperCase() + x.slice(1)}</button>`).join("")}</div></div>${content}
      <div class="resp-head" style="border-top:1px solid var(--line);border-bottom:0">${resp.example ? "" : `<button type="button" class="btn light sm" data-a="saveExample" data-api="${a.id}" data-id="${r.id}">Save as example <kbd>${MOD} ⇧ S</kbd></button>`}<button type="button" class="btn light sm" data-a="toolFromReq" data-api="${a.id}" data-id="${r.id}">${ic("code", 14)}Create tool</button>${copyBtn(`${a.id}.${r.id}(${r.params?.length ? `{ ${r.params.map(([k]) => k).join(", ")} }` : ""})`, "Copy as tool call", "Copied the call for the tool editor")}${copyBtn(curlOf(a, r), "Copy as curl", "Copied as curl. The credential is left out.")}</div></div>`;
  }
  return `<div class="card" style="padding:0;overflow:hidden">
    <div class="row nw" style="gap:0;border-bottom:1px solid var(--line);overflow-x:auto">${open.map((id) => { const x = a.requests.find((q) => q.id === id); return x ? `<div class="row nw" style="gap:4px;padding:8px 6px 8px 12px;border-right:1px solid var(--line);${id === r.id ? "background:var(--glass-strong)" : ""}"><button type="button" class="treeitem" style="padding:0;width:auto" data-a="openReq" data-api="${a.id}" data-id="${id}">${m(x.method)}<span class="lbl">${id}</span></button><button type="button" class="iconbtn" style="width:22px;height:22px" aria-label="Close ${id}" data-a="closeReq" data-api="${a.id}" data-id="${id}">${ic("x", 12)}</button></div>` : ""; }).join("")}</div>
    <div class="stack" style="padding:14px;gap:12px">
      <div class="urlbar">${m(r.method)}<div class="url">${urlHtml(a, r, S.ui.env[a.id])}</div><button type="button" class="btn blue" data-a="send" data-api="${a.id}" data-id="${r.id}">${ic("play", 13)}Send</button></div>
      ${accessRow}
      <div><div class="seg" style="margin-bottom:10px">${[["params", "Params"], ["auth", "Auth"], ["headers", "Headers"], ["body", "Body"], ["scripts", "Scripts"]].map(([x, l]) => `<button type="button" class="${sub === x ? "on" : ""}" data-a="tab" data-key="req-${a.id}${r.id}" data-v="${x}">${l}</button>`).join("")}</div>${subBody[sub]}</div>
    </div></div>${respHtml}`;
}
function curlOf(a, r) {
  const q = (r.query || []).map(([k, v]) => `${k}=${encodeURIComponent(v)}`).join("&");
  const path = r.path.replace(/\{(\w+)\}/g, (s, p) => (r.params || []).find(([k]) => k === p)?.[1] || s);
  return `curl -X ${r.method} '{{baseUrl}}${path}${q ? "?" + q : ""}'` + (r.body ? ` \\\n  -H 'Content-Type: ${r.bodyType === "XML" ? "text/xml" : "application/json"}' \\\n  --data '${r.body.replace(/\n\s*/g, " ")}'` : "");
}
function apiEnvs(a) {
  const active = S.ui.env[a.id] || a.envs[0];
  return `<p class="small muted" style="margin-bottom:12px">Variables work in URLs, params, headers, bodies and auth as <code>{{name}}</code>, like Postman. Secret values are only filled in when a request leaves mcpDeck.</p>
  <div class="tablewrap"><table><thead><tr><th>Variable</th>${a.envs.map((e) => `<th>${e}${e === active ? ` <span class="chip info">active</span>` : ""}</th>`).join("")}<th>Kind</th></tr></thead><tbody>
  ${a.vars.map(([k, v1, v2, kind]) => `<tr><td class="name">${k}</td>${a.envs.map((e, i) => { const v = i === 0 ? v1 : v2; return `<td>${kind === "secret" ? `<span class="row nw"><span class="mono muted">••••••••</span><button type="button" class="btn ghost sm" data-a="replaceSecret" data-api="${a.id}" data-k="${k}" data-env="${e}">Replace</button></span>` : v ? `<code>${esc(v)}</code>` : `<span class="muted small">not set</span>`}</td>`; }).join("")}<td>${kind === "secret" ? `<span class="chip neutral">${ic("lock", 11)}secret</span>` : `<span class="chip neutral">text</span>`}</td></tr>`).join("")}
  </tbody></table></div><div class="row" style="margin-top:12px"><button type="button" class="btn light sm" data-a="addVar" data-api="${a.id}">${ic("plus", 14)}Add variable</button><span class="small muted">Values can also come from an environment variable or a <code>.env</code> file you pick.</span></div>`;
}
function apiCreds(a) {
  return `<div class="grid cols-2"><div class="stack">
    <div class="setgroup"><div class="setrow"><div class="l"><b>Auth type</b><span class="small muted">Set here once. Folders and requests inherit it, like collection auth in Postman.</span></div>
      <select class="input" style="width:auto" data-a="authType" data-api="${a.id}" aria-label="Auth type">${["None", "API key (header X-Api-Key)", "Bearer token", "Basic", "OAuth 2.0 client credentials", "Digest", "Login request (cookie)"].map((x) => `<option ${x === a.authType ? "selected" : ""}>${x}</option>`).join("")}</select></div>
      ${a.login ? `<div class="setrow"><div class="l"><b>Login request</b><span class="small muted">For systems that log in with a form. mcpDeck runs it when needed and adds what it captures to later calls.</span></div><div class="stack" style="gap:4px;font-size:13px"><span>Request <code>${a.login.request}</code></span><span>Capture <code>${esc(a.login.capture)}</code></span><span>Lasts ${a.login.lasts}</span></div></div>` : ""}
    </div>
    <h3 style="margin-top:6px">Credentials</h3>
    <div class="setgroup">${a.envs.map((e) => { const set = a.creds?.[e]; return `<div class="setrow"><div class="l"><b>${e}</b><span class="small muted">${set ? `Saved ${set}. Encrypted; it can be replaced, not shown.` : "Not set. Calls in this environment will fail with a message saying so."}</span></div>${set ? `<span class="mono muted">••••••••••••</span><button type="button" class="btn light sm" data-a="replaceSecret" data-api="${a.id}" data-env="${e}">Replace</button>` : `<button type="button" class="btn dark sm" data-a="replaceSecret" data-api="${a.id}" data-env="${e}">Add credential</button>`}</div>`; }).join("")}</div>
  </div><div class="stack">
    <div class="notice info">${ic("shield")}<div><b>Shared credential, separate agents.</b> Every agent uses this ${a.id} credential, so ${esc(a.title)} sees all of them as one user. In mcpDeck each agent still has its own access and its own calls.</div></div>
    <div class="notice info">${ic("key")}<div>Secrets are encrypted with AES-256-GCM. The key is in your ${isMac ? "macOS Keychain" : "Windows Credential Manager"}, not in the database. <a href="#preferences" data-a="tab" data-key="prefs" data-v="security">What this protects</a></div></div>
  </div></div>`;
}
function apiSettings(a) {
  const users = S.tools.filter((t) => t.apis.includes(a.id));
  return `<div class="stack" style="max-width:760px">
    <div class="setgroup">
      <div class="setrow"><div class="l"><b>Allowed hosts</b><span class="small muted">Calls to any other host are refused and recorded.</span></div><div class="row">${a.hosts.map((h) => `<span class="chip neutral mono">${esc(h)}</span>`).join("")}<button type="button" class="btn ghost sm" data-a="addHost" data-api="${a.id}">${ic("plus", 13)}Add</button></div></div>
      <div class="setrow"><div class="l"><b>Run scripts from this collection</b><span class="small muted">${a.scripts ? `${a.scripts.count} imported Postman scripts. Off until you trust them, like Bruno's safe mode.` : "This API has no scripts."}</span></div><input type="checkbox" class="toggle" aria-label="Run scripts" data-a="trust" data-api="${a.id}" ${a.scripts?.trusted ? "checked" : ""} ${a.scripts ? "" : "disabled"}></div>
      <div class="setrow"><div class="l"><b>Name in tool code</b><span class="small muted">Tools call this API as <code>${a.id}.request()</code>. Renaming updates no tools, so it's only allowed while no tool uses it.</span></div><code>${a.id}</code></div>
    </div>
    <div class="setgroup"><div class="setrow"><div class="l"><b>Delete this API</b><span class="small muted">${users.length ? `Used by ${users.map((t) => `<code>${t.id}</code>`).join(", ")}. Remove it from those tools first.` : "Deletes its requests, environments, examples and credentials."}</span></div><button type="button" class="btn danger sm" ${users.length ? "disabled" : ""} data-a="deleteApi" data-api="${a.id}">Delete API</button></div></div>
  </div>`;
}

PAGES.toolkits = () => {
  const t = tab("toolkits", "kits");
  const acts = t === "kits" ? `<button type="button" class="btn dark" data-a="newToolkit" ${S.apis.length ? "" : "disabled"}>${ic("plus")}New toolkit</button>` : `<button type="button" class="btn dark" data-a="newTool" ${S.toolkits.length ? "" : "disabled"}>${ic("plus")}New tool</button>`;
  const seg = `<div class="seg" role="tablist"><button type="button" class="${t === "kits" ? "on" : ""}" data-a="tab" data-key="toolkits" data-v="kits">Toolkits</button><button type="button" class="${t === "tools" ? "on" : ""}" data-a="tab" data-key="toolkits" data-v="tools">All tools</button></div>`;
  const h = head("Toolkits", "A toolkit is a set of tools that an agent adds as one MCP server. Tools live in one library and can be in several toolkits.", acts);
  if (t === "tools") return h + `<section class="glass panel"><div class="panel-head">${seg}</div>${toolTable(S.tools, null)}</section>`;
  if (!S.toolkits.length) return h + `<section class="glass panel"><div class="panel-head">${seg}</div>${empty("layers", "No toolkits yet", S.apis.length ? "Create one, choose the APIs its agents may use, then connect an agent." : "Add an API first. A toolkit's tools call your APIs.", S.apis.length ? acts : `<button type="button" class="btn dark" data-a="addApi">${ic("plus")}Add API</button>`)}</section>`;
  return h + `<section class="glass panel"><div class="panel-head">${seg}</div><div class="grid" style="grid-template-columns:repeat(auto-fill,minmax(340px,1fr))">${S.toolkits.map((k) => {
    const gs = grantsFor(k.id);
    return `<div class="card stack" style="gap:12px"><div class="row nw"><a class="name grow" style="font-size:16px;color:inherit" href="#toolkit-${k.id}">${k.id}</a><div class="row nw" style="gap:2px">${gs.map((g) => avatar(g.agent.client, 22)).join("")}</div></div>
      <p class="small muted">${esc(k.desc)}</p>${copyField(`${HOST}/mcp/${k.id}`, "Endpoint copied")}
      <div class="row small muted"><span>${plural(k.tools.length, "tool")}</span>·<span>${plural(gs.length, "agent")}</span>·<span>tools/list ${fmt(toolsListTokens(k.id))} tokens</span></div>
      <div class="row"><a class="btn light sm" href="#toolkit-${k.id}">Open</a><button type="button" class="btn dark sm" data-a="connect" data-tk="${k.id}">${ic("agent", 14)}Connect agent</button></div></div>`;
  }).join("")}</div></section>`;
};
function toolTable(list, tkId) {
  if (!list.length) return empty("code", "No tools yet", "Ask an agent with build access to make one, create one from requests you've sent, or write it in the editor.", `<button type="button" class="btn dark" data-a="newTool" ${tkId ? `data-tk="${tkId}"` : ""}>${ic("plus")}New tool</button>`);
  return `<div class="tablewrap"><table><thead><tr><th>Tool</th><th>APIs</th><th>Access</th><th>Version</th>${tkId ? "" : "<th>In toolkits</th>"}<th class="num">Calls 7d</th><th class="num">p50</th><th class="num">Avg output</th><th>Made by</th></tr></thead><tbody>
  ${list.map((t) => { const run = RUN[t.id] ? safeRun(t.id) : null; const outTok = run ? tok(SIZE(run.out)) : null; return `<tr class="click" data-go="#${t.status === "pending" ? "review" : "tool"}-${t.id}"><td><div class="name">${t.id}</div><div class="sub" style="max-width:380px">${esc(descOf(t.id))}</div></td>
    <td><div class="row" style="gap:4px">${t.apis.map((x) => `<span class="chip neutral mono">${x}</span>`).join("")}</div></td><td>${accessChip(t.access)}</td><td class="nowrap">v${t.version} ${statusChip(t.status)}</td>
    ${tkId ? "" : `<td class="small">${S.toolkits.filter((k) => k.tools.includes(t.id)).map((k) => k.id).join(", ") || `<span class="muted">none</span>`}</td>`}
    <td class="num">${t.calls7 ? fmt(t.calls7) : "–"}</td><td class="num">${t.p50 ? t.p50 + " ms" : "–"}</td><td class="num">${outTok && t.calls7 ? `${fmt(outTok)} tok` : "–"}</td><td>${t.by === "you" ? `<span class="small">You</span>` : `<span class="row nw" style="gap:6px">${avatar(agent(t.by)?.client || t.by, 20)}<span class="small">${esc(agentName(t.by))}</span></span>`}</td></tr>`; }).join("")}
  </tbody></table></div>`;
}
const descOf = (id) => /description:\s*"([^"]*)"/.exec(S.ui.drafts[id + "@saved"] || src(id) || S.ui.drafts[id] || "")?.[1] || "";
function safeRun(id, inp) { const s = S.ui.drafts[id + "@saved"] || src(id); const ins = inputsOf(s); const x = inp || Object.fromEntries(ins.map((i) => [i.name, i.example])); try { return RUN[id](x); } catch { return null; } }

PAGES.toolkit = (id) => {
  const k = kit(id);
  if (!k) return notFound("toolkit", "#toolkits");
  const t = tab("toolkit-" + id, "tools");
  const gs = grantsFor(id), reqs = S.requests.filter((r) => r.tk === id && r.state === "open"), pend = k.tools.map(tool).filter((x) => x?.status === "pending");
  const h = head(`<span class="mono">${k.id}</span>`, esc(k.desc), `${askBtn("toolkit:" + id)}<button type="button" class="btn light" data-a="newTool" data-tk="${id}">${ic("plus")}New tool</button><button type="button" class="btn dark" data-a="connect" data-tk="${id}">${ic("agent")}Connect agent</button>`, `<a href="#toolkits">Toolkits</a>${ic("chev", 12)}<span>${k.id}</span>`);
  const endpoint = `<div class="grid" style="grid-template-columns:minmax(0,1.4fr) minmax(0,1fr)"><div class="stack" style="gap:6px"><span class="small muted">MCP endpoint (Streamable HTTP)</span>${copyField(`${HOST}/mcp/${k.id}`, "Endpoint copied")}</div><div class="stack" style="gap:6px"><span class="small muted">For clients that only run commands (stdio)</span>${copyField(`mcpdeck stdio --toolkit ${k.id}`, "Command copied")}</div></div>`;
  const tabs = tabsBar("toolkit-" + id, [["tools", "Tools", k.tools.length], ["agents", "Agents", gs.length], ["requests", "Requests and reviews", reqs.length + pend.length || ""], ["settings", "Settings"]], "tools");
  let body = "";
  if (t === "tools") body = toolTable(k.tools.map(tool).filter(Boolean), id) + `<div class="row" style="margin-top:14px"><button type="button" class="btn light sm" data-a="addFromLibrary" data-tk="${id}">${ic("plus", 14)}Add from library</button><span class="small muted grow">Agents load the list of tools once per session: ${fmt(toolsListTokens(id))} tokens for this toolkit.</span></div>`;
  if (t === "agents") body = grantTable(gs.map((g) => ({ ...g, key: g.agent.id })), "toolkit", id);
  if (t === "requests") body = `<div class="stack">${pend.map((x) => `<div class="item">${who(x.versions[0].by)}<div class="body"><div class="row"><b class="name">${x.id}</b> v${x.version} ${accessChip(x.access)} ${statusChip("pending")}</div><div class="sub">Saved by ${esc(agentName(x.versions[0].by))} ${x.versions[0].when}. All other rules passed; tools that write wait for a person.</div></div><div class="acts"><a class="btn blue sm" href="#review-${x.id}">Review</a></div></div>`).join("")}
    ${reqs.map((r) => `<div class="item">${who(r.agent)}<div class="body"><div class="row"><b>Tool request #${r.id}</b><span class="muted small">from ${esc(agentName(r.agent))} · ${r.when}</span></div><div>${esc(r.text)}</div><div class="sub">Tried: ${r.tried.map(esc).join(", ")}</div><div class="sub" style="margin-top:4px">${esc(prompts({ kind: "request", req: r })[0].note)}</div></div><div class="acts">${copyBtn(prompts({ kind: "request", req: r })[0].text, "Copy prompt", "Prompt copied. Paste it into an agent that can build.", "blue sm")}<button type="button" class="btn light sm" data-a="newTool" data-how="requests" data-tk="${id}" data-req="${r.id}">Write it myself</button><button type="button" class="btn ghost sm" data-a="closeRequest" data-id="${r.id}">Close</button></div></div>`).join("")}
    ${!reqs.length && !pend.length ? `<p class="muted">No open requests. When an agent can't find a tool it calls <code>request_tool</code>, and the request shows here.</p>` : ""}</div>`;
  if (t === "settings") body = toolkitSettings(k);
  return h + `<section class="glass panel">${endpoint}</section><section class="glass panel" style="padding-top:8px">${tabs}${body}</section>`;
};
function grantTable(rows, from, ctxId) {
  if (!rows.length) return empty("agent", from === "toolkit" ? "No agent uses this toolkit yet" : "No toolkits granted", from === "toolkit" ? "Connect Claude Code, VS Code, Cursor or Claude Desktop. It gets read access unless you choose otherwise." : "Grant a toolkit to give this agent tools.", `<button type="button" class="btn dark" data-a="connect" ${from === "toolkit" ? `data-tk="${ctxId}"` : `data-agent="${ctxId}"`}>${ic("agent")}${from === "toolkit" ? "Connect agent" : "Grant a toolkit"}</button>`);
  return `<div class="tablewrap"><table><thead><tr><th>${from === "toolkit" ? "Agent" : "Toolkit"}</th><th>Access</th><th>May build tools</th><th>Environment</th><th>Versions</th><th></th></tr></thead><tbody>
  ${rows.map((g) => { const aId = g.agent.id, tk = g.tk; return `<tr><td>${from === "toolkit" ? `<a class="row nw" style="color:inherit" href="#agent-${aId}">${avatar(g.agent.client)}<span><b>${esc(g.agent.name)}</b><div class="sub">${g.agent.live ? "Connected now" : "Last seen " + (g.agent.lastSeen || "never")}</div></span></a>` : `<a class="name" href="#toolkit-${tk}">${tk}</a>`}</td>
    <td><div class="seg"><button type="button" class="${g.access === "read" ? "on" : ""}" data-a="grant" data-agent="${aId}" data-tk="${tk}" data-f="access" data-v="read">Read</button><button type="button" class="${g.access === "read_write" ? "on" : ""}" data-a="grant" data-agent="${aId}" data-tk="${tk}" data-f="access" data-v="read_write">Read and write</button></div></td>
    <td><input type="checkbox" class="toggle" aria-label="May build tools" data-a="grant" data-agent="${aId}" data-tk="${tk}" data-f="build" ${g.build ? "checked" : ""}></td>
    <td class="small">${g.env}</td><td class="small">${g.pin === "latest" ? "Latest" : g.pin}</td>
    <td class="num"><button type="button" class="btn ghost sm" data-a="removeGrant" data-agent="${aId}" data-tk="${tk}">Remove</button></td></tr>`; }).join("")}
  </tbody></table></div><p class="small muted" style="margin-top:10px">Read access: write requests are refused by mcpDeck and write tools aren't listed. Building: the agent also gets search_apis, run_api, save_tool and the other build tools, limited to this toolkit's APIs.</p>`;
}
function toolkitSettings(k) {
  const rule = (key, label, text, locked) => `<label class="setrow" style="cursor:${locked ? "default" : "pointer"}"><span class="l"><b>${label}</b><span class="small muted">${text}</span></span>${locked ? `<span class="chip neutral">${ic("lock", 11)}Always on</span>` : `<input type="checkbox" class="toggle" data-a="rule" data-tk="${k.id}" data-k="${key}" ${k.rules[key] ? "checked" : ""}>`}</label>`;
  return `<div class="grid cols-2"><div class="stack">
    <h3>APIs agents may build with</h3><p class="small muted">Building agents can only search and call requests in these APIs.</p>
    <div class="setgroup">${S.apis.map((a) => `<label class="setrow" style="cursor:pointer"><span class="l"><b class="mono">${a.id}</b><span class="small muted">${esc(a.title)} · ${a.requests.length} requests</span></span><input type="checkbox" class="toggle" data-a="scope" data-tk="${k.id}" data-api="${a.id}" ${k.scope.includes(a.id) ? "checked" : ""}></label>`).join("") || `<p class="small muted" style="padding:12px 0">No APIs yet.</p>`}</div>
    <h3 style="margin-top:8px">Variables</h3><p class="small muted">Tools read these as <code>vars.name</code>. Agents never see or set them.</p>
    <div class="setgroup">${k.vars.map(([n, v]) => `<div class="setrow"><span class="l"><b class="mono">${n}</b></span><code>${esc(v)}</code></div>`).join("") || `<p class="small muted" style="padding:12px 0">None.</p>`}</div>
  </div><div class="stack">
    <h3>Approval rules for tools agents save</h3><p class="small muted">A version saved by an agent goes live only if every rule passes. Otherwise it waits for you on Home.</p>
    <div class="setgroup">
      ${rule("read", "Only reads", "Tools that write always wait for a person.", true)}
      ${rule("scope", "Uses only the APIs above", "Checked against the build list.", true)}
      ${rule("tests", "Has tests, and they pass", "At least one test case, run against saved examples.")}
      ${rule("desc", "Description check", "Length, and no text aimed at other agents (\"ignore\", \"always call\", links).")}
      ${rule("output", `Output under ${fmt(k.limit)} tokens`, "Measured on its tests.")}
      ${rule("dup", "Not a duplicate", "No other tool in the library uses the same requests.")}
    </div>
    <h3 style="margin-top:8px">Responses</h3>
    <div class="setgroup"><label class="setrow" style="cursor:pointer"><span class="l"><b>Hint on large responses</b><span class="small muted">Adds one line under 30 tokens to responses over ${fmt(k.limit)} tokens, asking the agent to report which fields it used.</span></span><input type="checkbox" class="toggle" data-a="hints" data-tk="${k.id}" ${k.hints ? "checked" : ""}></label></div>
  </div></div>`;
}

PAGES.tool = (id) => {
  const t = tool(id);
  if (!t) return notFound("tool", "#toolkits");
  if (t.status === "pending") return PAGES.review(id);
  const k = S.toolkits.find((x) => x.tools.includes(id));
  const tb = tab("tool-" + id, "code");
  const h = head(`<span class="mono">${t.id}</span>`, `${accessChip(t.access)} <span class="chip neutral">v${t.version}</span> ${statusChip(t.status)} <span class="small muted">Uses ${t.apis.map((x) => `<code>${x}</code>`).join(", ")} · in ${S.toolkits.filter((x) => x.tools.includes(id)).map((x) => `<a href="#toolkit-${x.id}">${x.id}</a>`).join(", ") || "no toolkit"}</span>`,
    `${askBtn("tool:" + id)}<button type="button" class="btn light" data-a="runTool" data-id="${id}">${ic("play", 13)}Test <kbd>${MOD} Enter</kbd></button><button type="button" class="btn dark" data-a="saveTool" data-id="${id}">Save <kbd style="color:inherit;border-color:currentColor">${MOD} S</kbd></button>`,
    k ? `<a href="#toolkits">Toolkits</a>${ic("chev", 12)}<a href="#toolkit-${k.id}">${k.id}</a>${ic("chev", 12)}<span>${t.id}</span>` : `<a href="#toolkits" data-a="tab" data-key="toolkits" data-v="tools">All tools</a>${ic("chev", 12)}<span>${t.id}</span>`);
  const tabs = tabsBar("tool-" + id, [["code", "Code"], ["tests", "Tests", t.tests.length], ["versions", "Versions", t.versions.length], ["fields", "Output fields"]], "code");
  let body = "";
  if (tb === "code") body = toolCode(t);
  if (tb === "tests") body = `<div class="stack" style="max-width:820px">${t.tests.map((x, i) => `<div class="item"><span class="rule"><span class="ic ${x.pass === null ? "wait" : x.pass ? "ok" : "bad"}">${ic(x.pass === false ? "x" : "check", 12)}</span></span><div class="body"><b>${esc(x.name)}</b><span class="sub">Runs against saved examples, no network. ${x.pass === null ? "Running…" : x.pass ? "Passed" : "Failed"}</span></div></div>`).join("")}<div class="row"><button type="button" class="btn dark sm" data-a="runTests" data-id="${id}">${ic("play", 12)}Run all</button><span class="small muted">Add a test from a test run or from a recorded call in Activity ("Save as test").</span></div></div>`;
  if (tb === "versions") body = versionsView(t);
  if (tb === "fields") body = fieldsView(t);
  return h + `<section class="glass panel" style="padding-top:8px">${tabs}${body}</section>`;
};
function toolCode(t) {
  const code = S.ui.drafts[t.id] ?? (S.ui.drafts[t.id + "@saved"] || src(t.id));
  const apis = S.apis.filter((a) => t.apis.includes(a.id) || kitScopeFor(t.id).includes(a.id));
  const run = S.ui.runs["tool:" + t.id];
  const ins = inputsOf(code);
  return `<div class="editor-layout">
    <div class="card apilist apilist-panel" style="padding:8px"><div class="small muted" style="padding:4px 6px">Click to insert a call</div>${apis.map((a) => `<div class="client">${ic("api", 13)}${a.id}</div>${a.requests.map((r) => `<button type="button" data-a="insertCall" data-api="${a.id}" data-id="${r.id}" title="${r.method} ${r.path}">${m(r.method)}<span style="overflow:hidden;text-overflow:ellipsis">${r.id}</span></button>`).join("")}`).join("")}</div>
    <div class="stack" style="gap:0;min-width:0"><div class="editor" id="editor"><div class="gutter" id="gutter"></div><pre class="hl" id="hl" aria-hidden="true"></pre><textarea id="code" wrap="off" spellcheck="false" aria-label="Tool code" data-tool="${t.id}">${esc(code)}</textarea></div><div class="problems" id="problems"></div></div>
    <div class="card stack" style="gap:12px"><div class="row"><h3 class="grow">Test</h3><div class="seg"><button type="button" class="${tab("against", "examples") === "examples" ? "on" : ""}" data-a="tab" data-key="against" data-v="examples">Saved examples</button><button type="button" class="${tab("against", "examples") === "live" ? "on" : ""}" data-a="tab" data-key="against" data-v="live">Demo env</button></div></div>
      ${ins.map((i) => `<label class="field"><span>${i.name} <span class="muted small">${i.type}${i.required ? " · required" : ""}</span></span><input class="input mono" id="in-${i.name}" value="${esc(S.ui.runs["in:" + t.id + i.name] ?? i.example ?? "")}" placeholder="${esc(i.example || "")}"></label>`).join("") || `<p class="small muted">No inputs.</p>`}
      <button type="button" class="btn dark" data-a="runTool" data-id="${t.id}">${ic("play", 13)}Run</button>
      ${run ? runResult(t, run) : `<p class="small muted">Run shows the output, each API call it made, and how many tokens the agent would read.</p>`}
    </div></div>`;
}
const kitScopeFor = (toolId) => S.toolkits.filter((k) => k.tools.includes(toolId)).flatMap((k) => k.scope);
function runResult(t, run) {
  if (run.error) return `<div class="notice bad">${ic("alert")}<div><b>Error the agent would get</b><br>${esc(run.error)}</div></div>${copyBtn(run.error, "Copy error", "Copied")}`;
  const up = run.calls.reduce((n, c) => n + SIZE(c[6]), 0), out = SIZE(run.out);
  const total = Math.max(...run.calls.map((c) => c[5] + c[4]));
  return `<div class="small muted">${tab("against", "examples") === "examples" ? "Ran against saved examples. No network." : "Ran against the Demo environment, through the proxy."}</div><div class="sizes"><span>API responses (${run.calls.length})</span><span>${kb(up)} · ${fmt(tok(up))} tokens</span><span class="big">Output to the agent</span><span class="big">${kb(out)} · ${fmt(tok(out))} tokens</span><span class="muted">Saved</span><span class="muted">${Math.max(0, Math.round((1 - out / up) * 100))}% smaller than the raw responses</span></div>
    <div class="small muted">Calls · ${total} ms</div><div class="timeline">${run.calls.map((c) => `<div class="tl"><span class="mono" style="overflow:hidden;text-overflow:ellipsis;white-space:nowrap">${c[0]}</span><span class="bar"><i class="${c[1] === "GET" ? "read" : ""}" style="left:${(c[5] / total) * 100}%;width:${Math.max(3, (c[4] / total) * 100)}%"></i></span><span class="small muted" style="text-align:right">${c[4]} ms</span></div>`).join("")}</div>
    <div class="small muted">Output</div><div class="resp"><pre style="max-height:260px">${hlJson(run.out)}</pre></div>
    <div class="row"><button type="button" class="btn light sm" data-a="saveTest" data-id="${t.id}">Save as test</button>${copyBtn(JSON.stringify(run.out, null, 2), "Copy output", "Output copied")}</div>`;
}
function versionsView(t) {
  const a = S.ui.sel["va-" + t.id] ?? (t.versions[1]?.v || t.versions[0].v), b = S.ui.sel["vb-" + t.id] ?? t.versions[0].v;
  return `<div class="grid" style="grid-template-columns:minmax(0,340px) minmax(0,1fr)"><div class="stack">${t.versions.map((v) => `<div class="item"><div class="body"><div class="row"><b>v${v.v}</b>${v.v === t.version ? statusChip("live") : ""}<span class="small muted">${v.when}</span></div><div class="sub">${esc(v.note)}</div><div class="row small muted" style="gap:6px">${v.by === "you" ? "You" : avatar(agent(v.by)?.client || v.by, 18) + esc(agentName(v.by))}${v.rules ? " · " + esc(v.rules) : ""}</div></div><div class="acts" style="flex-direction:column">${v.v !== t.version ? `<button type="button" class="btn light sm" data-a="rollback" data-id="${t.id}" data-v="${v.v}">Roll back</button>` : ""}</div></div>`).join("")}</div>
  <div class="stack" style="min-width:0"><div class="row"><span class="small muted">Compare</span><select class="input" style="width:auto" data-a="vsel" data-k="va-${t.id}" aria-label="From version">${t.versions.map((v) => `<option value="${v.v}" ${v.v == a ? "selected" : ""}>v${v.v}</option>`).join("")}</select><span>→</span><select class="input" style="width:auto" data-a="vsel" data-k="vb-${t.id}" aria-label="To version">${t.versions.map((v) => `<option value="${v.v}" ${v.v == b ? "selected" : ""}>v${v.v}</option>`).join("")}</select></div>
  ${diffHtml(lineDiff(sourceAt(t.id, +a), sourceAt(t.id, +b)))}<p class="small muted">Versions never change. Rolling back saves a new version with the old code.</p></div></div>`;
}
function fieldsView(t) {
  const run = safeRun(t.id);
  if (!run) return `<p class="muted">Run the tool once to see its output fields.</p>`;
  const rows = [];
  const walk = (v, path) => {
    if (Array.isArray(v)) { if (v.length && typeof v[0] === "object") walk(v[0], path + "[]"); else rows.push([path, SIZE(v)]); return; }
    if (v && typeof v === "object") { for (const [k, x] of Object.entries(v)) walk(x, path ? path + "." + k : k); return; }
    rows.push([path, SIZE(v) + path.split(".").pop().length + 3]);
  };
  walk(run.out, "");
  const total = rows.reduce((n, r) => n + r[1], 0) || 1;
  const fb = { "issues[].fields.description": { unused: ["claude-code", "vscode"] }, "openTickets[].title": { unused: ["cursor"], used: ["claude-code", "vscode"] }, "openTickets[].key": { used: ["claude-code", "vscode", "cursor"] }, "issues[].key": { used: ["claude-code", "vscode"] }, "issues[].fields.summary": { used: ["claude-code"] }, "balance": { used: ["claude-code", "vscode", "cursor"] } };
  const gs = S.agents.filter((a) => a.grants.some((g) => kit(g.tk)?.tools.includes(t.id)));
  return `<div class="notice info" style="margin-bottom:14px">${ic("info")}<div>Agents say which fields they used with <code>report_field_usage</code>. mcpDeck also notices when a value from a field shows up in the agent's next call. Nothing changes until you choose to.</div></div>
  <div class="tablewrap"><table><thead><tr><th>Field</th><th>Share of output</th><th>Used by</th><th>Reported unused</th><th></th></tr></thead><tbody>
  ${rows.map(([p, b]) => { const f = fb[p] || {}; const dropped = gs.filter((a) => a.drops.some((d) => d.tool === t.id && d.path === p)); return `<tr><td class="name" style="font-weight:500">${esc(p)}</td><td style="min-width:160px"><div class="row nw"><div class="tl" style="display:block;width:100px"><div class="bar"><i class="read" style="left:0;width:${Math.max(2, (b / total) * 100)}%"></i></div></div><span class="small muted">${Math.round((b / total) * 100)}% · ${kb(b)}</span></div></td>
    <td><div class="row nw" style="gap:2px">${(f.used || []).map((x) => avatar(agent(x)?.client || x, 20)).join("") || `<span class="small muted">–</span>`}</div></td><td><div class="row nw" style="gap:2px">${(f.unused || []).map((x) => avatar(agent(x)?.client || x, 20)).join("") || `<span class="small muted">–</span>`}</div></td>
    <td class="num">${dropped.length ? `<span class="small muted">Left out for ${dropped.map((a) => esc(a.name)).join(", ")}</span>` : `<select class="input" style="width:auto;height:28px;font-size:12.5px" data-a="dropField" data-id="${t.id}" data-path="${esc(p)}" aria-label="Leave out for an agent"><option value="">Leave out for…</option>${gs.map((a) => `<option value="${a.id}">${esc(a.name)}</option>`).join("")}</select>`}</td></tr>`; }).join("")}
  </tbody></table></div><p class="small muted" style="margin-top:10px">"Leave out for" changes only that agent's output. To remove a field for everyone, edit the code and save a new version.</p>`;
}

PAGES.review = (id) => {
  const t = tool(id);
  if (!t) return notFound("tool", "#toolkits");
  const v = t.versions[0], k = S.toolkits.find((x) => x.tools.includes(id));
  const code = src(id);
  const uses = [...code.matchAll(/(\w+)\.(\w+)\(\{/g)].map(([, a, r]) => reqOf(a, r) && { a, r: reqOf(a, r) }).filter(Boolean);
  const rw = grantsFor(k.id).filter((g) => g.access === "read_write"), ro = grantsFor(k.id).filter((g) => g.access === "read");
  const done = t.status !== "pending";
  const ruleRow = (state, label, text) => `<div class="rule"><span class="ic ${state}">${ic(state === "ok" ? "check" : state === "bad" ? "x" : "clock", 12)}</span><div><b>${label}</b><div class="small muted">${text}</div></div></div>`;
  return head(`Review <span class="mono">${id}</span> v${v.v}`, `Saved by ${esc(agentName(v.by))} ${v.when} in session <a href="#activity" data-a="session" data-id="s-91c2">s-91c2</a>, through the <a href="#toolkit-${k.id}">${k.id}</a> toolkit.`,
    done ? `<a class="btn light" href="#tool-${id}">Open tool</a>` : `${copyBtn(`Your mcpDeck tool ${id} v${v.v} is waiting for review. `, "Copy note for the agent", "Copied", "light")}<button type="button" class="btn light" data-a="reject" data-id="${id}">Reject</button><button type="button" class="btn blue" data-a="approve" data-id="${id}">${ic("check", 14)}Approve and publish</button>`,
    `<a href="#toolkits">Toolkits</a>${ic("chev", 12)}<a href="#toolkit-${k.id}">${k.id}</a>${ic("chev", 12)}<span>${id}</span>`) + `
  <div class="grid cols-2"><section class="glass panel" style="min-width:0"><div class="panel-head"><h2>Code</h2><span class="chip ok">new tool</span><span class="grow"></span>${copyBtn(code, "Copy file", "Tool file copied")}</div>${diffHtml(code.trimEnd().split("\n").map((l) => ["add", l]))}</section>
  <div class="stack" style="gap:16px">
    <section class="glass panel"><div class="panel-head"><h2>Approval rules</h2><span class="grow"></span>${done ? statusChip(t.status) : `<span class="chip warn">5 of 6 passed</span>`}</div><div class="rules">
      ${ruleRow("wait", "Only reads", `Writes with <code>support.assignIssue</code> and <code>support.addComment</code>. A person approves every tool that writes.`)}
      ${ruleRow("ok", "Tests pass", `${t.tests.length} of ${t.tests.length} passed against saved examples.`)}
      ${ruleRow("ok", "Description check", "68 characters, nothing aimed at other agents.")}
      ${ruleRow("ok", "Uses only allowed APIs", `support is in ${k.id}'s build list.`)}
      ${ruleRow("ok", "Output size", "45 tokens on its tests. Limit 2,000.")}
      ${ruleRow("ok", "Not a duplicate", "No other tool uses support.assignIssue.")}
    </div></section>
    <section class="glass panel"><h2 style="margin-bottom:10px">Requests it sends</h2><div class="stack" style="gap:6px">${uses.map(({ a, r }) => `<div class="row nw">${m(r.method)}<code class="grow">${a}.${r.id}</code>${accessChip(r.access)}</div>`).join("")}</div></section>
    <section class="glass panel"><h2 style="margin-bottom:10px">Who gets it</h2><p class="small">${rw.length ? `Agents with read and write on ${k.id}: ${rw.map((g) => `<b>${esc(g.agent.name)}</b>`).join(", ")}. They get it straight away.` : `No agent has read and write on ${k.id}, so nobody can call it yet.`}</p>${ro.length ? `<p class="small muted" style="margin-top:6px">${ro.map((g) => esc(g.agent.name)).join(", ")} ${ro.length === 1 ? "has" : "have"} read access and won't see it.</p>` : ""}</section>
  </div></div>`;
};

PAGES.agents = () => {
  const acts = `<button type="button" class="btn dark" data-a="connect" ${S.toolkits.length ? "" : "disabled"}>${ic("agent")}Connect agent</button>`;
  const h = head("Agents", "Each agent is one client, like Claude Code on this laptop, with its own token. It only sees the toolkits you grant it.", acts);
  if (!S.agents.length) return h + `<section class="glass panel">${empty("agent", "No agents connected", S.toolkits.length ? "Connect Claude Code, VS Code, Cursor or Claude Desktop to a toolkit. It takes one command or one click." : "Create a toolkit first. Agents connect to toolkits.", S.toolkits.length ? acts : `<a class="btn dark" href="#toolkits">Go to toolkits</a>`)}</section>`;
  return h + `<section class="glass panel"><div class="tablewrap"><table><thead><tr><th>Agent</th><th>Toolkits</th><th>Status</th><th class="num">Calls today</th><th class="num">Errors</th><th class="num">Writes blocked</th></tr></thead><tbody>
  ${S.agents.map((a) => `<tr class="click" data-go="#agent-${a.id}"><td><div class="row nw">${avatar(a.client, 30)}<div><b>${esc(a.name)}</b><div class="sub">${CLIENTS[a.client].label}</div></div></div></td>
    <td><div class="stack" style="gap:4px">${a.grants.map((g) => `<div class="row nw" style="gap:6px"><span class="name" style="font-weight:500">${g.tk}</span>${accessChip(g.access)}${g.build ? `<span class="chip neutral">builds</span>` : ""}</div>`).join("") || `<span class="muted small">none</span>`}</div></td>
    <td><span class="row nw small"><span class="dot ${a.live ? "" : "off"}"></span>${a.revoked ? "Revoked" : a.live ? "Connected now" : "Last seen " + (a.lastSeen || "never")}</span></td><td class="num">${a.today}</td><td class="num">${a.errors || "–"}</td><td class="num">${a.blocked || "–"}</td></tr>`).join("")}
  </tbody></table></div></section>`;
};

PAGES.agent = (id) => {
  const a = agent(id);
  if (!a) return notFound("agent", "#agents");
  const t = tab("agent-" + id, "toolkits");
  const sessions = [...new Set(S.calls.filter((c) => c.agent === id).map((c) => c.session))];
  const h = head(esc(a.name), `${CLIENTS[a.client].label}${a.version ? " · " + esc(a.version) : ""} · connected since ${a.since}`,
    a.revoked ? `<span class="chip bad">Revoked</span>` : `<button type="button" class="btn light" data-a="rotate" data-agent="${id}">${ic("key", 14)}Rotate token</button><button type="button" class="btn danger" data-a="revoke" data-agent="${id}">${ic("ban", 14)}Revoke</button>`,
    `<a href="#agents">Agents</a>${ic("chev", 12)}<span>${esc(a.name)}</span>`);
  const cards = `<div class="grid cols-4">
    <div class="card stat"><div class="label">Status</div><div class="row nw" style="margin-top:6px"><span class="dot ${a.live && !a.revoked ? "" : "off"}"></span><b>${a.revoked ? "Revoked" : a.live ? "Connected now" : "Last seen " + a.lastSeen}</b></div></div>
    <div class="card stat"><div class="label">Token</div><div class="mono" style="margin-top:6px">${a.token}</div><div class="hint">Stored in ${esc(a.where)}</div></div>
    <div class="card stat"><div class="label">Calls today</div><div class="value">${a.today}</div></div>
    <div class="card stat"><div class="label">Errors · writes blocked</div><div class="value">${a.errors} · ${a.blocked}</div></div></div>`;
  const tabs = tabsBar("agent-" + id, [["toolkits", "Toolkits", a.grants.length], ["sessions", "Sessions", sessions.length], ["output", "Trimmed output", a.drops.length || ""]], "toolkits");
  let body = "";
  if (t === "toolkits") body = grantTable(a.grants.map((g) => ({ ...g, agent: a })), "agent", id) + `<div class="row" style="margin-top:12px"><button type="button" class="btn light sm" data-a="connect" data-agent="${id}" ${a.revoked ? "disabled" : ""}>${ic("plus", 14)}Grant another toolkit</button></div>`;
  if (t === "sessions") body = sessions.length ? `<div class="tablewrap"><table><thead><tr><th>Session</th><th>Toolkit</th><th>Started</th><th class="num">Calls</th><th class="num">Tokens returned</th><th></th></tr></thead><tbody>${sessions.map((s) => { const cs = S.calls.filter((c) => c.session === s); return `<tr class="click" data-a="session" data-id="${s}"><td class="name">${s}</td><td>${cs[0].tk}</td><td>${cs[cs.length - 1].t.slice(0, 5)}</td><td class="num">${cs.length}</td><td class="num">${fmt(cs.reduce((n, c) => n + tok(c.bytes || (c.tool ? SIZE(safeRun(c.tool, c.input)?.out || "") : 0)), 0))}</td><td class="num"><span class="btn ghost sm">Open in Activity</span></td></tr>`; }).join("")}</tbody></table></div>` : `<p class="muted">No sessions yet. They show up as soon as the client connects.</p>`;
  if (t === "output") body = a.drops.length ? `<div class="stack">${a.drops.map((d, i) => `<div class="item"><div class="body"><b class="mono">${d.tool}</b><span class="sub">Leaves out <code>${esc(d.path)}</code> for this agent only. Approved by you.</span></div><button type="button" class="btn ghost sm" data-a="undrop" data-agent="${id}" data-i="${i}">Put back</button></div>`).join("")}</div>` : `<p class="muted">This agent gets every tool's full output. Fields can be left out for one agent from a tool's Output fields tab, or by accepting a suggestion.</p>`;
  return h + cards + `<section class="glass panel" style="padding-top:8px">${tabs}${body}</section>`;
};

PAGES.activity = () => {
  const t = tab("activity", "calls");
  const h = head("Activity", "Every call agents made, what it cost, and every change to mcpDeck.", t === "calls" ? `${S.ui.filters.session ? `<button type="button" class="btn light" data-a="session" data-id="">${ic("x", 13)}Session ${S.ui.filters.session}</button>` : ""}` : "");
  const tabs = tabsBar("activity", [["calls", "Calls"], ["usage", "Usage"], ["audit", "Audit log"]], "calls");
  if (!S.calls.length && t !== "audit") return h + `<section class="glass panel" style="padding-top:8px">${tabs}${empty("activity", "No calls yet", "When an agent connects and calls a tool, every call shows here with time, size and tokens.", S.agents.length ? "" : `<button type="button" class="btn dark" data-a="connect" ${S.toolkits.length ? "" : "disabled"}>${ic("agent")}Connect agent</button>`)}</section>`;
  if (t === "usage") return h + `<section class="glass panel" style="padding-top:8px">${tabs}${usageView()}</section>`;
  if (t === "audit") return h + `<section class="glass panel" style="padding-top:8px">${tabs}<div class="row" style="margin-bottom:12px"><span class="small muted grow">Append-only. Each entry stores the hash of the one before it.</span><button type="button" class="btn light sm" data-a="verify">${ic("shield", 14)}Verify chain</button></div><div class="tablewrap"><table><thead><tr><th>When</th><th>Who</th><th>What</th><th>On</th><th>Detail</th></tr></thead><tbody>${S.audit.map(([w, a, what, on, d]) => `<tr><td class="small nowrap">${w}</td><td><span class="row nw" style="gap:6px">${who(a)}<span class="small">${esc(agentName(a))}</span></span></td><td>${esc(what)}</td><td class="mono small">${esc(on)}</td><td class="small muted">${esc(d)}</td></tr>`).join("") || `<tr><td colspan="5" class="muted">No changes yet.</td></tr>`}</tbody></table></div></section>`;
  const f = S.ui.filters;
  const rows = S.calls.filter((c) => (!f.agent || c.agent === f.agent) && (!f.tk || c.tk === f.tk) && (!f.session || c.session === f.session) && (f.kind === "all" || (f.kind === "errors" ? c.status !== "ok" : c.kind === f.kind)));
  const sel = S.calls.find((c) => c.id === S.ui.sel.call) || rows[0];
  const sec = (t) => { const [hh, mm, ss] = t.split(":").map(Number); return hh * 3600 + mm * 60 + ss; };
  const span = f.session && rows.length ? [Math.min(...rows.map((c) => sec(c.t))), Math.max(...rows.map((c) => sec(c.t) + c.ms / 1000))] : null;
  const maxMs = Math.max(...rows.map((c) => c.ms), 1);
  const wf = (c) => span ? `<div class="wf"><i class="${c.status === "ok" ? "" : "err"}" style="left:${((sec(c.t) - span[0]) / (span[1] - span[0] || 1)) * 96}%;width:${Math.max(2, (c.ms / 1000 / (span[1] - span[0] || 1)) * 100)}%"></i></div>` : `<div class="wf"><i class="${c.status === "ok" ? "" : "err"}" style="left:0;width:${Math.max(2, (c.ms / maxMs) * 100)}%"></i></div>`;
  const outBytes = (c) => c.bytes || (c.tool ? SIZE(safeRun(c.tool, c.input)?.out || "") : 0);
  const filters = `<div class="row" style="margin-bottom:12px">
    <select class="input" style="width:auto" data-a="filter" data-k="agent" aria-label="Agent"><option value="">All agents</option>${S.agents.map((a) => `<option value="${a.id}" ${f.agent === a.id ? "selected" : ""}>${esc(a.name)}</option>`).join("")}</select>
    <select class="input" style="width:auto" data-a="filter" data-k="tk" aria-label="Toolkit"><option value="">All toolkits</option>${S.toolkits.map((k) => `<option ${f.tk === k.id ? "selected" : ""}>${k.id}</option>`).join("")}</select>
    <div class="seg">${[["all", "All"], ["tool", "Tools"], ["build", "Build"], ["feedback", "Requests and reports"], ["errors", "Errors and blocks"]].map(([k, l]) => `<button type="button" class="${f.kind === k ? "on" : ""}" data-a="filter" data-k="kind" data-v="${k}">${l}</button>`).join("")}</div>
    <input class="input" type="search" style="width:200px" placeholder="customer_360" aria-label="Search calls" data-filter="tr.callrow"></div>`;
  return h + `<section class="glass panel" style="padding-top:8px">${tabs}${filters}<div class="split"><div class="tablewrap" style="margin:0"><table><thead><tr><th>Time</th><th>Agent</th><th>Name</th><th>Status</th><th class="num">Duration</th><th class="num">Returned</th><th>${span ? "Waterfall" : ""}</th></tr></thead><tbody>
    ${rows.map((c) => `<tr class="click callrow ${sel && c.id === sel.id ? "sel" : ""}" data-a="pickcall" data-id="${c.id}" data-text="${esc(c.name)} ${c.session}"><td class="small nowrap">${c.t}</td><td>${avatar(agent(c.agent)?.client || c.agent, 22)}</td><td><span class="name" style="font-weight:500">${esc(c.name)}</span><div class="sub">${{ tool: "tool", build: "build tool", list: "list tools", feedback: "agent feedback", blocked: "upstream request" }[c.kind]} · ${c.tk}</div></td>
      <td>${c.status === "ok" ? `<span class="chip ok">ok</span>` : c.status === "blocked" ? `<span class="chip bad">blocked</span>` : `<span class="chip warn">error</span>`}</td><td class="num">${c.ms} ms</td><td class="num nowrap">${fmt(tok(outBytes(c)))} tok</td><td>${wf(c)}</td></tr>`).join("") || `<tr><td colspan="7" class="muted">No calls match.</td></tr>`}
  </tbody></table></div>${sel ? callDetail(sel) : ""}</div></section>`;
};
function callDetail(c) {
  const t = tab("calldetail", "output");
  const run = c.tool && c.status === "ok" ? safeRun(c.tool, c.input) : null;
  const output = c.error ? c.error : run ? run.out : c.output ?? (c.kind === "feedback" ? "Stored against this agent and session. Nothing changes until a person approves a suggestion." : "(no body)");
  const up = run ? run.calls : [];
  const body = t === "input" ? `<pre>${hlJson(c.input || {})}</pre>` : t === "output" ? `<pre>${typeof output === "string" ? esc(output) : hlJson(output)}</pre>`
    : t === "upstream" ? (up.length ? `<div class="stack" style="padding:12px 14px">${up.map((u) => `<div class="row nw">${m(u[1])}<code class="grow" style="overflow:hidden;text-overflow:ellipsis">${u[0]}</code><span class="chip ok">${u[3]}</span><span class="small muted">${u[4]} ms · ${kb(SIZE(u[6]))}</span></div>`).join("")}</div>` : `<p class="small muted" style="padding:12px 14px">${c.status === "blocked" ? "Refused at the proxy before it left mcpDeck." : "No API calls."}</p>`)
    : `<pre>${esc(`Started   ${c.t}\nSession   ${c.session}\nDuration  ${c.ms} ms\nReturned  ${kb(c.bytes || SIZE(output))} · ${fmt(tok(c.bytes || SIZE(output)))} tokens (counted by mcpDeck)`)}</pre>`;
  return `<div class="card stack" style="gap:10px;position:sticky;top:16px"><div class="row nw"><b class="name grow" style="overflow:hidden;text-overflow:ellipsis">${esc(c.name)}</b><span class="small muted">${c.t}</span></div>
    <div class="row small muted" style="gap:6px">${avatar(agent(c.agent)?.client || c.agent, 20)}${esc(agentName(c.agent))} · <a href="#activity" data-a="session" data-id="${c.session}">${c.session}</a> · ${c.tk}</div>
    ${c.error ? `<div class="notice ${c.status === "blocked" ? "bad" : "warn"}">${ic("alert")}<div>${esc(c.error)}</div></div>` : ""}
    <div class="seg">${[["input", "Input"], ["output", "Output"], ["upstream", "API calls"], ["timing", "Timing"]].map(([k, l]) => `<button type="button" class="${t === k ? "on" : ""}" data-a="tab" data-key="calldetail" data-v="${k}">${l}</button>`).join("")}</div>
    <div class="resp">${body}</div>
    <div class="row">${copyBtn(prompts({ kind: "call", c })[0].text, c.status === "ok" ? "Copy for agent" : "Copy fix prompt", "Copied. Paste it into the agent.", "blue sm")}${c.tool ? `<button type="button" class="btn light sm" data-a="saveTestFromCall" data-id="${c.id}">Save as test</button><a class="btn light sm" href="#tool-${c.tool}">Open tool</a>` : ""}${c.kind === "blocked" ? `<a class="btn light sm" href="#agent-${c.agent}">Change access</a>` : ""}</div></div>`;
}
function usageView() {
  const u = S.ui.usage;
  const days = u.range === "24h" ? 24 : u.range === "7d" ? 7 : 30;
  const series = Array.from({ length: days }, (_, i) => u.range === "24h" ? [0, 0, 0, 0, 0, 0, 0, 2, 11, 48, 62, 63, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0][i] : Math.round(560 + 160 * Math.sin(i * 1.7) + (i % 3) * 40));
  if (u.range !== "24h") series[days - 1] = 186;
  const max = Math.max(...series);
  const label = u.range === "24h" ? ["00:00", "06:00", "12:00", "18:00", "now"] : u.range === "7d" ? ["Nov 19", "Nov 21", "Nov 23", "today"] : ["Oct 27", "Nov 6", "Nov 16", "today"];
  const sum = series.reduce((a, b) => a + b, 0);
  const spark = (seed) => { const pts = Array.from({ length: 14 }, (_, i) => 10 - 8 * Math.abs(Math.sin((i + seed) * 0.9))); return `<svg class="spark" width="84" height="22" viewBox="0 0 84 22" aria-hidden="true"><polyline fill="none" stroke="var(--accent)" stroke-width="1.5" points="${pts.map((y, i) => `${i * 6},${y + 6}`).join(" ")}"/><circle cx="78" cy="${pts[13] + 6}" r="2" fill="var(--accent)"/></svg>`; };
  let table;
  if (u.by === "tools") table = `<thead><tr><th>Tool</th><th class="num">Calls</th><th class="num">Errors</th><th class="num">p50</th><th class="num">p95</th><th class="num">Avg output</th><th class="num">API calls per call</th><th>Trend</th></tr></thead><tbody>${S.tools.filter((t) => t.calls7).map((t, i) => { const r = safeRun(t.id); return `<tr class="click" data-go="#tool-${t.id}"><td class="name">${t.id}</td><td class="num">${fmt(t.calls7)}</td><td class="num">${t.err}%</td><td class="num">${t.p50} ms</td><td class="num">${t.p95} ms</td><td class="num">${r ? fmt(tok(SIZE(r.out))) : "–"} tok</td><td class="num">${r ? r.calls.length : "–"}</td><td>${spark(i)}</td></tr>`; }).join("")}</tbody>`;
  if (u.by === "toolkits") table = `<thead><tr><th>Toolkit</th><th class="num">Calls</th><th class="num">Agents</th><th class="num">tools/list per session</th><th class="num">Errors</th><th>Trend</th></tr></thead><tbody>${S.toolkits.map((k, i) => `<tr class="click" data-go="#toolkit-${k.id}"><td class="name">${k.id}</td><td class="num">${fmt(k.tools.reduce((n, x) => n + (tool(x)?.calls7 || 0), 0))}</td><td class="num">${grantsFor(k.id).length}</td><td class="num">${fmt(toolsListTokens(k.id))} tok</td><td class="num">${i ? "2.4%" : "0.6%"}</td><td>${spark(i + 3)}</td></tr>`).join("")}</tbody>`;
  if (u.by === "agents") table = `<thead><tr><th>Agent</th><th class="num">Calls today</th><th class="num">Sessions</th><th class="num">Errors</th><th class="num">Writes blocked</th><th>Trend</th></tr></thead><tbody>${S.agents.map((a, i) => `<tr class="click" data-go="#agent-${a.id}"><td><span class="row nw">${avatar(a.client, 22)}<b>${esc(a.name)}</b></span></td><td class="num">${a.today}</td><td class="num">${new Set(S.calls.filter((c) => c.agent === a.id).map((c) => c.session)).size}</td><td class="num">${a.errors}</td><td class="num">${a.blocked}</td><td>${spark(i + 5)}</td></tr>`).join("")}</tbody>`;
  return `<div class="row" style="margin-bottom:14px"><div class="seg">${["24h", "7d", "30d"].map((r) => `<button type="button" class="${u.range === r ? "on" : ""}" data-a="usage" data-k="range" data-v="${r}">${r}</button>`).join("")}</div><div class="seg">${[["tools", "By tool"], ["toolkits", "By toolkit"], ["agents", "By agent"]].map(([k, l]) => `<button type="button" class="${u.by === k ? "on" : ""}" data-a="usage" data-k="by" data-v="${k}">${l}</button>`).join("")}</div><span class="small muted grow" style="text-align:right">Tokens are what mcpDeck returned to agents, counted by mcpDeck.</span></div>
  <div class="grid" style="grid-template-columns:minmax(0,1fr) minmax(0,2fr);margin-bottom:16px"><div class="grid" style="grid-template-columns:repeat(2,minmax(0,1fr))"><div class="card stat"><div class="label">Calls</div><div class="value">${fmt(sum)}</div></div><div class="card stat"><div class="label">Error rate</div><div class="value">1.1%</div></div><div class="card stat"><div class="label">p50 tool time</div><div class="value">142 ms</div></div><div class="card stat"><div class="label">Avg tokens per call</div><div class="value">330</div></div></div>
  <div class="card"><div class="row"><b class="grow">Calls per ${u.range === "24h" ? "hour" : "day"}</b><span class="small muted">max ${fmt(max)}</span></div><div class="bars" role="img" aria-label="Calls over time">${series.map((v, i) => `<i style="height:${(v / max) * 100}%" title="${v} calls"></i>`).join("")}</div><div class="axis">${label.map((l) => `<span>${l}</span>`).join("")}</div></div></div>
  <div class="tablewrap"><table>${table}</table></div>`;
}

PAGES.preferences = () => {
  const t = tab("prefs", "general");
  const groups = [["general", "General"], ["appearance", "Appearance"], ["security", "Security"], ["skills", "Agent skills"], ["data", "Data"], ["about", "About"]];
  const row = (l, d, r) => `<div class="setrow"><div class="l"><b>${l}</b>${d ? `<span class="small muted">${d}</span>` : ""}</div>${r}</div>`;
  const pages = {
    general: `<div class="setgroup">${row("Port", "Client configs use this port. Changing it needs a restart and new configs.", `<input class="input mono" style="width:110px" value="47800" aria-label="Port">`)}${row("Open the browser on start", "", `<input type="checkbox" class="toggle" checked aria-label="Open the browser on start">`)}${row("Tool timeout", "Default for new tools.", `<select class="input" style="width:auto" aria-label="Tool timeout"><option>10 s</option><option>30 s</option><option>60 s</option></select>`)}${row("Request timeout", "For each API call.", `<select class="input" style="width:auto" aria-label="Request timeout"><option>30 s</option><option>60 s</option></select>`)}</div>`,
    appearance: `<div class="setgroup">${row("Theme", "Follows your system unless you pick one.", `<div class="seg">${["system", "light", "dark"].map((x) => `<button type="button" class="${S.theme === x ? "on" : ""}" data-a="theme" data-v="${x}">${x[0].toUpperCase() + x.slice(1)}</button>`).join("")}</div>`)}</div>`,
    security: `<div class="setgroup">${row("Key store", `${isMac ? "macOS Keychain" : "Windows Credential Manager"} holds the key that encrypts your credentials.`, `<span class="chip ok">${ic("check", 11)}Working</span>`)}${row("Listening on", "Only this computer can connect. Requests from web pages are refused.", `<code>127.0.0.1:47800</code>`)}</div>
      <h3 style="margin-top:8px">What mcpDeck protects</h3><div class="setgroup" style="padding:12px 16px"><p class="small">Agents connected over MCP never receive credentials, and tool code can't read them. The database file on its own decrypts nothing. Each agent only sees the toolkits you grant, and read access can't write.</p></div>
      <h3 style="margin-top:8px">What it doesn't</h3><div class="setgroup" style="padding:12px 16px"><p class="small">A program running as you on this computer, including an agent with a shell, can ask your key store or call an API directly. mcpDeck controls what agents do through MCP; it doesn't sandbox the agent.</p></div>`,
    skills: `<p class="small muted" style="margin-bottom:10px">Skills teach agents to use mcpDeck well. "Connect agent" offers to install them; install or update them here too.</p><div class="setgroup">${[["mcpdeck-tools", "Picking tools and handling errors"], ["mcpdeck-request-tool", "Asking for a tool people can act on"], ["mcpdeck-trim-output", "Measuring output and reporting fields · field-sizes.mjs"], ["mcpdeck-build-tool", "Building and checking a tool · check-tool.mjs"]].map(([n, d]) => row(`<span class="mono">${n}</span>`, d, `<span class="small muted">${S.agents.filter((a) => a.client !== "other").length ? "Installed for " + [...new Set(S.agents.map((a) => CLIENTS[a.client].label))].join(", ") : "Not installed"}</span>`)).join("")}</div><div class="row" style="margin-top:10px"><button type="button" class="btn light sm" data-a="toastOnly" data-msg="Skills updated in every place they were installed">Update installed skills</button></div>`,
    data: `<div class="setgroup">${row("Data folder", "One SQLite database. Deleting the folder resets mcpDeck.", `<code class="small">${isMac ? "~/Library/Application Support/mcpDeck" : "%LOCALAPPDATA%\\mcpDeck"}</code><button type="button" class="btn light sm" data-a="toastOnly" data-msg="Opened the data folder">Reveal</button>`)}${row("Keep call bodies", "Inputs and outputs of calls.", `<select class="input" style="width:auto" aria-label="Keep call bodies"><option>30 days</option><option>7 days</option><option>90 days</option></select>`)}${row("Keep call details", "Who, when, how long, tokens.", `<select class="input" style="width:auto" aria-label="Keep call details"><option>1 year</option><option>90 days</option></select>`)}</div>`,
    about: `<div class="setgroup">${row("mcpDeck", "Version, commit and build date of this binary.", `<code>mcpdeck version</code>`)}${row("Updates", "mcpDeck never updates itself. Check GitHub for a newer release.", `<button type="button" class="btn light sm" data-a="toastOnly" data-msg="Checked GitHub releases">Check for updates</button>`)}${row("Docs", "", `<a href="https://github.com/abhijeet-oxide/mcpDeck" target="_blank" rel="noopener">github.com/abhijeet-oxide/mcpDeck</a>`)}</div>`,
  };
  return head("Settings", "", "") + `<div class="settings"><nav class="glass" style="padding:10px" aria-label="Settings sections">${groups.map(([k, l]) => `<button type="button" class="nav ${t === k ? "on" : ""}" data-a="tab" data-key="prefs" data-v="${k}">${l}</button>`).join("")}</nav><section class="glass panel stack">${`<h2>${groups.find((g) => g[0] === t)[1]}</h2>`}${pages[t]}</section></div>`;
};
PAGES.tools = () => { S.ui.tabs.toolkits = "tools"; return PAGES.toolkits(); };
PAGES.usage = () => { S.ui.tabs.activity = "usage"; return PAGES.activity(); };
PAGES.audit = () => { S.ui.tabs.activity = "audit"; return PAGES.activity(); };
const notFound = (what, back) => `<section class="glass panel">${empty("alert", `That ${what} doesn't exist`, "It may have been deleted, or the link is from the demo world.", `<a class="btn dark" href="${back}">Back</a>`)}</section>`;

/* ---------- editor ---------- */
function mountEditor() {
  const ta = $("#code"); if (!ta) return;
  const id = ta.dataset.tool;
  const hl = $("#hl"), gut = $("#gutter");
  let pop = null, items = [], idx = 0, ctx = null;
  const charW = (() => { const s = document.createElement("span"); s.style.font = getComputedStyle(ta).font; s.style.position = "absolute"; s.style.visibility = "hidden"; s.textContent = "0".repeat(100); document.body.append(s); const w = s.getBoundingClientRect().width / 100; s.remove(); return w || 7.5; })();
  const paint = () => {
    hl.innerHTML = hlTs(ta.value) + "\n";
    const n = ta.value.split("\n").length;
    gut.innerHTML = Array.from({ length: n }, (_, i) => `<div>${i + 1}</div>`).join("");
    ta.style.height = Math.max(520, n * 20 + 30) + "px";
    problems();
  };
  const problems = () => {
    const c = ta.value; const p = [];
    if (/\bfetch\s*\(/.test(c)) p.push(["error", "fetch() isn't available; call an API client like crm.getAccount() instead"]);
    if (/^\s*import\s+(?!type\b)/m.test(c)) p.push(["error", "imports aren't available; API clients, call() and vars are globals"]);
    if (/\bprocess\./.test(c)) p.push(["error", "process isn't available; use vars.name"]);
    if (!/export\s+default\s+async\s+function/.test(c)) p.push(["error", "missing export default async function"]);
    if (!/description:\s*"[^"]{20,}"/.test(c)) p.push(["error", "description must say what the tool returns, in at least 20 characters"]);
    if (/\breturn\s+[A-Za-z_$][\w$]*\s*;/.test(c)) p.push(["warning", "returns a whole response; return only the fields the agent needs"]);
    for (const [, cl, me] of c.matchAll(/\b(\w+)\.(\w+)\(\{/g)) if (api(cl) && !api(cl).requests.some((r) => r.id === me)) p.push(["error", `${cl} has no request named ${me}`]);
    $("#problems").innerHTML = p.length ? p.map(([k, t]) => `<span class="chip ${k === "error" ? "bad" : "warn"}">${k}</span><span>${esc(t)}</span>`).join("") : `<span class="chip ok">${ic("check", 11)}No problems</span><span class="muted">TypeScript checks run in the real editor. Type <code>${S.apis.find((a) => tool(id)?.apis.includes(a.id))?.id || "crm"}.</code> for completions.</span>`;
  };
  const close = () => { pop?.remove(); pop = null; };
  const complete = () => {
    const before = ta.value.slice(0, ta.selectionStart);
    const mm = /(\w+)\.(\w*)$/.exec(before);
    if (!mm || !api(mm[1])) return close();
    const a = api(mm[1]);
    items = a.requests.filter((r) => r.id.toLowerCase().startsWith(mm[2].toLowerCase()));
    if (!items.length) return close();
    ctx = { prefix: mm[2] }; idx = 0;
    const lines = before.split("\n");
    const top = 12 + (lines.length) * 20 - ta.parentElement.scrollTop;
    const left = Math.min(58 + lines[lines.length - 1].length * charW, ta.clientWidth - 330);
    if (!pop) { pop = document.createElement("div"); pop.className = "complete"; pop.setAttribute("role", "listbox"); $("#editor").append(pop); }
    pop.style.top = top + "px"; pop.style.left = Math.max(58, left) + "px";
    const draw = () => { pop.innerHTML = items.map((r, i) => `<div class="${i === idx ? "on" : ""}" data-i="${i}" role="option"><span class="m ${r.method}" style="width:44px;font-size:9.5px">${r.method}</span><span>${r.id}</span><span class="t">${r.method} ${esc(r.path)} · ${r.access}</span></div>`).join(""); };
    draw(); pop.draw = draw;
  };
  const accept = () => {
    const r = items[idx]; if (!r) return;
    const s = ta.selectionStart; const args = r.params?.length ? `{ ${r.params.map(([k]) => `${k}: ${JSON.stringify(r.params[0][1])}`).slice(0, 1).join(", ")} }` : r.body ? "{ body: {} }" : "";
    const ins = r.id.slice(ctx.prefix.length) + `(${args})`;
    ta.setRangeText(ins, s, s, "end"); close(); S.ui.drafts[id] = ta.value; paint();
  };
  ta.addEventListener("input", () => { S.ui.drafts[id] = ta.value; paint(); complete(); });
  ta.addEventListener("keydown", (e) => {
    if (pop && ["ArrowDown", "ArrowUp"].includes(e.key)) { e.preventDefault(); idx = (idx + (e.key === "ArrowDown" ? 1 : items.length - 1)) % items.length; pop.draw(); return; }
    if (pop && (e.key === "Enter" || e.key === "Tab")) { e.preventDefault(); accept(); return; }
    if (pop && e.key === "Escape") { close(); return; }
    if (e.key === "Tab") { e.preventDefault(); ta.setRangeText("  ", ta.selectionStart, ta.selectionEnd, "end"); S.ui.drafts[id] = ta.value; paint(); }
  });
  ta.addEventListener("blur", () => setTimeout(close, 150));
  ta.addEventListener("scroll", () => { hl.scrollLeft = ta.scrollLeft; });
  $("#editor").addEventListener("mousedown", (e) => { const d = e.target.closest(".complete [data-i]"); if (d) { e.preventDefault(); idx = +d.dataset.i; accept(); } });
  paint();
}

/* ---------- modals ---------- */
function openModal(state) { S.modal = state; renderModal(); }
function closeModal() { S.modal = null; clearTimeout(S.connectTimer); $("#layer").innerHTML = ""; }
function renderModal() {
  const md = S.modal; if (!md) return;
  const html = { addApi: addApiModal, connect: connectModal, newToolkit: newToolkitModal, newTool: newToolModal, library: libraryModal, confirm: confirmModal, palette: paletteModal, suggestion: suggestionDrawer, secret: secretModal }[md.type](md);
  $("#layer").innerHTML = md.type === "suggestion" ? `<div class="scrim" data-a="scrim" style="place-items:stretch end;padding:0"></div>${html}` : `<div class="scrim" data-a="scrim">${html}</div>`;
  const f = $("#layer [autofocus]"); if (f) f.focus();
}
const modalShell = (title, sub, body, foot, wide) => `<div class="modal ${wide ? "wide" : ""}" role="dialog" aria-modal="true" aria-label="${esc(title)}"><div class="modal-head"><div class="grow"><h2 style="font-size:19px">${title}</h2>${sub ? `<p class="small muted">${sub}</p>` : ""}</div><button type="button" class="iconbtn" data-a="close" aria-label="Close">${ic("x")}</button></div><div class="modal-body">${body}</div>${foot ? `<div class="modal-foot">${foot}</div>` : ""}</div>`;

const SOURCES = [
  ["swagger", "Swagger or Redoc page", "The docs page URL. mcpDeck finds the spec."],
  ["openapi", "OpenAPI file or URL", "3.x or Swagger 2.0, JSON or YAML"],
  ["postman", "Postman collection", "v2.1 export, with environments"],
  ["curl", "curl commands", "Paste one or more"],
  ["blank", "Start empty", "Add requests by hand"],
];
function addApiModal(md) {
  if (md.step === 2) {
    const r = md.result, into = api(md.target);
    return modalShell(`Imported ${plural(r.added.length, "request")} into <span class="mono">${md.target}</span>`, esc(r.from), `
      ${r.moved.length ? `<div class="notice ok">${ic("lock")}<div>${r.moved.map((x) => `Moved <code>${esc(x)}</code> out of the request into an encrypted credential.`).join("<br>")}</div></div>` : ""}
      <div class="row small"><span class="chip info">${r.added.filter((x) => x.access === "read").length} read</span><span class="chip warn">${r.added.filter((x) => x.access === "write").length} write</span>${r.added.some((x) => x.access === "check") ? `<span class="chip warn">${r.added.filter((x) => x.access === "check").length} to check</span>` : ""}<span class="muted">Read or write comes from the method. Change it on any request.</span></div>
      <div class="card" style="max-height:300px;overflow:auto;padding:6px 10px">${r.added.map((x) => `<div class="row nw" style="padding:5px 0;border-bottom:1px solid var(--line)">${m(x.method)}<code class="grow" style="overflow:hidden;text-overflow:ellipsis">${x.id}</code><span class="small muted" style="overflow:hidden;text-overflow:ellipsis;white-space:nowrap;max-width:45%">${esc(x.path)}</span></div>`).join("") || `<p class="small muted">No requests. Add them with New request.</p>`}</div>
      ${into && into.authType !== "None" && !Object.values(into.creds || {}).some(Boolean) ? `<div class="notice info">${ic("key")}<div>${esc(into.authType)} auth was found. Add the credential before sending requests.</div></div>` : ""}`,
      `<a class="btn light" href="#api-${md.target}" data-a="close" data-tab="credentials">Add credentials</a><a class="btn dark" href="#api-${md.target}" data-a="close">Open ${md.target}</a>`);
  }
  const s = md.src;
  const input = s === "curl" ? `<label class="field">curl commands<textarea class="input" id="imp-text" rows="7" autofocus placeholder="curl https://api.example.com/v1/orders?status=open -H 'X-Api-Key: …'">${esc(md.value || "")}</textarea><span class="hint">Headers that carry credentials (Authorization, Cookie, X-Api-Key) are moved into an encrypted credential. They're never saved on the request.</span></label>`
    : s === "blank" ? `<label class="field">Base URL<input class="input mono" id="imp-text" autofocus placeholder="https://erp.internal.example/api" value="${esc(md.value || "")}"><span class="hint">Becomes <code>{{baseUrl}}</code> in the first environment.</span></label>`
    : s === "postman" ? `<label class="field">Collection file<input class="input" type="file" id="imp-file" accept=".json" style="padding-top:6px"><span class="hint">Postman: Collection › Export › v2.1. Folders, variables, auth and saved responses come across; scripts stay off until you trust them.</span></label>`
    : `<label class="field">${s === "swagger" ? "Docs page URL" : "Spec URL"}<input class="input mono" id="imp-text" autofocus placeholder="${s === "swagger" ? "https://petstore3.swagger.io/" : "https://petstore3.swagger.io/api/v3/openapi.json"}" value="${esc(md.value || "")}"></label>${s === "openapi" ? `<div class="row small"><span class="muted">or</span><label class="btn light sm" style="cursor:pointer">${ic("file", 13)}Choose a file<input type="file" id="imp-file" accept=".json,.yaml,.yml" class="sr"></label></div>` : ""}`;
  const target = md.into ? `<div class="notice info">${ic("api")}<div>Into <b class="mono">${md.into}</b>. New requests are added and matching ones updated; nothing is deleted.</div></div>`
    : `<label class="field">API name<input class="input mono" id="imp-name" value="${esc(md.name || "")}" placeholder="billing" pattern="[a-z][a-z0-9_]*"><span class="hint">One API per system. Tools call it by this name: <code id="imp-preview">${esc(md.name || "billing")}.getInvoice()</code>. Leave empty to use the spec's title or the host name.</span></label>`;
  return modalShell(md.into ? `Import into ${md.into}` : "Add an API", md.into ? "" : "An API is one system you call, like your CRM. Bring its requests from wherever they are now.", `
    <div class="optgrid" role="radiogroup" aria-label="Source">${SOURCES.filter(([k]) => !(md.into && k === "blank")).map(([k, t, d]) => `<button type="button" role="radio" aria-checked="${s === k}" class="opt ${s === k ? "on" : ""}" data-a="impSrc" data-v="${k}"><span class="t">${t}</span><span class="d">${d}</span></button>`).join("")}</div>
    ${input}${target}${md.error ? `<div class="notice bad">${ic("alert")}<div>${esc(md.error)}</div></div>` : ""}`,
    `<span class="left small muted">From the terminal: <code>mcpdeck import &lt;url-or-file&gt; --api &lt;name&gt;</code></span><button type="button" class="btn light" data-a="close">Cancel</button><button type="button" class="btn dark" data-a="doImport">${s === "blank" ? "Create API" : "Import"}</button>`);
}
function shellSplit(cmd) {
  const out = []; let cur = "", q = null, i = 0, has = false;
  cmd = cmd.replace(/\\\r?\n/g, " ");
  for (; i < cmd.length; i++) {
    const c = cmd[i];
    if (q) { if (c === q) q = null; else if (c === "\\" && q === '"') cur += cmd[++i]; else cur += c; }
    else if (c === "'" || c === '"') { q = c; has = true; }
    else if (/\s/.test(c)) { if (cur || has) out.push(cur); cur = ""; has = false; }
    else if (c === "\\") cur += cmd[++i] ?? ""; else cur += c;
  }
  if (cur || has) out.push(cur);
  return out;
}
function parseCurls(text) {
  const cmds = text.split(/\n(?=\s*curl\s)/).map((x) => x.trim()).filter((x) => x.startsWith("curl"));
  if (!cmds.length) throw new Error("No curl command found. Each command starts with curl.");
  const reqs = [], moved = []; let host = null, base = null;
  for (const c of cmds) {
    const a = shellSplit(c).slice(1); let method = null, url = null, body = null; const headers = [];
    for (let i = 0; i < a.length; i++) {
      const x = a[i];
      if (x === "-X" || x === "--request") method = a[++i];
      else if (x === "-H" || x === "--header") headers.push(a[++i]);
      else if (["-d", "--data", "--data-raw", "--data-binary", "--json"].includes(x)) body = a[++i];
      else if (x === "-u" || x === "--user") { a[++i]; moved.push("basic auth (-u)"); }
      else if (x === "--url") url = a[++i];
      else if (!x.startsWith("-") && !url) url = x;
    }
    if (!url) throw new Error("A curl command has no URL.");
    let u; try { u = new URL(url); } catch { throw new Error(`"${url}" isn't a full URL. Include https://.`); }
    host = host || u.host; base = base || `${u.protocol}//${u.host}`;
    for (const h of headers) { const k = h.split(":")[0].trim(); if (/^(authorization|cookie|x-api-key|api-key)$/i.test(k)) moved.push(k + " header"); }
    for (const k of [...u.searchParams.keys()]) if (/^(api_key|apikey|token|key|access_token)$/i.test(k)) { moved.push(`query ${k}`); u.searchParams.delete(k); }
    method = (method || (body ? "POST" : "GET")).toUpperCase();
    const segs = u.pathname.split("/").filter(Boolean).filter((s) => !/^\d+$|^[0-9a-f-]{8,}$/i.test(s));
    const nm = (method === "GET" ? "get" : method === "POST" ? "create" : method === "DELETE" ? "delete" : "update") + (segs.slice(-2).map((s) => s.replace(/[^a-z0-9]/gi, "")).map((s) => s[0].toUpperCase() + s.slice(1)).join("") || "Root");
    let id = nm, n = 2; while (reqs.some((r) => r.id === id)) id = nm + n++;
    reqs.push(R(id, segs[0] || "requests", method, u.pathname.replace(/\/\d+(?=\/|$)/g, "/{id}") , ["GET", "HEAD", "OPTIONS"].includes(method) ? "read" : "write", { query: [...u.searchParams.entries()], body, params: /\/\d+(\/|$)/.test(u.pathname) ? [["id", /\/(\d+)(?:\/|$)/.exec(u.pathname)[1]]] : [] }));
  }
  return { reqs, moved: [...new Set(moved)], host, base };
}
function parseSpec(text, name) {
  let j; try { j = JSON.parse(text); } catch { throw new Error("This mockup reads JSON files only. mcpDeck itself also reads YAML."); }
  if (j.openapi || j.swagger) {
    const reqs = [];
    for (const [p, ops] of Object.entries(j.paths || {})) for (const [mth, op] of Object.entries(ops)) {
      if (!["get", "post", "put", "patch", "delete", "head", "options"].includes(mth)) continue;
      const M = mth.toUpperCase(); const id = op.operationId || (mth + p.replace(/[{}]/g, "").split("/").filter(Boolean).map((s) => s[0].toUpperCase() + s.slice(1)).join(""));
      reqs.push(R(id, op.tags?.[0] || p.split("/")[1] || "requests", M, p, ["GET", "HEAD", "OPTIONS"].includes(M) ? (/(delete|remove|update|create|set|reset)/i.test(p) ? "check" : "read") : "write", {}));
    }
    return { reqs, kind: j.openapi ? `OpenAPI ${j.openapi}` : `Swagger ${j.swagger}`, title: j.info?.title, base: j.servers?.[0]?.url || (j.host ? `https://${j.host}${j.basePath || ""}` : ""), moved: [] };
  }
  if (j.info?._postman_id || /postman/.test(j.info?.schema || "")) {
    const reqs = [];
    const walk = (items, folder) => items.forEach((it) => { if (it.item) walk(it.item, it.name); else if (it.request) { const M = (it.request.method || "GET").toUpperCase(); const raw = typeof it.request.url === "string" ? it.request.url : it.request.url?.raw || ""; reqs.push(R(it.name.replace(/[^a-z0-9]+(.)?/gi, (s, c) => (c ? c.toUpperCase() : "")).replace(/^./, (c) => c.toLowerCase()), folder || "requests", M, raw.replace(/^\{\{\w+\}\}/, "").split("?")[0] || "/", ["GET", "HEAD", "OPTIONS"].includes(M) ? "read" : "write", {})); } });
    walk(j.item || []);
    return { reqs, kind: "Postman collection", title: j.info?.name, base: "", moved: [] };
  }
  throw new Error("That file isn't an OpenAPI, Swagger or Postman collection.");
}
function finishImport(md, parsed, from) {
  const name = (md.into || md.name || (parsed.title || parsed.host || "api").toLowerCase().replace(/^api\./, "").split(/[.\s-]/)[0].replace(/[^a-z0-9_]/g, "")) || "api";
  if (!md.into && api(name)) { md.error = `There's already an API named ${name}. Pick another name, or open ${name} and import into it.`; return renderModal(); }
  let a = api(name);
  if (!a) {
    a = { id: name, title: parsed.title || name, kind: parsed.kind || "curl", from, authType: parsed.moved.length ? (parsed.moved.some((x) => /authorization/i.test(x)) ? "Bearer token" : "API key (header X-Api-Key)") : "None", envs: ["Local"], vars: [["baseUrl", parsed.base || "", "", "text"]], creds: parsed.moved.length ? { Local: "just now" } : {}, hosts: parsed.base ? [new URL(parsed.base, "http://x").host] : [], scripts: null, requests: [] };
    S.apis.push(a);
  }
  for (const r of parsed.reqs) { const i = a.requests.findIndex((x) => x.id === r.id); if (i >= 0) a.requests[i] = r; else a.requests.push(r); }
  S.ui.open[a.id] = a.requests.slice(0, 1).map((r) => r.id);
  S.audit.unshift(["now", "you", md.into ? "Imported requests" : "Added API", a.id, `${parsed.reqs.length} requests from ${from}`]);
  openModal({ type: "addApi", step: 2, target: a.id, result: { added: parsed.reqs, moved: parsed.moved, from } });
  route();
}
function doImport() {
  const md = S.modal; md.error = null;
  md.value = $("#imp-text")?.value.trim() || ""; md.name = $("#imp-name")?.value.trim() || md.name || "";
  if (md.name && !/^[a-z][a-z0-9_]*$/.test(md.name)) { md.error = "API names are lowercase letters, digits and _, starting with a letter, because tools use them in code."; return renderModal(); }
  const file = $("#imp-file")?.files?.[0];
  if (file) { const fr = new FileReader(); fr.onload = () => { try { finishImport(md, parseSpec(String(fr.result), md.name), file.name); } catch (e) { md.error = e.message; renderModal(); } }; fr.readAsText(file); return; }
  try {
    if (md.src === "curl") { if (!md.value) throw new Error("Paste at least one curl command."); const p = parseCurls(md.value); return finishImport(md, { ...p, kind: "curl" }, "pasted curl"); }
    if (md.src === "blank") { let u; try { u = new URL(md.value); } catch { throw new Error("Enter the base URL, like https://erp.internal.example/api."); } return finishImport(md, { reqs: [], moved: [], base: md.value, host: u.host, kind: "by hand", title: md.name || u.host.split(".")[0] }, "by hand"); }
    if (md.src === "postman") throw new Error("Choose the exported collection file.");
    if (!md.value) throw new Error(md.src === "swagger" ? "Enter the docs page URL." : "Enter the spec URL, or choose a file.");
    let u; try { u = new URL(md.value); } catch { throw new Error("That isn't a URL. Include https://."); }
    const demo = { "127.0.0.1:9001": "crm", "127.0.0.1:9002": "billing", "127.0.0.1:9003": "support", "127.0.0.1:9004": "legacy" }[u.host];
    if (demo) { const d = demoWorld().apis.find((x) => x.id === demo); if (!md.into && api(demo)) throw new Error(`${demo} is already here. Open it and use Import to update it.`); return finishImport({ ...md, name: md.name || demo }, { reqs: d.requests, moved: [], base: `http://${u.host}`, kind: d.kind, title: d.title }, md.value); }
    if (/petstore/.test(u.host)) return finishImport(md, { reqs: PETSTORE.map(([f, id, M, p]) => R(id, f, M, p, ["GET"].includes(M) ? "read" : "write", {})), moved: [], base: "https://petstore3.swagger.io/api/v3", kind: "OpenAPI 3.0.4", title: "Swagger Petstore - OpenAPI 3.0", host: "petstore" }, u.host === "petstore3.swagger.io" && u.pathname.endsWith(".json") ? md.value : `${md.value} → /api/v3/openapi.json`);
    throw new Error(md.src === "swagger" ? "This mockup can't reach the network. Try https://petstore3.swagger.io/ or a demo URL like http://127.0.0.1:9002/." : "This mockup can't reach the network. Try https://petstore3.swagger.io/api/v3/openapi.json, or choose a JSON file.");
  } catch (e) { md.error = e.message; renderModal(); }
}

function token() { const a = "abcdefghijkmnpqrstuvwxyz23456789"; let s = "mcpd_"; const r = crypto.getRandomValues(new Uint8Array(32)); for (const b of r) s += a[b % a.length]; return s; }
function connectModal(md) {
  const c = CLIENTS[md.client];
  if (md.step === 1) return modalShell("Connect an agent", "Pick the app your agent runs in. mcpDeck creates a token for it and shows the one command or click that adds the toolkit.", `
    <div class="optgrid" role="radiogroup" aria-label="Client">${Object.entries(CLIENTS).map(([k, x]) => `<button type="button" role="radio" aria-checked="${md.client === k}" class="opt ${md.client === k ? "on" : ""}" data-a="cSet" data-f="client" data-v="${k}"><span class="row nw">${avatar(k, 24)}<span class="t">${x.label}</span></span><span class="d">${{ "claude-code": "One terminal command", vscode: "Opens VS Code to install", cursor: "Opens Cursor to install", "claude-desktop": "Writes its config file", other: "URL and header" }[k]}</span></button>`).join("")}</div>
    <div class="grid" style="grid-template-columns:minmax(0,1fr) minmax(0,1fr)">
      <label class="field">Toolkit<select class="input" data-a="cSet" data-f="tk" aria-label="Toolkit">${S.toolkits.map((k) => `<option ${md.tk === k.id ? "selected" : ""}>${k.id}</option>`).join("")}</select><span class="hint">${esc(kit(md.tk)?.desc || "")}</span></label>
      <label class="field">Name<input class="input" id="c-name" value="${esc(md.name ?? (md.client === "other" ? "" : c.label.replace(" (Copilot)", "") + " on this computer"))}" placeholder="Support bot"><span class="hint">How it shows up in Activity.</span></label>
    </div>
    <div class="setgroup">
      <div class="setrow"><div class="l"><b>Access</b><span class="small muted">${md.access === "read" ? "Write requests are refused and write tools aren't listed." : "Can call tools that create, change or delete."}</span></div><div class="seg"><button type="button" class="${md.access === "read" ? "on" : ""}" data-a="cSet" data-f="access" data-v="read">Read</button><button type="button" class="${md.access === "read_write" ? "on" : ""}" data-a="cSet" data-f="access" data-v="read_write">Read and write</button></div></div>
      <label class="setrow" style="cursor:pointer"><div class="l"><b>May build tools</b><span class="small muted">Gets search_apis, run_api, save_tool and the other build tools for ${(kit(md.tk)?.scope || []).join(", ") || "this toolkit's APIs"}. What it saves goes through the toolkit's approval rules.</span></div><input type="checkbox" class="toggle" data-a="cSet" data-f="build" ${md.build ? "checked" : ""}></label>
      ${md.client !== "other" ? `<label class="setrow" style="cursor:pointer"><div class="l"><b>Install mcpDeck skills</b><span class="small muted">4 skills that teach it to pick tools, ask for missing ones and report the fields it uses. ${md.client === "claude-code" ? "Into ~/.claude/skills." : md.client === "vscode" ? "As instructions files in .github/instructions." : md.client === "cursor" ? "As rules in .cursor/rules." : "Claude Desktop doesn't read skill files; skipped."}</span></div><input type="checkbox" class="toggle" data-a="cSet" data-f="skills" ${md.skills && md.client !== "claude-desktop" ? "checked" : ""} ${md.client === "claude-desktop" ? "disabled" : ""}></label>` : ""}
    </div>`,
    `<button type="button" class="btn light" data-a="close">Cancel</button><button type="button" class="btn dark" data-a="cCreate">Create token and continue</button>`);
  const url = `${HOST}/mcp/${md.tk}`, hdr = `Authorization: Bearer ${md.token}`, srv = md.tk;
  const ways = {
    "claude-code": [
      ["Run in your terminal", "Adds it for every project (user scope). Then start a new Claude Code session.", codeBlock(`claude mcp add --transport http --scope user ${srv} ${url} --header "${hdr}"`, "Terminal", "Command copied. Run it in your terminal.")],
      ["Or let mcpDeck write the config", `Adds <code>${srv}</code> to <code>~/.claude.json</code> and keeps a backup. Other servers in the file stay as they are.`, `<button type="button" class="btn light sm" data-a="cWrote" data-msg="Wrote ~/.claude.json. Backup: ~/.claude.json.mcpdeck-backup">Write config</button>`],
    ],
    vscode: [
      ["Open in VS Code", "VS Code asks you to confirm, then adds it to your user settings.", `<a class="btn dark sm" href="vscode:mcp/install?${encodeURIComponent(JSON.stringify({ name: srv, type: "http", url, headers: { Authorization: "Bearer " + md.token } }))}" data-a="cWrote" data-msg="Opening VS Code">${ic("link", 13)}Open in VS Code</a>`],
      ["Or add it to a workspace", "Put this in <code>.vscode/mcp.json</code>. Don't commit the token.", codeBlock(JSON.stringify({ servers: { [srv]: { type: "http", url, headers: { Authorization: "Bearer " + md.token } } } }, null, 2), ".vscode/mcp.json", "Config copied")],
    ],
    cursor: [
      ["Open in Cursor", "Cursor shows an install prompt with the server filled in.", `<a class="btn dark sm" href="cursor://anysphere.cursor-deeplink/mcp/install?name=${srv}&config=${btoa(JSON.stringify({ url, headers: { Authorization: "Bearer " + md.token } }))}" data-a="cWrote" data-msg="Opening Cursor">${ic("link", 13)}Open in Cursor</a>`],
      ["Or edit the config", "Add this to <code>~/.cursor/mcp.json</code>, or let mcpDeck do it.", codeBlock(JSON.stringify({ mcpServers: { [srv]: { url, headers: { Authorization: "Bearer " + md.token } } } }, null, 2), "~/.cursor/mcp.json", "Config copied") + `<button type="button" class="btn light sm" style="margin-top:8px" data-a="cWrote" data-msg="Wrote ~/.cursor/mcp.json. Backup kept next to it.">Write config</button>`],
    ],
    "claude-desktop": [
      ["Let mcpDeck write the config", `Claude Desktop starts mcpDeck's stdio bridge. Adds <code>${srv}</code> to <code>${isMac ? "~/Library/Application Support/Claude/claude_desktop_config.json" : "%APPDATA%\\Claude\\claude_desktop_config.json"}</code> and keeps a backup. Restart Claude Desktop afterwards.`, `<button type="button" class="btn dark sm" data-a="cWrote" data-msg="Wrote claude_desktop_config.json. Restart Claude Desktop.">Write config</button>`],
      ["Or paste it yourself", "", codeBlock(JSON.stringify({ mcpServers: { [srv]: { command: "mcpdeck", args: ["stdio", "--toolkit", srv], env: { MCPDECK_TOKEN: md.token } } } }, null, 2), "claude_desktop_config.json", "Config copied")],
    ],
    other: [["Server URL", "Streamable HTTP.", copyField(url, "URL copied")], ["Header", "Send it with every request.", copyField(hdr, "Header copied")], ["stdio clients", "Run this command with MCPDECK_TOKEN set to the token.", copyField(`mcpdeck stdio --toolkit ${srv}`, "Command copied")]],
  }[md.client];
  const first = `Use the mcpDeck ${srv} tools to ${srv === "finance" ? "check whether account 001Ab00000XyZ has overdue invoices and is on credit hold" : "summarise account 001Ab00000XyZ: balance, overdue invoices and open tickets"}.`;
  return modalShell(`Add ${srv} to ${c.label}`, `${md.access === "read" ? "Read" : "Read and write"}${md.build ? ", may build tools" : ""}. ${md.reused ? `Uses the existing agent ${esc(md.name)}.` : `New agent: ${esc(md.name)}.`}`, `
    <div class="notice warn">${ic("key")}<div><b>This token is shown once.</b> It's in the command and config below. ${c.label} stores it in its config file in plain text, as every MCP client does. Revoke it any time on the Agents page.</div></div>
    ${ways.map(([t, d, x]) => `<div class="stack" style="gap:6px"><h3>${t}</h3>${d ? `<p class="small muted">${d}</p>` : ""}${x}</div>`).join("")}
    <div class="card row nw" style="gap:12px"><span class="dot ${md.connected ? "" : "wait"}"></span><div class="grow">${md.connected ? `<b>Connected.</b> <span class="small muted">${c.label} started a session and loaded ${plural(kit(srv).tools.filter((t) => tool(t)?.status === "live" && (md.access === "read_write" || tool(t).access === "read")).length, "tool")}${md.build ? " and the build tools" : ""}.</span>` : `<b>Waiting for ${c.label} to connect.</b> <span class="small muted">${md.client === "claude-code" ? "Start a new session, then run /mcp to see it." : "It shows here as soon as the client connects."}</span>`}</div></div>
    ${md.connected ? promptCard({ title: "Try it", text: first, note: `Paste into ${c.label}.` }) : ""}`,
    `<a class="btn light left" href="#agent-${md.agentId}" data-a="close">Open agent</a><button type="button" class="btn dark" data-a="close">Done</button>`, true);
}
function newToolkitModal(md) {
  return modalShell("New toolkit", "A toolkit is one MCP server. Agents you connect to it see its tools and nothing else.", `
    <label class="field">Name<input class="input mono" id="tk-name" autofocus value="${esc(md.name || "")}" placeholder="support" pattern="[a-z][a-z0-9-]*"><span class="hint">Its address will be <code>${HOST}/mcp/<span id="tk-prev">${esc(md.name || "support")}</span></code>.</span></label>
    <label class="field">What it's for<input class="input" id="tk-desc" value="${esc(md.desc || "")}" placeholder="Customer support: one customer's account, invoices and tickets."><span class="hint">Agents read this when they connect.</span></label>
    <div class="stack" style="gap:6px"><b style="font-size:13px">APIs its agents may build with</b><div class="setgroup">${S.apis.map((a) => `<label class="check"><input type="checkbox" name="tk-scope" value="${a.id}" checked><span><span class="t mono">${a.id}</span> <span class="small muted">${esc(a.title)} · ${a.requests.length} requests</span></span></label>`).join("")}</div></div>
    ${S.tools.length ? `<div class="stack" style="gap:6px"><b style="font-size:13px">Start with these tools</b><div class="setgroup" style="max-height:200px;overflow:auto">${S.tools.filter((t) => t.status === "live").map((t) => `<label class="check"><input type="checkbox" name="tk-tools" value="${t.id}"><span><span class="t mono">${t.id}</span> <span class="small muted">${esc(descOf(t.id))}</span></span></label>`).join("")}</div></div>` : `<p class="small muted">The toolkit starts empty. Add tools after, or let an agent build them.</p>`}
    ${md.error ? `<div class="notice bad">${ic("alert")}<div>${esc(md.error)}</div></div>` : ""}`,
    `<button type="button" class="btn light" data-a="close">Cancel</button><button type="button" class="btn dark" data-a="tkCreate">Create toolkit</button>`);
}
function newToolModal(md) {
  const k = kit(md.tk) || S.toolkits[0];
  const reqText = md.req ? S.requests.find((r) => r.id === md.req)?.text : "";
  const goal = md.goal ?? (reqText || "Overdue invoices for one account, with the billing contact to chase.");
  const scopeApis = S.apis.filter((a) => k.scope.includes(a.id));
  const b = builders(k.id);
  const promptText = `In the mcpDeck ${k.id} toolkit, build a read-only tool for this: ${goal.trim()} Check search_tools first so you don't duplicate one. Use search_apis and describe_api, return only the fields needed, test it with test_tool against saved examples, then save_tool${md.req ? ` with fulfils: ${md.req}` : ""}.`;
  const opts = [["agent", "Ask an agent", "Describe it; your agent builds and tests it"], ["requests", "From requests", "Pick requests; mcpDeck writes the code"], ["blank", "Write it myself", "Start from a one-call template"]];
  let body = "";
  if (md.how === "agent") body = `<label class="field">What should the tool answer?<textarea class="input" id="nt-goal" rows="2" style="font-family:var(--font);font-size:14px" data-a="ntGoal">${esc(goal)}</textarea></label>${promptCard({ title: "Prompt", text: promptText, note: b.length ? `Paste into ${b.map((a) => a.name).join(" or ")}. They may build in ${k.id}.` : `No agent may build in ${k.id}. Turn on "May build" for one, or connect one with it on.` })}`;
  if (md.how === "requests") body = `<p class="small muted">Tick the requests in the order the tool should call them. Values that flow from one response into the next are wired up; independent calls run together.</p><div class="setgroup" style="max-height:300px;overflow:auto">${scopeApis.map((a) => `<div class="kicker" style="padding:10px 0 2px">${a.id}</div>` + a.requests.filter((r) => r.access !== "check").map((r) => `<label class="check"><input type="checkbox" name="nt-req" value="${a.id}.${r.id}" ${(md.picked || []).includes(a.id + "." + r.id) ? "checked" : ""}>${m(r.method)}<span class="mono" style="font-size:12.5px">${r.id}</span><span class="small muted">${esc(r.path)}</span></label>`).join("")).join("") || `<p class="small muted" style="padding:12px 0">No APIs in ${k.id}'s build list.</p>`}</div>`;
  if (md.how === "requests" || md.how === "blank") body += `<label class="field">Tool name<input class="input mono" id="nt-name" value="${esc(md.name || "")}" placeholder="overdue_invoices"><span class="hint">Lowercase letters, digits and _. Agents see this name.</span></label>`;
  return modalShell(`New tool in ${k.id}`, "Tools are TypeScript functions over your API requests. Pick how to start.", `
    ${S.toolkits.length > 1 ? `<label class="field">Toolkit<select class="input" data-a="ntSet" data-f="tk">${S.toolkits.map((x) => `<option ${x.id === k.id ? "selected" : ""}>${x.id}</option>`).join("")}</select></label>` : ""}
    <div class="optgrid" role="radiogroup" aria-label="How to start">${opts.map(([v, t, d]) => `<button type="button" role="radio" aria-checked="${md.how === v}" class="opt ${md.how === v ? "on" : ""}" data-a="ntSet" data-f="how" data-v="${v}"><span class="t">${t}</span><span class="d">${d}</span></button>`).join("")}</div>${body}
    ${md.error ? `<div class="notice bad">${ic("alert")}<div>${esc(md.error)}</div></div>` : ""}`,
    md.how === "agent" ? `<button type="button" class="btn light" data-a="close">Close</button>${copyBtn(promptText, "Copy prompt", "Prompt copied. Paste it into your agent.", "dark")}` : `<button type="button" class="btn light" data-a="close">Cancel</button><button type="button" class="btn dark" data-a="ntCreate">Create and open editor</button>`);
}
function genTool(name, picks) {
  const calls = picks.map((p) => { const [a, r] = p.split("."); return { a, r: reqOf(a, r) }; });
  const first = calls[0];
  const inputs = (first.r.params || []).map(([k, v]) => [k === "id" ? "id" : k, v]);
  const argOf = (c) => c.r.params?.length ? `{ ${c.r.params.map(([k]) => `${k}: input.${k}`).join(", ")} }` : c.r.body ? "{ body: {} }" : "";
  const props = inputs.map(([k, v]) => `${k}: { type: "string", examples: ["${v}"] }`).join(", ");
  let code = `export const tool = {\n  name: "${name}",\n  description: "${calls.map((c) => c.r.id.replace(/([A-Z])/g, " $1").toLowerCase()).join(" and ").replace(/^./, (c) => c.toUpperCase())}, in one call.",\n  inputSchema: {\n    type: "object",\n    properties: { ${props} },\n    required: [${inputs.map(([k]) => `"${k}"`).join(", ")}],\n  },\n};\n\nexport default async function (input: Input) {\n`;
  if (calls.length === 1) code += `  const res = await ${first.a}.${first.r.id}(${argOf(first)});\n  return {\n    // Pick the fields the agent needs, for example:\n    // id: res.id,\n  };\n}\n`;
  else code += `  const [${calls.map((c, i) => "r" + (i + 1)).join(", ")}] = await Promise.all([\n${calls.map((c) => `    ${c.a}.${c.r.id}(${argOf(c)}),`).join("\n")}\n  ]);\n  return {\n${calls.map((c, i) => `    ${c.r.id}: r${i + 1},`).join("\n")}\n  };\n}\n`;
  return code;
}
function libraryModal(md) {
  const k = kit(md.tk); const avail = S.tools.filter((t) => !k.tools.includes(t.id) && t.status === "live");
  return modalShell(`Add tools to ${k.id}`, "Same tool, same versions: changes show up in every toolkit that has it.", avail.length ? `<div class="setgroup">${avail.map((t) => `<label class="check"><input type="checkbox" name="lib" value="${t.id}"><span><span class="t mono">${t.id}</span> ${accessChip(t.access)}<br><span class="small muted">${esc(descOf(t.id))}</span></span></label>`).join("")}</div>` : `<p class="muted">Every tool in the library is already in ${k.id}.</p>`,
    `<button type="button" class="btn light" data-a="close">Cancel</button><button type="button" class="btn dark" data-a="libAdd" ${avail.length ? "" : "disabled"}>Add</button>`);
}
const confirmModal = (md) => modalShell(md.title, "", `<p>${md.text}</p>`, `<button type="button" class="btn light" data-a="close">Cancel</button><button type="button" class="btn ${md.danger ? "danger" : "dark"}" data-a="confirmYes">${md.yes}</button>`);
const secretModal = (md) => modalShell(`${md.has ? "Replace" : "Add"} credential`, `${md.api} · ${md.env}`, `<label class="field">Value<input class="input mono" type="password" id="sec-val" autofocus autocomplete="off" placeholder="paste the key or token"><span class="hint">Encrypted when you save. It can't be shown again, only replaced. Tools pick it up on their next call.</span></label>`, `<button type="button" class="btn light" data-a="close">Cancel</button><button type="button" class="btn dark" data-a="secSave">Save</button>`);
function suggestionDrawer(md) {
  const s = S.suggestions.find((x) => x.id === md.id); if (!s) return "";
  const p = prompts({ kind: "suggestion", s })[0];
  return `<aside class="drawer" role="dialog" aria-modal="true" aria-label="Suggestion"><div class="modal-head"><div class="grow"><span class="kicker">Suggestion · ${s.rule}</span><h2 style="font-size:18px">${esc(s.title)}</h2></div><button type="button" class="iconbtn" data-a="close" aria-label="Close">${ic("x")}</button></div>
  <div class="modal-body"><p>${esc(s.why)}</p><div class="card stat"><div class="label">Expected saving</div><div style="font-weight:600;margin-top:2px">${esc(s.save)}</div></div>
    <h3>Evidence</h3><div class="stack" style="gap:6px">${s.evidence.map(([a, what, where]) => `<div class="row nw">${avatar(agent(a)?.client || a, 22)}<b class="small">${esc(agentName(a))}</b><span class="chip ${what === "used" ? "ok" : what === "blocked" ? "bad" : "neutral"}">${what}</span><span class="small muted">${esc(where)}</span></div>`).join("")}</div>
    ${s.diff ? `<h3>Change</h3>${diffHtml(s.diff)}<p class="small muted">Applying saves a new version of ${s.tool}. Agents on latest get it; pinned agents keep theirs.</p>` : ""}
    ${s.kind === "peragent" ? `<p class="small muted">Only Cursor's output changes. The tool, its tests and versions stay the same.</p>` : ""}
    ${p ? promptCard(p) : ""}</div>
  <div class="modal-foot"><button type="button" class="btn ghost left" data-a="dismiss" data-id="${s.id}">Dismiss</button>${s.kind === "blocked" ? `<a class="btn light" href="#agent-cursor" data-a="close">Change access</a>` : `<button type="button" class="btn blue" data-a="applySuggestion" data-id="${s.id}">${s.kind === "peragent" ? "Leave it out for Cursor" : "Save as new version"}</button>`}</div></aside>`;
}
function paletteItems() {
  const it = [];
  NAV.forEach(([id, l]) => it.push(["Go to", l, "#" + id])); it.push(["Go to", "Settings", "#preferences"]);
  it.push(["Action", "Add API", "act:addApi"], ["Action", "New toolkit", "act:newToolkit"], ["Action", "New tool", "act:newTool"], ["Action", "Connect agent", "act:connect"]);
  S.toolkits.forEach((k) => Object.entries(CLIENTS).slice(0, 4).forEach(([c, x]) => it.push(["Action", `Add ${k.id} to ${x.label}`, `connect:${k.id}:${c}`])));
  S.apis.forEach((a) => { it.push(["API", a.id, "#api-" + a.id]); a.requests.forEach((r) => it.push(["Request", `${a.id}.${r.id}`, `req:${a.id}:${r.id}`])); });
  S.toolkits.forEach((k) => it.push(["Toolkit", k.id, "#toolkit-" + k.id]));
  S.tools.forEach((t) => it.push(["Tool", t.id, (t.status === "pending" ? "#review-" : "#tool-") + t.id]));
  S.agents.forEach((a) => it.push(["Agent", a.name, "#agent-" + a.id]));
  it.push(["Action", "Switch theme", "act:theme"]);
  return it;
}
function paletteModal(md) {
  const q = (md.q || "").toLowerCase();
  const items = paletteItems().filter(([g, l]) => !q || (l + " " + g).toLowerCase().includes(q)).slice(0, 40);
  md.items = items; md.i = Math.min(md.i || 0, Math.max(0, items.length - 1));
  let lastG = "";
  return `<div class="palette" role="dialog" aria-modal="true" aria-label="Search or run"><input id="pal" autofocus placeholder="Search APIs, requests, tools, agents, or run an action" value="${esc(md.q || "")}" aria-label="Search"><div class="list" role="listbox">${items.map(([g, l, go], i) => { const h = g !== lastG ? `<div class="grp">${g}</div>` : ""; lastG = g; return h + `<div class="pi ${i === md.i ? "on" : ""}" role="option" data-a="palGo" data-i="${i}">${ic({ "Go to": "chev", Action: "play", API: "api", Request: "file", Toolkit: "layers", Tool: "code", Agent: "agent" }[g], 14)}<span>${esc(l)}</span><span class="k">${g}</span></div>`; }).join("") || `<p class="muted" style="padding:14px">Nothing matches.</p>`}</div></div>`;
}

/* ---------- ask popover ---------- */
function openAsk(btn, ctxStr) {
  closePop();
  const [kind, id] = ctxStr.split(":");
  const ctx = kind === "home" ? { kind } : kind === "api" ? { kind, api: api(id) } : kind === "toolkit" ? { kind, tk: kit(id) } : kind === "tool" ? { kind, tool: tool(id) } : { kind };
  const ps = prompts(ctx);
  const pop = document.createElement("div"); pop.className = "pop"; pop.id = "pop"; pop.setAttribute("role", "dialog"); pop.setAttribute("aria-label", "Prompts for your agent");
  pop.innerHTML = `<div class="row nw"><b class="grow">Prompts for your agent</b><button type="button" class="iconbtn" data-a="closePop" aria-label="Close">${ic("x", 14)}</button></div><p class="small muted">Copy one and paste it into Claude Code, VS Code, Cursor or Claude Desktop. They use real tool names from this page.</p>${ps.map(promptCard).join("")}`;
  document.body.append(pop);
  const r = btn.getBoundingClientRect(); const w = pop.offsetWidth;
  pop.style.top = r.bottom + window.scrollY + 8 + "px";
  pop.style.left = Math.max(16, Math.min(r.right - w, window.innerWidth - w - 16)) + window.scrollX + "px";
}
const closePop = () => $("#pop")?.remove();

/* ---------- actions ---------- */
const A = {
  copy: (el) => copy(CLIP.get(el.dataset.k), el.dataset.msg),
  tab: (el) => { S.ui.tabs[el.dataset.key] = el.dataset.v; if (!el.getAttribute("href")) render(); },
  mockstate: (el) => { const v = el.dataset.v; if (v === "demo") { S = { ...S, mode: "demo", ...demoWorld() }; } else { S = { ...S, mode: "new", ...emptyWorld() }; } S.ui = { ...S.ui, tabs: {}, open: {}, sel: {}, drafts: {}, runs: {}, filters: { agent: "", tk: "", kind: "all", session: "" } }; closeModal(); location.hash = "#home"; render(); },
  palette: () => openModal({ type: "palette", q: "", i: 0 }),
  ask: (el) => openAsk(el, el.dataset.ctx),
  closePop,
  close: (el) => { if (el.dataset.tab && S.modal?.target) S.ui.tabs["api-" + S.modal.target] = el.dataset.tab; closeModal(); render(); },
  scrim: (el, e) => { if (e.target === el) closeModal(); },
  addApi: (el) => openModal({ type: "addApi", step: 1, src: el.dataset.into ? "openapi" : "swagger", into: el.dataset.into || null }),
  impSrc: (el) => { S.modal.value = $("#imp-text")?.value || ""; S.modal.name = $("#imp-name")?.value || S.modal.name; S.modal.src = el.dataset.v; S.modal.error = null; renderModal(); },
  doImport,
  newRequest: (el) => { const a = api(el.dataset.api); let n = 1; while (a.requests.some((r) => r.id === "newRequest" + n)) n++; const r = R("newRequest" + n, "requests", "GET", "/", "read", {}); a.requests.push(r); (S.ui.open[a.id] ||= []).push(r.id); S.ui.sel[a.id] = r.id; S.ui.tabs["api-" + a.id] = "requests"; S.ui.tabs["tree-" + a.id] = "requests"; render(); toast(`Added ${r.id}. Set its method and URL, then Send.`); },
  openReq: (el) => { const o = (S.ui.open[el.dataset.api] ||= []); if (!o.includes(el.dataset.id)) o.push(el.dataset.id); S.ui.sel[el.dataset.api] = el.dataset.id; render(); },
  closeReq: (el, e) => { e.stopPropagation(); const o = S.ui.open[el.dataset.api]; o.splice(o.indexOf(el.dataset.id), 1); render(); },
  send: (el) => {
    const a = api(el.dataset.api), r = reqOf(a.id, el.dataset.id);
    el.disabled = true; el.innerHTML = "Sending…";
    setTimeout(() => {
      const missing = !a.creds?.[S.ui.env[a.id] || a.envs[0]] && a.authType !== "None";
      const body = missing ? { error: `No credential for ${a.id} in ${S.ui.env[a.id] || a.envs[0]}. Add it on the Auth and credentials tab.` } : RESP[r.resp] ?? { message: "This mockup only has responses for the demo world." };
      const status = missing ? 401 : r.resp === "login" ? 302 : r.resp === "created" ? 201 : RESP[r.resp] !== undefined ? 200 : 501;
      const ms = 30 + ((r.id.length * 13) % 90);
      S.ui.runs["req:" + a.id + r.id] = { status, ms, body };
      S.history.unshift({ api: a.id, req: r.id, status, ms, when: "now", who: "you" });
      render();
    }, 350);
  },
  saveExample: (el) => { const r = reqOf(el.dataset.api, el.dataset.id); r.example = true; toast(`Saved as an example of ${r.id}. Fields that look like secrets or personal data were masked.`); render(); },
  setAccess: (el) => { const r = reqOf(el.dataset.api, el.dataset.id); r.access = el.dataset.v; r.override = r.method !== "GET" && el.dataset.v === "read" ? `${r.method}, marked read by you` : r.method === "GET" && el.dataset.v === "write" ? "GET, marked write by you" : null; S.audit.unshift(["now", "you", "Changed access", `${el.dataset.api}.${r.id}`, `→ ${el.dataset.v}`]); render(); toast(`${r.id} is now ${el.dataset.v}`); },
  env: () => {},
  replaceSecret: (el) => openModal({ type: "secret", api: el.dataset.api, env: el.dataset.env || api(el.dataset.api).envs[0], has: !!api(el.dataset.api).creds?.[el.dataset.env] }),
  secSave: () => { const v = $("#sec-val").value; if (!v) return toast("Paste the value first"); const a = api(S.modal.api); (a.creds ||= {})[S.modal.env] = "just now"; S.audit.unshift(["now", "you", "Replaced credential", `${a.id} · ${S.modal.env}`, "value not recorded"]); closeModal(); render(); toast("Saved and encrypted. The next call uses it."); },
  addVar: (el) => { const a = api(el.dataset.api); let n = 1; while (a.vars.some((v) => v[0] === "var" + n)) n++; a.vars.push(["var" + n, "", "", "text"]); render(); },
  addHost: (el) => toast("Type a host like api.example.com and press Enter"),
  authType: (el) => { api(el.dataset.api).authType = el.value; render(); },
  trust: (el) => { const a = api(el.dataset.api); a.scripts.trusted = el.checked; toast(el.checked ? `Scripts in ${a.id} will run` : `Scripts in ${a.id} won't run`); },
  deleteApi: (el) => openModal({ type: "confirm", title: `Delete ${el.dataset.api}?`, text: "Its requests, environments, examples and credentials are deleted. This can't be undone.", yes: "Delete API", danger: true, run: () => { S.apis = S.apis.filter((a) => a.id !== el.dataset.api); location.hash = "#apis"; } }),
  confirmYes: () => { const f = S.modal.run; closeModal(); f(); render(); },
  histsel: (el) => { const k = "hist-" + el.dataset.api; const s = (S.ui.sel[k] ||= []); const i = +el.dataset.i; if (el.checked) s.push(i); else s.splice(s.indexOf(i), 1); render(); },
  toolFromHistory: (el) => { const hs = (S.ui.sel["hist-" + el.dataset.api] || []).map((i) => S.history.filter((h) => h.api === el.dataset.api)[i]); const picks = [...new Set(hs.map((h) => `${h.api}.${h.req}`))]; openModal({ type: "newTool", how: "requests", tk: S.toolkits.find((k) => k.scope.includes(el.dataset.api))?.id || S.toolkits[0]?.id, picked: picks }); if (!S.toolkits.length) { closeModal(); toast("Create a toolkit first; tools are added to one."); } },
  toolFromReq: (el) => { if (!S.toolkits.length) return toast("Create a toolkit first; tools are added to one."); openModal({ type: "newTool", how: "requests", tk: S.toolkits.find((k) => k.scope.includes(el.dataset.api))?.id || S.toolkits[0].id, picked: [`${el.dataset.api}.${el.dataset.id}`] }); },
  newToolkit: () => openModal({ type: "newToolkit" }),
  tkCreate: () => {
    const md = S.modal; md.name = $("#tk-name").value.trim(); md.desc = $("#tk-desc").value.trim();
    if (!/^[a-z][a-z0-9-]{1,30}$/.test(md.name)) { md.error = "Use lowercase letters, digits and -, starting with a letter. It becomes part of the URL."; return renderModal(); }
    if (kit(md.name)) { md.error = `There's already a toolkit named ${md.name}.`; return renderModal(); }
    if (!md.desc) { md.error = "Say what it's for in one line. Agents read it."; return renderModal(); }
    S.toolkits.push({ id: md.name, desc: md.desc, tools: $$("[name=tk-tools]:checked").map((x) => x.value), scope: $$("[name=tk-scope]:checked").map((x) => x.value), hints: true, limit: 2000, rules: { tests: true, desc: true, output: true, dup: true }, vars: [] });
    S.audit.unshift(["now", "you", "Created toolkit", md.name, ""]);
    closeModal(); location.hash = "#toolkit-" + md.name; toast(`Created ${md.name}. Connect an agent to use it.`);
  },
  connect: (el) => {
    const tk = el.dataset.tk || (el.dataset.agent ? S.toolkits.find((k) => !agent(el.dataset.agent).grants.some((g) => g.tk === k.id))?.id : null) || S.toolkits[0]?.id;
    if (!tk) return toast("Create a toolkit first. Agents connect to toolkits.");
    const client = el.dataset.client || (el.dataset.agent ? agent(el.dataset.agent).client : "claude-code");
    openModal({ type: "connect", step: 1, client, tk, access: "read", build: S.mode === "new", skills: true, agentId: el.dataset.agent || null });
  },
  cSet: (el) => { const md = S.modal; const f = el.dataset.f; md.name = $("#c-name")?.value ?? md.name; if (f === "client") { md.client = el.dataset.v; md.name = undefined; } else if (f === "tk") md.tk = el.value; else if (f === "build" || f === "skills") md[f] = el.checked; else md[f] = el.dataset.v; renderModal(); },
  cCreate: () => {
    const md = S.modal; md.name = ($("#c-name")?.value || "").trim() || CLIENTS[md.client].label;
    let a = md.agentId ? agent(md.agentId) : S.agents.find((x) => x.client === md.client && !x.revoked);
    md.reused = !!a;
    if (!a) { a = { id: md.client + (S.agents.some((x) => x.id === md.client) ? "-" + (S.agents.length + 1) : ""), name: md.name, client: md.client, since: "today", where: { "claude-code": "~/.claude.json", vscode: "VS Code user settings", cursor: "~/.cursor/mcp.json", "claude-desktop": "claude_desktop_config.json", other: "your client's config" }[md.client], live: false, lastSeen: "never", today: 0, errors: 0, blocked: 0, grants: [], drops: [] }; S.agents.push(a); }
    else md.name = a.name;
    md.token = token(); a.token = md.token.slice(0, 5) + "…" + md.token.slice(-4);
    const g = a.grants.find((x) => x.tk === md.tk);
    if (g) Object.assign(g, { access: md.access, build: md.build }); else a.grants.push({ tk: md.tk, access: md.access, build: md.build, env: api(kit(md.tk).scope[0])?.envs[0] || "Local", pin: "latest" });
    md.agentId = a.id; md.step = 2;
    S.audit.unshift(["now", "you", "Connected agent", a.name, `${md.tk} · ${md.access === "read" ? "read" : "read+write"}${md.build ? " · may build" : ""}`]);
    renderModal(); route(false);
  },
  cWrote: (el) => { toast(el.dataset.msg); const md = S.modal; clearTimeout(S.connectTimer); S.connectTimer = setTimeout(() => { if (S.modal !== md) return; md.connected = true; const a = agent(md.agentId); a.live = true; renderModal(); route(false); }, 2500); },
  newTool: (el) => { if (!S.toolkits.length) return toast("Create a toolkit first; tools are added to one."); openModal({ type: "newTool", how: el.dataset.how || "agent", tk: el.dataset.tk || S.toolkits[0].id, req: el.dataset.req ? +el.dataset.req : null }); },
  ntSet: (el) => { const md = S.modal; md.goal = $("#nt-goal")?.value ?? md.goal; md.name = $("#nt-name")?.value ?? md.name; md.picked = $$("[name=nt-req]:checked").map((x) => x.value).length ? $$("[name=nt-req]:checked").map((x) => x.value) : md.picked; if (el.dataset.f === "tk") md.tk = el.value; else md[el.dataset.f] = el.dataset.v; md.error = null; renderModal(); },
  ntGoal: () => {},
  ntCreate: () => {
    const md = S.modal; const name = ($("#nt-name")?.value || "").trim(); md.name = name;
    if (!/^[a-z][a-z0-9_]{2,63}$/.test(name)) { md.error = "Name the tool with 3 to 64 lowercase letters, digits and _, like overdue_invoices."; return renderModal(); }
    if (tool(name)) { md.error = `There's already a tool named ${name}.`; return renderModal(); }
    const picks = md.how === "requests" ? $$("[name=nt-req]:checked").map((x) => x.value) : [];
    if (md.how === "requests" && !picks.length) { md.error = "Tick at least one request."; return renderModal(); }
    const k = kit(md.tk);
    const firstApi = picks[0]?.split(".")[0] || k.scope[0];
    const r0 = api(firstApi)?.requests.find((r) => r.access === "read") || api(firstApi)?.requests[0];
    if (!r0) { md.error = `Add an API with at least one request to ${k.id}'s build list first (toolkit Settings).`; return renderModal(); }
    const code = md.how === "requests" ? genTool(name, picks) : genTool(name, [`${firstApi}.${r0.id}`]);
    S.ui.drafts[name + "@saved"] = code; S.ui.drafts[name] = code;
    S.tools.push({ id: name, apis: [...new Set((picks.length ? picks : [firstApi + "."]).map((p) => p.split(".")[0]))], access: picks.some((p) => reqOf(...p.split("."))?.access !== "read") ? "write" : "read", version: 1, by: "you", calls7: 0, err: 0, p50: 0, p95: 0, versions: [{ v: 1, by: "you", when: "just now", note: "First version", status: "live" }], tests: [], status: "live" });
    k.tools.push(name); if (md.req) { const rq = S.requests.find((r) => r.id === md.req); if (rq) rq.state = "fulfilled"; }
    S.audit.unshift(["now", "you", "Saved tool version", `${name} v1`, `in ${k.id}`]);
    closeModal(); location.hash = "#tool-" + name; toast(`Created ${name} v1 in ${k.id}. Edit, test, then save.`);
  },
  addFromLibrary: (el) => openModal({ type: "library", tk: el.dataset.tk }),
  libAdd: () => { const k = kit(S.modal.tk); $$("[name=lib]:checked").forEach((x) => k.tools.push(x.value)); closeModal(); render(); toast(`Added to ${k.id}. Connected agents get the new list now.`); },
  grant: (el) => { const a = agent(el.dataset.agent), g = a.grants.find((x) => x.tk === el.dataset.tk); if (el.dataset.f === "build") g.build = el.checked; else g.access = el.dataset.v; S.audit.unshift(["now", "you", "Changed access", `${a.name} → ${g.tk}`, `${g.access === "read" ? "read" : "read+write"}${g.build ? ", may build" : ""}`]); render(); toast(`${a.name}: ${g.access === "read" ? "read" : "read and write"}${g.build ? ", may build" : ""} on ${g.tk}. Applies to its next call.`); },
  removeGrant: (el) => { const a = agent(el.dataset.agent); openModal({ type: "confirm", title: `Remove ${el.dataset.tk} from ${a.name}?`, text: `${a.name} loses those tools on its next call. Its calls stay in Activity.`, yes: "Remove", danger: true, run: () => { a.grants = a.grants.filter((g) => g.tk !== el.dataset.tk); } }); },
  rule: (el) => { kit(el.dataset.tk).rules[el.dataset.k] = el.checked; toast(el.checked ? "Rule on" : "Rule off. Versions saved by agents skip it."); },
  scope: (el) => { const k = kit(el.dataset.tk); if (el.checked) k.scope.push(el.dataset.api); else k.scope = k.scope.filter((x) => x !== el.dataset.api); toast(`${el.dataset.api} ${el.checked ? "added to" : "removed from"} ${k.id}'s build list`); },
  hints: (el) => { kit(el.dataset.tk).hints = el.checked; },
  closeRequest: (el) => { S.requests.find((r) => r.id === +el.dataset.id).state = "closed"; render(); toast("Request closed"); },
  insertCall: (el) => { const ta = $("#code"); if (!ta) return; const r = reqOf(el.dataset.api, el.dataset.id); const line = `  const ${r.id.replace(/^(get|list|search)/, "").replace(/^./, (c) => c.toLowerCase()) || "res"} = await ${el.dataset.api}.${r.id}(${r.params?.length ? `{ ${r.params[0][0]}: ${JSON.stringify(r.params[0][1])} }` : r.body ? "{ body: {} }" : ""});\n`; const pos = ta.value.lastIndexOf("  return"); ta.focus(); ta.setRangeText(line, pos > 0 ? pos : ta.selectionStart, pos > 0 ? pos : ta.selectionStart, "end"); ta.dispatchEvent(new Event("input")); },
  runTool: (el) => {
    const id = el.dataset.id, t = tool(id); const code = S.ui.drafts[id] ?? src(id);
    const ins = inputsOf(code); const inp = {}; ins.forEach((i) => { const v = $("#in-" + i.name)?.value ?? i.example; inp[i.name] = i.type === "integer" ? +v : v; S.ui.runs["in:" + id + i.name] = v; });
    if (tab("tool-" + id, "code") !== "code") S.ui.tabs["tool-" + id] = "code";
    let res;
    try { res = RUN[id] ? RUN[id](inp) : genericRun(code); } catch (e) { res = { error: e.msg || String(e) }; }
    S.ui.runs["tool:" + id] = res; render();
  },
  saveTool: (el) => { const id = el.dataset.id, t = tool(id); const code = S.ui.drafts[id]; const saved = S.ui.drafts[id + "@saved"] || src(id); if (code === undefined || code === saved) return toast("No changes to save"); if (/\bfetch\s*\(/.test(code) || !/export\s+default\s+async\s+function/.test(code)) return toast("Fix the problems under the editor first"); t.version++; t.versions.unshift({ v: t.version, by: "you", when: "just now", note: "Edited in mcpDeck", status: "live" }); S.ui.drafts[id + "@saved"] = code; S.ui.drafts[`${id}@v${t.version}`] = code; S.audit.unshift(["now", "you", "Saved tool version", `${id} v${t.version}`, "live"]); render(); const n = S.agents.filter((a) => a.grants.some((g) => kit(g.tk)?.tools.includes(id))).length; toast(`Saved ${id} v${t.version}. Live for ${plural(n, "agent")} on latest.`); },
  runTests: (el) => { const t = tool(el.dataset.id); t.tests.forEach((x) => (x.pass = null)); render(); setTimeout(() => { t.tests.forEach((x) => (x.pass = true)); render(); toast(`${t.tests.length} of ${t.tests.length} tests passed`); }, 600); },
  saveTest: (el) => { const t = tool(el.dataset.id); const ins = inputsOf(S.ui.drafts[t.id] ?? src(t.id)); t.tests.push({ name: `returns the same for ${ins.map((i) => S.ui.runs["in:" + t.id + i.name] ?? i.example).join(", ")}`, pass: true }); render(); toast("Saved as a test. It runs on every save."); },
  saveTestFromCall: (el) => { const c = S.calls.find((x) => x.id === el.dataset.id); const t = tool(c.tool); t.tests.push({ name: `recorded call ${c.t} from ${agentName(c.agent)}`, pass: true }); toast(`Saved as a test on ${c.tool}`); },
  rollback: (el) => { const t = tool(el.dataset.id), v = +el.dataset.v; const code = sourceAt(t.id, v); t.version++; t.versions.unshift({ v: t.version, by: "you", when: "just now", note: `Rolled back to v${v}`, status: "live" }); S.ui.drafts[t.id + "@saved"] = code; S.ui.drafts[t.id] = code; S.ui.drafts[`${t.id}@v${t.version}`] = code; render(); toast(`Saved v${t.version} with the code from v${v}`); },
  vsel: (el) => { S.ui.sel[el.dataset.k] = +el.value; render(); },
  dropField: (el) => { if (!el.value) return; const a = agent(el.value); a.drops.push({ tool: el.dataset.id, path: el.dataset.path }); S.audit.unshift(["now", "you", "Trimmed output", `${el.dataset.id} for ${a.name}`, el.dataset.path]); render(); toast(`${a.name} no longer gets ${el.dataset.path} from ${el.dataset.id}`); },
  undrop: (el) => { agent(el.dataset.agent).drops.splice(+el.dataset.i, 1); render(); },
  approve: (el) => { const t = tool(el.dataset.id); t.status = "live"; t.versions[0].status = "live"; t.versions[0].rules = "approved by you"; S.audit.unshift(["now", "you", "Approved", `${t.id} v${t.version}`, "write tool"]); const k = S.toolkits.find((x) => x.tools.includes(t.id)); const n = grantsFor(k.id).filter((g) => g.access === "read_write").length; location.hash = "#tool-" + t.id; toast(`${t.id} is live for ${plural(n, "agent")} with read and write on ${k.id}`); },
  reject: (el) => { const t = tool(el.dataset.id); openModal({ type: "confirm", title: `Reject ${t.id} v${t.version}?`, text: `The version stays in history as rejected and no agent gets it. Tell ${agentName(t.versions[0].by)} why, so its next version is better.`, yes: "Reject", danger: true, run: () => { t.status = "rejected"; t.versions[0].status = "rejected"; S.audit.unshift(["now", "you", "Rejected", `${t.id} v${t.version}`, ""]); location.hash = "#home"; } }); },
  rotate: (el) => { const a = agent(el.dataset.agent); const g = a.grants[0]; openModal({ type: "connect", step: 1, client: a.client, tk: g?.tk || S.toolkits[0].id, access: g?.access || "read", build: !!g?.build, skills: false, agentId: a.id, name: a.name }); toast("The old token stops working when you create the new one"); },
  revoke: (el) => { const a = agent(el.dataset.agent); openModal({ type: "confirm", title: `Revoke ${a.name}?`, text: "Its token stops working now and open sessions end within a second. Its calls stay in Activity.", yes: "Revoke", danger: true, run: () => { a.revoked = true; a.live = false; S.audit.unshift(["now", "you", "Revoked agent", a.name, ""]); toast(`${a.name} revoked`); } }); },
  pickcall: (el) => { S.ui.sel.call = el.dataset.id; S.ui.tabs.activity = "calls"; if (location.hash !== "#activity") location.hash = "#activity"; else render(); },
  session: (el) => { S.ui.filters.session = el.dataset.id; S.ui.tabs.activity = "calls"; S.ui.sel.call = null; if (location.hash !== "#activity") location.hash = "#activity"; else render(); },
  filter: (el) => { S.ui.filters[el.dataset.k] = el.dataset.v ?? el.value; S.ui.sel.call = null; render(); },
  usage: (el) => { S.ui.usage[el.dataset.k] = el.dataset.v; render(); },
  verify: () => toast(`Chain intact: ${S.audit.length} entries, each hash matches the one before`),
  suggestion: (el) => openModal({ type: "suggestion", id: el.dataset.id }),
  dismiss: (el) => { S.suggestions = S.suggestions.filter((s) => s.id !== el.dataset.id); closeModal(); render(); toast("Dismissed. It won't come back for this evidence."); },
  applySuggestion: (el) => {
    const s = S.suggestions.find((x) => x.id === el.dataset.id);
    if (s.kind === "peragent") { agent(s.agent).drops.push({ tool: s.tool, path: "openTickets[].title" }); toast("Cursor no longer gets openTickets[].title. Others are unchanged."); }
    else { const t = tool(s.tool); let code = S.ui.drafts[t.id + "@saved"] || src(t.id); code = s.kind === "fields" ? code.replace("        description: t.fields.description,\n", "") : code.replace(/  const invoices = await (.*);\n  const contacts = await (.*);\n/, (x, a, b) => `  const [invoices, contacts] = await Promise.all([\n    ${a.replace(/;$/, "")},\n    ${b.replace(/;$/, "")},\n  ]);\n`); t.version++; t.versions.unshift({ v: t.version, by: "you", when: "just now", note: s.title, status: "live" }); S.ui.drafts[t.id + "@saved"] = code; S.ui.drafts[t.id] = code; S.ui.drafts[`${t.id}@v${t.version}`] = code; toast(`Saved ${t.id} v${t.version}. Tests passed.`); }
    S.audit.unshift(["now", "you", "Accepted suggestion", s.rule, s.title]);
    S.suggestions = S.suggestions.filter((x) => x !== s); closeModal(); render();
  },
  palGo: (el) => runPalette(+el.dataset.i),
  theme: (el) => setTheme(el.dataset.v),
  toastOnly: (el) => toast(el.dataset.msg),
};
function genericRun(code) {
  const calls = [...code.matchAll(/\b(\w+)\.(\w+)\(/g)].map(([, a, r]) => reqOf(a, r) && [a + "." + r, reqOf(a, r).method, reqOf(a, r).path, 200, 40, 0, RESP[reqOf(a, r).resp] ?? {}]).filter(Boolean);
  if (!calls.length) return { error: "The tool made no API calls. Call one of your APIs, like crm.getAccount({ id })." };
  calls.forEach((c, i) => (c[5] = i ? calls[0][4] : 0));
  return { calls, out: /return\s*\{\s*(\/\/[^\n]*\n\s*)*\}/.test(code) ? {} : calls[calls.length - 1][6] };
}
function runPalette(i) {
  const it = S.modal.items[i]; if (!it) return; const go = it[2]; closeModal();
  if (go.startsWith("#")) location.hash = go;
  else if (go.startsWith("act:")) { const a = go.slice(4); if (a === "theme") setTheme(document.documentElement.dataset.theme === "dark" ? "light" : "dark"); else A[a]({ dataset: {} }); }
  else if (go.startsWith("connect:")) { const [, tk, c] = go.split(":"); A.connect({ dataset: { tk, client: c } }); }
  else if (go.startsWith("req:")) { const [, a, r] = go.split(":"); (S.ui.open[a] ||= []).includes(r) || S.ui.open[a].push(r); S.ui.sel[a] = r; S.ui.tabs["api-" + a] = "requests"; location.hash = "#api-" + a; render(); }
}
function setTheme(v) { S.theme = v; if (v === "system") delete document.documentElement.dataset.theme; else document.documentElement.dataset.theme = v; try { localStorage.setItem("mcpdeck-mock-theme", v); } catch {} render(); }

/* ---------- routing and events ---------- */
function render() {
  const r = (location.hash || "#home").slice(1);
  const [page, ...rest] = r.split("-"); const id = rest.join("-");
  renderSide(r);
  $$("#mock-state button").forEach((b) => b.classList.toggle("on", b.dataset.v === S.mode));
  const fn = PAGES[page] || PAGES.home;
  const y = window.scrollY;
  $("#main").innerHTML = fn(id);
  window.scrollTo(0, y);
  mountEditor();
  document.title = `${page === "home" || !PAGES[page] ? "mcpDeck UI Mockups" : (id || page[0].toUpperCase() + page.slice(1)) + " · mcpDeck UI Mockups"}`;
}
function route(scrollTop = true) {
  closePop();
  if (scrollTop) { const h = location.hash; if (h === "#activity") S.ui.tabs.activity = "calls"; if (h === "#toolkits") S.ui.tabs.toolkits = "kits"; }
  render(); if (scrollTop) window.scrollTo(0, 0); }
window.addEventListener("hashchange", () => route(true));
document.addEventListener("click", (e) => {
  if (!e.target.closest("#pop") && !e.target.closest("[data-a=ask]")) closePop();
  const el = e.target.closest("[data-a]");
  const go = e.target.closest("[data-go]");
  if (el && A[el.dataset.a] && el.tagName !== "SELECT" && el.type !== "checkbox") { A[el.dataset.a](el, e); return; }
  if (go && !e.target.closest("a,button,input,select,label")) location.hash = go.dataset.go;
});
document.addEventListener("change", (e) => {
  const el = e.target.closest("[data-a]"); if (!el || !A[el.dataset.a]) return;
  if (el.tagName === "SELECT" || el.type === "checkbox") A[el.dataset.a](el, e);
  if (el.dataset.a === "env") { S.ui.env[el.dataset.api] = el.value; render(); }
});
document.addEventListener("input", (e) => {
  const f = e.target.closest("[data-filter]");
  if (f) { const q = f.value.toLowerCase(); $$(f.dataset.filter, $("#main")).forEach((x) => (x.hidden = q && !(x.dataset.text || x.textContent).toLowerCase().includes(q))); }
  if (e.target.id === "imp-name") { const p = $("#imp-preview"); if (p) p.textContent = (e.target.value || "billing") + ".getInvoice()"; }
  if (e.target.id === "tk-name") { const p = $("#tk-prev"); if (p) p.textContent = e.target.value || "support"; }
  if (e.target.id === "nt-goal") { S.modal.goal = e.target.value; const c = e.target.selectionStart; renderModal(); const t = $("#nt-goal"); t.focus(); t.setSelectionRange(c, c); }
  if (e.target.id === "pal") { S.modal.q = e.target.value; S.modal.i = 0; const c = e.target.selectionStart; renderModal(); const t = $("#pal"); t.focus(); t.setSelectionRange(c, c); }
});
document.addEventListener("keydown", (e) => {
  const mod = isMac ? e.metaKey : e.ctrlKey;
  if (mod && e.key.toLowerCase() === "k") { e.preventDefault(); S.modal?.type === "palette" ? closeModal() : A.palette(); return; }
  if (S.modal?.type === "palette") {
    if (e.key === "ArrowDown" || e.key === "ArrowUp") { e.preventDefault(); S.modal.i = (S.modal.i + (e.key === "ArrowDown" ? 1 : S.modal.items.length - 1)) % Math.max(1, S.modal.items.length); const q = $("#pal").value; renderModal(); $("#pal").value = q; $(".pi.on")?.scrollIntoView({ block: "nearest" }); return; }
    if (e.key === "Enter") { e.preventDefault(); runPalette(S.modal.i); return; }
  }
  if (e.key === "Escape") { if (S.modal) closeModal(); closePop(); return; }
  const r = location.hash.slice(1);
  if (mod && e.key === "Enter") { if (r.startsWith("tool-")) { e.preventDefault(); A.runTool({ dataset: { id: r.slice(5) } }); } else if (r.startsWith("api-")) { const b = $("[data-a=send]"); if (b) { e.preventDefault(); b.click(); } } }
  if (mod && e.key.toLowerCase() === "s" && r.startsWith("tool-")) { e.preventDefault(); A.saveTool({ dataset: { id: r.slice(5) } }); }
});
try { const t = localStorage.getItem("mcpdeck-mock-theme"); if (t && t !== "system") { S.theme = t; document.documentElement.dataset.theme = t; } } catch {}
render();
