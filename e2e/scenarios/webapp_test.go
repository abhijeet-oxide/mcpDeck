package scenarios

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/abhijeet-oxide/mcpdeck/internal/browser"
	"github.com/abhijeet-oxide/mcpdeck/internal/tokens"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// A web app with no API, signed in through company single sign-on: the
// mock expense portal. The person's browser is a real headless Chrome; the
// portal's pages act as the person (mockenv AutoSignIn, AutoRecord).

func needBrowser(t *testing.T) {
	t.Helper()
	if _, err := browser.Find(); err != nil {
		t.Skip("no Edge or Chrome on this machine")
	}
}

func example(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "examples", "expenses", "expenses", name+".ts"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

var (
	expensesOnce sync.Once
	expensesErr  string
	expensesRec  map[string]any
	receiptsDir  string
)

// copilot is the agent a person adds with Home › Build tools with your
// agent: the admin MCP server on stdio. Everything it builds goes
// through these tools; the person only signs in, clicks through the task
// once and confirms.
func copilot(t *testing.T) *agent { t.Helper(); return h.adminFor(t, "GitHub Copilot") }

// adminCall calls an admin tool without failing the test from a goroutine.
func adminCall(ad *agent, name string, args map[string]any, timeout time.Duration) (string, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	res, err := ad.cs.CallTool(ctx, &sdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		return "", true, err
	}
	var parts []string
	for _, c := range res.Content {
		if tc, ok := c.(*sdk.TextContent); ok {
			parts = append(parts, tc.Text)
		}
	}
	return strings.Join(parts, "\n"), res.IsError, nil
}

// personConfirms waits for a confirmation mcpDeck shows the person, checks
// its title and answers it as they would.
func personConfirms(t *testing.T, titlePart string, ok bool) map[string]any {
	t.Helper()
	var waiting []map[string]any
	for i := 0; i < 150 && len(waiting) == 0; i++ {
		time.Sleep(100 * time.Millisecond)
		h.mustAPI(t, "GET", "/api/confirmations", nil, &waiting)
	}
	expect(t, len(waiting) == 1, "nothing waits for the person to confirm %q", titlePart)
	c := waiting[0]
	expect(t, strings.Contains(c["title"].(string), titlePart), "confirmation shows %s", asJSON(c))
	err := h.apiAs("GitHub Copilot", "POST", "/api/confirmations/"+c["id"].(string), map[string]any{"confirm": true}, nil)
	expect(t, err != nil, "the agent confirmed for the person")
	h.mustAPI(t, "POST", "/api/confirmations/"+c["id"].(string), map[string]any{"confirm": ok}, nil)
	return c
}

// recordExpenses: the person asks Copilot to build tools for the portal;
// Copilot calls record_workflow; the person signs in and clicks through
// one claim, then clicks Done. Once.
func recordExpenses(t *testing.T) map[string]any {
	t.Helper()
	needBrowser(t)
	expensesOnce.Do(func() {
		env := h.env
		env.AutoSignIn.Store(true)
		env.AutoRecord.Store(true)
		defer env.AutoRecord.Store(false)
		ad := copilot(t)
		type res struct {
			text  string
			isErr bool
			err   error
		}
		done := make(chan res, 1)
		go func() {
			text, isErr, err := adminCall(ad, "record_workflow", map[string]any{"api": "expenses", "url": env.URLs["expenses"] + "/expenses/claims"}, 2*time.Minute)
			done <- res{text, isErr, err}
		}()
		deadline := time.Now().Add(45 * time.Second)
		for time.Now().Before(deadline) {
			submitted := false
			for _, c := range env.ExpenseClaims() {
				if c["purpose"] == "Home internet November" && c["status"] == "Submitted" {
					submitted = true
				}
			}
			if submitted {
				break
			}
			time.Sleep(200 * time.Millisecond)
		}
		// The person closes the window (here: Done, as headless Chrome can't).
		if err := h.api("POST", "/api/record/stop", map[string]any{"api": "expenses"}, nil); err != nil {
			// Usually the recording already ended with its own error.
			select {
			case r := <-done:
				expensesErr = fmt.Sprintf("record_workflow ended early: %v %s", r.err, r.text)
			case <-time.After(5 * time.Second):
				expensesErr = "stop: " + err.Error()
			}
			return
		}
		select {
		case r := <-done:
			if r.err != nil || r.isErr {
				expensesErr = fmt.Sprintf("record_workflow: %v %s", r.err, r.text)
				return
			}
			if err := json.Unmarshal([]byte(r.text), &expensesRec); err != nil {
				expensesErr = "record_workflow returned " + clip(r.text, 300)
			}
		case <-time.After(30 * time.Second):
			expensesErr = "record_workflow didn't return after Done"
		}
	})
	if expensesErr != "" {
		t.Fatal(expensesErr)
	}
	return expensesRec
}

// expenseKit: Copilot builds the toolkit from the recording through the
// admin MCP server; the person confirms the receipts folder and approves
// the write tool. Once.
var kitOnce sync.Once

func expenseKit(t *testing.T) {
	t.Helper()
	recordExpenses(t)
	kitOnce.Do(func() {
		receiptsDir, _ = os.MkdirTemp("", "mcpdeck-receipts-")
		_ = os.WriteFile(filepath.Join(receiptsDir, "december-internet.pdf"), []byte("%PDF-1.4 december internet receipt C4N4RY-pdf"), 0o600)
		ad := copilot(t)
		ad.must("create_toolkit", map[string]any{"name": "expenses", "apis": []any{"expenses"}, "description": "Expense reports: list them and submit one with its receipt."})
		got := make(chan string, 1)
		go func() {
			text, isErr, err := adminCall(ad, "set_files_folder", map[string]any{"api": "expenses", "folder": receiptsDir}, time.Minute)
			if err != nil || isErr {
				text = fmt.Sprintf("failed: %v %s", err, text)
			}
			got <- text
		}()
		personConfirms(t, "upload files from", true)
		r := <-got
		expect(t, strings.Contains(r, `"ok":true`), "set_files_folder: %s", r)
		read := ad.must("save_tool", map[string]any{"toolkit": "expenses", "source": example(t, "list_claims"),
			"tests": []any{map[string]any{"name": "lists claims", "input": map[string]any{}, "script": `pm.test("claims", () => pm.expect(output.claims.length).to.be.above(0))`}}})
		expect(t, strings.Contains(read, "live"), "list_claims didn't go live: %s", read)
		write := ad.must("save_tool", map[string]any{"toolkit": "expenses", "source": example(t, "submit_claim")})
		expect(t, strings.Contains(write, "pending"), "submit_claim should wait for the person: %s", write)
		// The person approves it in mcpDeck › Review.
		h.mustCLI(t, "tool", "approve", "expenses", "submit_claim")
	})
}

func init() {
	add("webapp", "record_workflow turns one clicked-through task into requests with form fields, without cookies or values", func(t *testing.T) {
		rec := recordExpenses(t)
		text := asJSON(rec)
		expect(t, rec["signedIn"] == true, "the recording kept no session: %s", clip(text, 400))
		reqs := map[string]map[string]any{}
		for _, r := range rec["requests"].([]any) {
			m := r.(map[string]any)
			reqs[m["method"].(string)+" "+m["path"].(string)] = m
		}
		post := reqs["POST /expenses/claims"]
		expect(t, post != nil, "no form post recorded: %s", clip(text, 600))
		fields := get(post, "form.fields").(map[string]any)
		expect(t, strings.Contains(fields["_state"].(string), "hidden"), "_state isn't marked hidden: %v", fields)
		expect(t, fields["amount"] == "number" && fields["date"] == "date", "typed fields not described by kind: %v", fields)
		expect(t, post["goesTo"] == "/expenses/claims/{id}", "redirect target %v", post["goesTo"])
		up := reqs["POST /expenses/claims/{id}/receipt"]
		expect(t, up != nil && get(up, "form.contentType") == "multipart/form-data" && get(up, "form.fields.receipt") == "file", "upload not recorded as a file: %s", clip(text, 800))
		for _, leak := range []string{"EXPSESSIONID", "52.10", "Home internet", "/sso/"} {
			expect(t, !strings.Contains(text, leak), "the recording shows %q to the agent: %s", leak, clip(text, 400))
		}
		// Enough for the agent to write the tools: page forms and table columns.
		for _, want := range []string{"claimForm POST /expenses/claims", "receiptForm POST /expenses/claims/{id}/receipt multipart", "submitForm POST /expenses/claims/{id}/submit",
			`["Report","Date","Amount","Category","Purpose","Status","Receipt"]`, "getExpensesClaimsNew", "postExpensesClaimsReceiptById", "postExpensesClaimsSubmitById", "set_files_folder"} {
			expect(t, strings.Contains(text, want), "the recording doesn't tell the agent %q: %s", want, clip(text, 2000))
		}
		h.noLeaks(t, "record_workflow", text)
		tokensUsed := tokens.Count(text)
		t.Logf("record_workflow result: %d tokens", tokensUsed)
		expect(t, tokensUsed < 1500, "record_workflow returns %d tokens; keep it under 1500", tokensUsed)
		a := h.apiByName(t, "expenses")
		expect(t, a.Auth["type"] == "browser", "the new application doesn't sign in through the browser: %v", a.Auth)
	})

	add("webapp", "an agent lists claims and submits a claim with a receipt from the folder once the person confirms", func(t *testing.T) {
		expenseKit(t)
		ag := h.agentFor(t, "expenses", "read_write")
		// The portal session ended overnight: the browser signs in again.
		h.env.ExpireExpenses()
		claims := ag.ok("list_claims", map[string]any{})
		expect(t, strings.Contains(asJSON(claims), `"1001"`), "no claims listed: %s", clip(asJSON(claims), 300))
		expect(t, !strings.Contains(strings.Join(ag.tools(), ","), "list_receipts"), "a receipts tool is listed; the agent reads the folder itself")

		before := h.env.Count("expenses.submit")
		type res struct {
			text  string
			isErr bool
		}
		out := make(chan res, 1)
		go func() {
			// The agent read december-internet.pdf itself and passes its path.
			text, isErr := ag.call("submit_claim", map[string]any{"date": "2026-12-02", "amount": "48.00", "category": "Internet", "purpose": "Home internet December", "receipt": filepath.Join(receiptsDir, "december-internet.pdf")})
			out <- res{text, isErr}
		}()
		c := personConfirms(t, "Submit Internet claim of 48.00", true)
		expect(t, get(c, "fields.receipt") == "december-internet.pdf", "confirmation shows %s", asJSON(c))
		r := <-out
		expect(t, !r.isErr, "submit_claim failed: %s", r.text)
		v := decodeOut(r.text)
		expect(t, get(v, "status") == "Submitted" && get(v, "receipt") == "december-internet.pdf", "result %s", r.text)
		expect(t, h.env.Count("expenses.submit") == before+1, "submitted %d times", h.env.Count("expenses.submit")-before)
		found := false
		for _, cl := range h.env.ExpenseClaims() {
			if cl["purpose"] == "Home internet December" {
				found = cl["receipt"] == "december-internet.pdf" && cl["size"].(int) > 10
			}
		}
		expect(t, found, "the portal has no December claim with its receipt: %v", h.env.ExpenseClaims())
		var calls []map[string]any
		h.mustAPI(t, "GET", "/api/calls?all=1&limit=500", nil, &calls)
		log := asJSON(calls)
		expect(t, !strings.Contains(log, "C4N4RY-pdf"), "the call log kept the receipt's contents")
		h.noLeaksInCallLog(t)
	})

	add("webapp", "a declined confirmation sends nothing", func(t *testing.T) {
		expenseKit(t)
		ag := h.agentFor(t, "expenses", "read_write")
		before := h.env.Count("expenses.create")
		out := make(chan string, 1)
		go func() {
			out <- ag.fails("submit_claim", map[string]any{"date": "2026-12-03", "amount": "9.99", "category": "Mobile", "purpose": "Declined", "receipt": "december-internet.pdf"}, "declined")
		}()
		personConfirms(t, "Submit Mobile claim", false)
		<-out
		expect(t, h.env.Count("expenses.create") == before, "a report was created after the person declined")
	})

	add("webapp", "a read agent can list claims but can't submit one", func(t *testing.T) {
		expenseKit(t)
		ag := h.agentFor(t, "expenses", "read")
		ag.ok("list_claims", map[string]any{"status": "Submitted"})
		ag.fails("submit_claim", map[string]any{"date": "2026-12-03", "amount": "1", "category": "Mobile", "purpose": "x", "receipt": "december-internet.pdf"}, "can't see")
	})

	add("webapp", "the toolkit exports to a file without secrets and imports elsewhere with its write tool waiting for approval", func(t *testing.T) {
		expenseKit(t)
		var bundle map[string]any
		h.mustAPI(t, "GET", "/api/toolkits/expenses/export", nil, &bundle)
		text := asJSON(bundle)
		h.noLeaks(t, "the toolkit file", text)
		for _, leak := range []string{"EXPSESSIONID", receiptsDir, "C4N4RY-pdf"} {
			expect(t, !strings.Contains(text, leak), "the toolkit file holds %q", leak)
		}
		expect(t, len(bundle["tools"].([]any)) == 2, "tools in file: %d", len(bundle["tools"].([]any)))

		// A colleague's mcpDeck.
		other := &harness{bin: h.bin, data: filepath.Join(t.TempDir(), "data"), env: h.env}
		if err := other.start(); err != nil {
			t.Fatal(err)
		}
		defer other.stop()
		var res map[string]any
		if err := other.api("POST", "/api/toolkit-import", bundle, &res); err != nil {
			t.Fatal(err)
		}
		status := map[string]string{}
		for _, x := range res["tools"].([]any) {
			m := x.(map[string]any)
			status[m["name"].(string)] = m["status"].(string)
		}
		expect(t, status["submit_claim"] == "pending" && status["list_claims"] == "live", "imported statuses %v", status)
		expect(t, strings.Contains(asJSON(res["signIn"]), "expenses") && strings.Contains(asJSON(res["filesFor"]), "expenses"), "import didn't say what to do next: %s", asJSON(res))
		// The file can't be imported by an agent.
		err := other.apiAs("GitHub Copilot", "POST", "/api/toolkit-import", bundle, nil)
		expect(t, err != nil, "an agent imported a toolkit file")
	})

	add("webapp", "Build with Copilot adds the admin MCP server in one click", func(t *testing.T) {
		var out struct {
			Plan struct {
				Link    string `json:"link"`
				Snippet string `json:"snippet"`
			} `json:"plan"`
		}
		h.mustAPI(t, "POST", "/api/connect-builder", map[string]any{"client": "vscode"}, &out)
		expect(t, strings.HasPrefix(out.Plan.Link, "vscode:mcp/install?") && strings.Contains(out.Plan.Snippet, `"mcp"`) && strings.Contains(out.Plan.Snippet, "GitHub Copilot"),
			"plan %+v", out.Plan)
		err := h.apiAs("GitHub Copilot", "POST", "/api/connect-builder", map[string]any{"client": "vscode"}, nil)
		expect(t, err != nil, "an agent added the admin server to a client")
	})
}
