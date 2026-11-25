package mock

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/abhijeet-oxide/mcpdeck/internal/audit"
	"github.com/abhijeet-oxide/mcpdeck/internal/deck"
	"github.com/abhijeet-oxide/mcpdeck/internal/store"
)

//go:embed demo/*.json
var demoFiles embed.FS

// DevToken is the fixed token of the dev agent (MOCK-4), as in .mcp.json.
const DevToken = "mcpdeck-dev"

func crmSpec(base string) string {
	return `{"openapi":"3.0.3","info":{"title":"Acme CRM","version":"1.4.0","description":"Accounts, contacts and owners."},
"servers":[{"url":"` + base + `"}],
"components":{"securitySchemes":{"bearer":{"type":"http","scheme":"bearer"}}},"security":[{"bearer":[]}],
"paths":{
 "/accounts":{"get":{"operationId":"searchAccounts","summary":"Search accounts by name","tags":["accounts"],"parameters":[{"name":"q","in":"query","schema":{"type":"string","example":"north"}}],"responses":{"200":{"description":"Accounts"}}}},
 "/accounts/{id}":{"get":{"operationId":"getAccount","summary":"One account","tags":["accounts"],"parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"string"}}],"responses":{"200":{"description":"One account"}}},
  "patch":{"operationId":"updateAccount","summary":"Update an account","tags":["accounts"],"parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"string"}}],"requestBody":{"content":{"application/json":{"example":{"tier":"gold"}}}},"responses":{"200":{"description":"Updated"}}}},
 "/accounts/{id}/contacts":{"get":{"operationId":"listContacts","summary":"Contacts of an account","tags":["contacts"],"parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"string"}}],"responses":{"200":{"description":"Contacts"}}}},
 "/owners/{id}":{"get":{"operationId":"getOwner","summary":"Account owner","tags":["owners"],"parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"string"}}],"responses":{"200":{"description":"Owner"}}}}}}`
}

func billingSpec(base string) string {
	return `{"openapi":"3.0.3","info":{"title":"Acme Billing","version":"2.0.0","description":"Customer balances and invoices."},
"servers":[{"url":"` + base + `"}],
"components":{"securitySchemes":{"key":{"type":"apiKey","in":"header","name":"X-Api-Key"}}},"security":[{"key":[]}],
"paths":{
 "/customers/{id}/summary":{"get":{"operationId":"getCustomerSummary","summary":"Balance and invoices of a customer","tags":["customers"],"parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"string"}}],"responses":{"200":{"description":"Summary"}}}},
 "/invoices":{"get":{"operationId":"listInvoices","summary":"Invoices, filtered","tags":["invoices"],"parameters":[{"name":"customerId","in":"query","schema":{"type":"string"}},{"name":"status","in":"query","schema":{"type":"string","enum":["open","overdue","paid"]}}],"responses":{"200":{"description":"Invoices"}}}},
 "/invoices/{number}":{"get":{"operationId":"getInvoice","summary":"One invoice","tags":["invoices"],"parameters":[{"name":"number","in":"path","required":true,"schema":{"type":"string"}}],"responses":{"200":{"description":"Invoice"}}}},
 "/invoices/{number}/remind":{"post":{"operationId":"remindInvoice","summary":"Send a payment reminder","tags":["invoices"],"parameters":[{"name":"number","in":"path","required":true,"schema":{"type":"string"}}],"responses":{"202":{"description":"Queued"}}}}}}`
}

const customer360 = `export const tool = {
  name: "customer_360",
  description: "One view of a customer: account, tier, balance, overdue invoices and open tickets.",
  useWhen: "A question is about one customer or account",
  dontUseWhen: "Searching across many customers",
  inputSchema: {
    type: "object",
    properties: { accountId: { type: "string", examples: ["001Ab00000NWTRD"] } },
    required: ["accountId"],
  },
};

export default async function (input: Input) {
  const account = await crm.getAccount({ id: input.accountId });
  const [summary, tickets] = await Promise.all([
    billing.getCustomerSummary({ id: account.billingId }),
    support.searchIssues({ jql: ` + "`" + `org = "${account.jiraOrg}" AND status != Done` + "`" + ` }),
  ]);
  return {
    name: account.name,
    tier: account.tier,
    balance: summary.balance,
    overdueInvoices: summary.invoices
      .filter((i) => i.overdue)
      .map((i) => ({ number: i.number, dueDate: i.dueDate, amount: i.amount - i.amountPaid })),
    openTickets: tickets.issues.map((t) => ({ key: t.key, title: t.fields.summary })),
  };
}
`

const openTickets = `export const tool = {
  name: "open_tickets_for_account",
  description: "Open support tickets for one account, newest first, with status, priority and assignee.",
  useWhen: "Someone asks what's open or in progress for a customer",
  inputSchema: {
    type: "object",
    properties: { accountId: { type: "string", examples: ["001Ab00000NWTRD"] } },
    required: ["accountId"],
  },
};

export default async function (input: Input) {
  const account = await crm.getAccount({ id: input.accountId });
  const res = await support.searchIssues({ jql: ` + "`" + `org = "${account.jiraOrg}" AND status != Done` + "`" + `, maxResults: 20 });
  return {
    account: account.name,
    tickets: res.issues.map((t) => ({
      key: t.key,
      title: t.fields.summary,
      status: t.fields.status.name,
      priority: t.fields.priority.name,
      assignee: t.fields.assignee?.emailAddress ?? null,
    })),
  };
}
`

const invoiceStatus = `export const tool = {
  name: "invoice_status",
  description: "Status of one invoice: amount, amount paid, due date and whether it is overdue.",
  inputSchema: {
    type: "object",
    properties: { number: { type: "string", examples: ["INV-2087"] } },
    required: ["number"],
  },
};

export default async function (input: Input) {
  const inv = await billing.getInvoice({ number: input.number });
  return { number: inv.number, amount: inv.amount, paid: inv.amountPaid, due: inv.dueDate, overdue: inv.status === "overdue" };
}
`

const reassignTicket = `export const tool = {
  name: "reassign_ticket",
  description: "Assign a support ticket to another person and add a comment saying why.",
  inputSchema: {
    type: "object",
    properties: { key: { type: "string" }, email: { type: "string" }, reason: { type: "string" } },
    required: ["key", "email", "reason"],
  },
};

export default async function (input: Input) {
  await support.assignIssue({ path: { key: input.key }, body: { email: input.email } });
  await support.addComment({ path: { key: input.key }, body: { body: input.reason } });
  return { key: input.key, assignee: input.email };
}
`

func test(name, input, script string) store.ToolTest {
	return store.ToolTest{Name: name, Input: json.RawMessage(input), Script: script}
}

// Seed fills a fresh deck with the demo. dev adds the `dev` toolkit
// and the fixed-token dev agent (MOCK-4).
func Seed(ctx context.Context, d *deck.Deck, w *World, dev bool) error {
	step := func(what string, err error) error {
		if err != nil {
			return fmt.Errorf("demo: %s: %w", what, err)
		}
		return nil
	}
	secret := func(apiID, name, value string) error {
		a, err := d.GetAPI(ctx, apiID)
		if err != nil {
			return err
		}
		for _, v := range a.Variables {
			if v.Name == name {
				_, err := d.SetVariable(ctx, apiID, deck.SetVariableInput{ID: v.ID, EnvID: v.EnvID, Name: name, Source: "secret", Value: value})
				return err
			}
		}
		_, err = d.SetVariable(ctx, apiID, deck.SetVariableInput{EnvID: a.ActiveEnvID, Name: name, Source: "secret", Value: value})
		return err
	}

	crm, err := d.Import(ctx, deck.ImportInput{ImportSource: deck.ImportSource{Text: crmSpec(w.URL("crm"))}, Name: "crm"})
	if err := step("crm", err); err != nil {
		return err
	}
	_ = secret(crm.APIID, "token", CRMToken)
	billing, err := d.Import(ctx, deck.ImportInput{ImportSource: deck.ImportSource{Text: billingSpec(w.URL("billing"))}, Name: "billing"})
	if err := step("billing", err); err != nil {
		return err
	}
	_ = secret(billing.APIID, "apiKey", BillingKey)
	coll, _ := demoFiles.ReadFile("demo/acme-support.postman_collection.json")
	env, _ := demoFiles.ReadFile("demo/acme-support.postman_environment.json")
	fix := func(b []byte) string { return strings.ReplaceAll(string(b), "http://127.0.0.1:9003", w.URL("support")) }
	support, err := d.Import(ctx, deck.ImportInput{ImportSource: deck.ImportSource{Text: fix(coll), Environment: fix(env)}, Name: "support"})
	if err := step("support", err); err != nil {
		return err
	}
	_ = secret(support.APIID, "token", SupportToken)
	// searchIssues only searches: confirm it reads (API-1).
	sapi, _ := d.GetAPI(ctx, support.APIID)
	for _, r := range sapi.Requests {
		if r.Name == "searchIssues" {
			_, _ = d.SetAccess(ctx, r.ID, "read", "POST, marked read: it only searches")
		}
	}

	// Legacy: added manually, with a login request.
	leg, err := d.CreateAPI(ctx, deck.CreateAPIInput{Name: "legacy", Title: "Acme Legacy ERP", BaseURL: w.URL("legacy")})
	if err := step("legacy", err); err != nil {
		return err
	}
	login, err := d.SaveRequest(ctx, leg.ID, store.Request{Name: "login", Title: "Log in", Folder: "session", Method: "POST", URL: "{{baseUrl}}/login",
		BodyType: "form", Body: "username={{username}}&password={{password}}", Access: "write", AccessReason: "POST writes (starts a session)"})
	if err := step("legacy login", err); err != nil {
		return err
	}
	_, _ = d.SaveRequest(ctx, leg.ID, store.Request{Name: "getCreditHold", Title: "Credit hold for an account", Folder: "credit", Method: "GET", URL: "{{baseUrl}}/api/credit-hold/{account}"})
	_, _ = d.SaveRequest(ctx, leg.ID, store.Request{Name: "setCreditHold", Title: "Set credit hold (SOAP)", Folder: "credit", Method: "POST", URL: "{{baseUrl}}/soap/CreditService", BodyType: "xml",
		Body: `<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"><soap:Body><SetCreditHold><account>001Ab00000FBRKM</account><hold>true</hold></SetCreditHold></soap:Body></soap:Envelope>`})
	la, _ := d.GetAPI(ctx, leg.ID)
	_, _ = d.SetVariable(ctx, leg.ID, deck.SetVariableInput{EnvID: la.ActiveEnvID, Name: "username", Source: "literal", Value: LegacyUser})
	_, _ = d.SetVariable(ctx, leg.ID, deck.SetVariableInput{EnvID: la.ActiveEnvID, Name: "password", Source: "secret", Value: LegacyPass})
	_, _ = d.UpdateAPI(ctx, leg.ID, deck.UpdateAPIInput{Auth: &store.Auth{Type: "login", LoginRequestID: login.ID, Capture: "cookie"}})

	// Send each read request once and keep the response as its example.
	args := map[string]map[string]any{
		"crm.getAccount": {"id": "001Ab00000NWTRD"}, "crm.listContacts": {"id": "001Ab00000NWTRD"}, "crm.getOwner": {"id": "005OWN01"},
		"crm.searchAccounts": {"q": "north"}, "billing.getCustomerSummary": {"id": "C-1001"}, "billing.getInvoice": {"number": "INV-2087"},
		"billing.listInvoices": {"customerId": "C-1001", "status": "overdue"}, "support.getIssue": {}, "support.searchIssues": {},
		"legacy.getCreditHold": {"account": "001Ab00000FBRKM"},
	}
	for _, id := range []string{crm.APIID, billing.APIID, support.APIID, leg.ID} {
		a, _ := d.GetAPI(ctx, id)
		for _, r := range a.Requests {
			x, ok := args[a.Name+"."+r.Name]
			if !ok {
				continue
			}
			draft := r
			for k, v := range x {
				draft.URL = strings.ReplaceAll(draft.URL, "{"+k+"}", fmt.Sprint(v))
				for i := range draft.Query {
					if draft.Query[i].Key == k {
						draft.Query[i].Value, draft.Query[i].Disabled = fmt.Sprint(v), false
					}
				}
			}
			res, err := d.Send(ctx, deck.SendInput{APIID: id, Draft: &draft})
			if err == nil && res.Error == "" && res.Status < 300 {
				_, _ = d.SaveExample(ctx, r.ID, json.RawMessage(res.Body))
			}
		}
	}

	name, desc := "support", "Customer support: one customer's account, invoices and tickets"
	scope := []string{"crm", "billing", "support", "legacy"}
	tk, err := d.CreateToolkit(ctx, deck.ToolkitInput{Name: &name, Description: &desc, BuildScope: &scope})
	if err := step("toolkit", err); err != nil {
		return err
	}
	you := deck.Caller{Actor: audit.You}
	for _, t := range []deck.SaveInput{
		{Source: customer360, Tests: []store.ToolTest{test("Northwind", `{"accountId":"001Ab00000NWTRD"}`, `pm.test("has tier", () => pm.expect(output.tier).to.equal("gold"))`)}},
		{Source: openTickets, Tests: []store.ToolTest{test("Northwind", `{"accountId":"001Ab00000NWTRD"}`, `pm.test("tickets", () => pm.expect(output.tickets).to.be.an("array"))`)}},
		{Source: invoiceStatus, Tests: []store.ToolTest{test("overdue", `{"number":"INV-2087"}`, `pm.test("overdue", () => pm.expect(output.overdue).to.be.true)`)}},
	} {
		if _, err := d.SaveTool(ctx, tk.ID, you, t); err != nil {
			return step("tool", err)
		}
	}
	// Agents.
	cc, err := d.Connect(ctx, deck.ConnectInput{Client: "claude-code", ToolkitID: tk.ID, Name: "Claude Code on this laptop", Access: "read", CanBuild: true})
	if err := step("agent", err); err != nil {
		return err
	}
	cur, _ := d.Connect(ctx, deck.ConnectInput{Client: "cursor", ToolkitID: tk.ID, Name: "Cursor", Access: "read"})
	// The Claude Code agent built a write tool: it waits for review (AB-3).
	ccSess := deck.AgentSession{Agent: cc.Agent, Grant: cc.Grant, Toolkit: *tk, SessionID: "demo-session-cc", Mode: "build"}
	d.SessionSeen(ctx, cc.Agent, *tk, "build", ccSess.SessionID, "claude-code", "2.1")
	_, _ = d.AgentCall(ctx, ccSess, "search_apis", json.RawMessage(`{"query":"assign issue"}`))
	_, _ = d.AgentCall(ctx, ccSess, "describe_api", json.RawMessage(`{"request":"support.assignIssue"}`))
	src, _ := json.Marshal(reassignTicket)
	_, _ = d.AgentCall(ctx, ccSess, "save_tool", json.RawMessage(`{"source":`+string(src)+`,"tests":[]}`))
	// Some real use, so Activity and Home have something to show.
	use := deck.AgentSession{Agent: cc.Agent, Grant: cc.Grant, Toolkit: *tk, SessionID: "demo-session-cc-use", Mode: "use"}
	d.SessionSeen(ctx, cc.Agent, *tk, "use", use.SessionID, "claude-code", "2.1")
	for _, x := range []struct{ name, args string }{
		{"customer_360", `{"accountId":"001Ab00000NWTRD"}`},
		{"open_tickets_for_account", `{"accountId":"001Ab00000CNTSO"}`},
		{"invoice_status", `{"number":"INV-2087"}`},
		{"invoice_status", `{"number":"INV-9999"}`},
		{"customer_360", `{"accountId":"001Ab00000GLBEX"}`},
	} {
		_, _ = d.AgentCall(ctx, use, x.name, json.RawMessage(x.args))
	}
	_, _ = d.AgentCall(ctx, use, "report_field_usage", json.RawMessage(`{"tool":"customer_360","used":["name","tier","balance","overdueInvoices[].number"],"unused":["openTickets[].title"],"note":"only needed money owed"}`))
	cs := deck.AgentSession{Agent: cur.Agent, Grant: cur.Grant, Toolkit: *tk, SessionID: "demo-session-cursor", Mode: "use"}
	d.SessionSeen(ctx, cur.Agent, *tk, "use", cs.SessionID, "cursor", "2.4")
	_, _ = d.AgentCall(ctx, cs, "open_tickets_for_account", json.RawMessage(`{"accountId":"001Ab00000NWTRD"}`))
	_, _ = d.AgentCall(ctx, cs, "request_tool", json.RawMessage(`{"description":"Accounts whose credit hold changed this week, with the reason. Today I can only check one account at a time.","tried":"search_tools 'credit hold' found nothing"}`))

	if dev {
		name, desc := "dev", "Every demo API, for coding agents testing mcpDeck end to end"
		scope := []string{"crm", "billing", "support", "legacy"}
		dtk, err := d.CreateToolkit(ctx, deck.ToolkitInput{Name: &name, Description: &desc, BuildScope: &scope})
		if err != nil {
			return step("dev toolkit", err)
		}
		for _, t := range []string{"customer_360", "open_tickets_for_account", "invoice_status"} {
			_ = d.AddToolToToolkit(ctx, dtk.ID, t)
		}
		now := time.Now().UTC()
		a := store.Agent{ID: d.IDs.New(), Name: "Dev agent (fixed token)", Client: "claude-code", TokenHash: deck.HashToken(DevToken), TokenPrefix: DevToken[:10], CreatedAt: now}
		if err := d.Store.PutAgent(ctx, a); err != nil {
			return err
		}
		_ = d.Store.PutGrant(ctx, store.Grant{ID: d.IDs.New(), AgentID: a.ID, ToolkitID: dtk.ID, Access: "read", CanBuild: true, CreatedAt: now})
	}
	return nil
}
