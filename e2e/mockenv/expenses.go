package mockenv

import (
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// expenses is a server-rendered expense portal with no REST API, shaped like
// the Oracle self-service expense pages people automate by hand:
//
//   - Every page needs a session cookie. Without one the portal answers with
//     a 302 to its single sign-on page (/sso/authorize?return=…), where a
//     person types a user, a password and a one-time code. Tools can't do
//     that; a person signs in through a browser (AUTH-5).
//   - Every form carries a hidden _state value that is good for one post
//     from this session only, so a form has to be fetched before it is posted.
//   - Posts answer with a 302 back to a page (post/redirect/get).
//   - The receipt is a multipart/form-data upload (FILE-1).
//   - Submitting twice is refused with "already submitted".
func (e *Env) expenses() http.Handler {
	type claim struct {
		ID       int
		Date     string
		Amount   string
		Category string
		Purpose  string
		Status   string // Draft | Submitted
		Receipt  string
		Size     int
	}
	var mu sync.Mutex
	sessions := map[string]string{} // session → user
	states := map[string]string{}   // _state → session
	claims := []*claim{{ID: 1001, Date: "2026-10-05", Amount: "45.00", Category: "Internet", Purpose: "Home internet October", Status: "Submitted", Receipt: "october-internet.pdf", Size: 2048}}
	next := 1002
	e.expireExpenses = func() {
		mu.Lock()
		defer mu.Unlock()
		for k := range sessions {
			delete(sessions, k)
		}
	}
	e.expenseClaims = func() []map[string]any {
		mu.Lock()
		defer mu.Unlock()
		out := []map[string]any{}
		for _, c := range claims {
			out = append(out, map[string]any{"id": c.ID, "amount": c.Amount, "purpose": c.Purpose, "status": c.Status, "receipt": c.Receipt, "size": c.Size})
		}
		return out
	}
	user := func(r *http.Request) string {
		c, err := r.Cookie("EXPSESSIONID")
		if err != nil {
			return ""
		}
		mu.Lock()
		defer mu.Unlock()
		return sessions[c.Value]
	}
	newState := func(r *http.Request) string {
		c, _ := r.Cookie("EXPSESSIONID")
		s := randHex(12)
		mu.Lock()
		states[s] = c.Value
		mu.Unlock()
		return s
	}
	useState := func(r *http.Request, s string) bool {
		c, _ := r.Cookie("EXPSESSIONID")
		mu.Lock()
		defer mu.Unlock()
		owner, ok := states[s]
		delete(states, s)
		return ok && c != nil && owner == c.Value
	}
	page := func(w http.ResponseWriter, status int, title, body string) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(status)
		// In scenarios a script stands in for the person in a real browser.
		if e.AutoSignIn.Load() && strings.Contains(body, `name="otp"`) {
			body += `<script>(function(){const f=document.forms[0];if(f&&f.otp){f.user.value=` + jsStr(ExpenseUser) + `;f.password.value=` + jsStr(ExpensePass) + `;f.otp.value=` + jsStr(ExpenseOTP) + `;f.submit()}})()</script>`
		}
		if e.AutoRecord.Load() {
			body += autoplay
		}
		fmt.Fprintf(w, `<!doctype html><html><head><title>%s</title><script>function submitForm(f,e){document.forms[f].event.value=e;document.forms[f].submit()}</script></head><body><h1>%s</h1>%s</body></html>`, title, title, body)
	}
	signedIn := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if user(r) == "" {
				http.Redirect(w, r, "/sso/authorize?return="+url.QueryEscape(r.URL.RequestURI()), http.StatusFound)
				return
			}
			h(w, r)
		}
	}
	find := func(r *http.Request) *claim {
		id, _ := strconv.Atoi(r.PathValue("id"))
		mu.Lock()
		defer mu.Unlock()
		for _, c := range claims {
			if c.ID == id {
				return c
			}
		}
		return nil
	}
	badState := func(w http.ResponseWriter) {
		e.inc("expenses.stale_state")
		page(w, http.StatusConflict, "Page expired", `<p class="error">This page has expired. Return to the previous page and try again.</p>`)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /sso/authorize", func(w http.ResponseWriter, r *http.Request) {
		page(w, 200, "Corporate sign-in", fmt.Sprintf(`<form method="post" action="/sso/authorize"><input type="hidden" name="return" value="%s">
<label>User <input name="user"></label><label>Password <input name="password" type="password"></label><label>One-time code <input name="otp"></label><button>Sign in</button></form>`, html.EscapeString(r.URL.Query().Get("return"))))
	})
	mux.HandleFunc("POST /sso/authorize", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.PostForm.Get("user") != ExpenseUser || r.PostForm.Get("password") != ExpensePass || r.PostForm.Get("otp") != ExpenseOTP {
			page(w, 200, "Corporate sign-in", `<p class="error">Sign-in failed.</p>`)
			return
		}
		e.inc("expenses.signin")
		sid := randHex(16)
		mu.Lock()
		sessions[sid] = ExpenseUser
		mu.Unlock()
		http.SetCookie(w, &http.Cookie{Name: "EXPSESSIONID", Value: sid, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode})
		ret := r.PostForm.Get("return")
		if !strings.HasPrefix(ret, "/") || strings.HasPrefix(ret, "//") {
			ret = "/expenses/claims"
		}
		http.Redirect(w, r, ret, http.StatusFound)
	})
	mux.HandleFunc("GET /expenses/claims", signedIn(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		list := append([]*claim(nil), claims...)
		mu.Unlock()
		sort.Slice(list, func(i, j int) bool { return list[i].ID > list[j].ID })
		var b strings.Builder
		b.WriteString(`<p><a href="/expenses/claims/new">Create expense report</a></p><table id="claims"><tr><th>Report</th><th>Date</th><th>Amount</th><th>Category</th><th>Purpose</th><th>Status</th><th>Receipt</th></tr>`)
		for _, c := range list {
			fmt.Fprintf(&b, `<tr><td><a href="/expenses/claims/%d">%d</a></td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>`,
				c.ID, c.ID, c.Date, c.Amount, html.EscapeString(c.Category), html.EscapeString(c.Purpose), c.Status, html.EscapeString(c.Receipt))
		}
		b.WriteString(`</table>`)
		page(w, 200, "Expense reports", b.String())
	}))
	mux.HandleFunc("GET /expenses/claims/new", signedIn(func(w http.ResponseWriter, r *http.Request) {
		page(w, 200, "Create expense report", fmt.Sprintf(`<form name="claimForm" method="post" action="/expenses/claims">
<input type="hidden" name="_state" value="%s"><input type="hidden" name="event" value="">
<label>Date <input name="date" type="date"></label><label>Amount <input name="amount"></label>
<label>Category <select name="category"><option>Internet</option><option>Mobile</option><option>Travel</option></select></label>
<label>Purpose <input name="purpose"></label>
<button type="button" onclick="submitForm('claimForm','save')">Save</button></form>`, newState(r)))
	}))
	mux.HandleFunc("POST /expenses/claims", signedIn(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if !useState(r, r.PostForm.Get("_state")) {
			badState(w)
			return
		}
		f := r.PostForm
		if f.Get("event") != "save" || f.Get("date") == "" || f.Get("amount") == "" || f.Get("purpose") == "" {
			page(w, 200, "Create expense report", `<p class="error">Date, amount and purpose are required.</p>`)
			return
		}
		if _, err := strconv.ParseFloat(f.Get("amount"), 64); err != nil {
			page(w, 200, "Create expense report", `<p class="error">Amount must be a number.</p>`)
			return
		}
		mu.Lock()
		c := &claim{ID: next, Date: f.Get("date"), Amount: f.Get("amount"), Category: f.Get("category"), Purpose: f.Get("purpose"), Status: "Draft"}
		next++
		claims = append(claims, c)
		mu.Unlock()
		e.inc("expenses.create")
		http.Redirect(w, r, fmt.Sprintf("/expenses/claims/%d", c.ID), http.StatusFound)
	}))
	mux.HandleFunc("GET /expenses/claims/{id}", signedIn(func(w http.ResponseWriter, r *http.Request) {
		c := find(r)
		if c == nil {
			page(w, 404, "Not found", `<p class="error">No such expense report.</p>`)
			return
		}
		body := fmt.Sprintf(`<dl id="claim"><dt>Report</dt><dd>%d</dd><dt>Amount</dt><dd>%s</dd><dt>Purpose</dt><dd>%s</dd><dt>Status</dt><dd id="status">%s</dd><dt>Receipt</dt><dd id="receipt">%s</dd></dl>`,
			c.ID, c.Amount, html.EscapeString(c.Purpose), c.Status, html.EscapeString(c.Receipt))
		if c.Status == "Draft" {
			body += fmt.Sprintf(`<form name="receiptForm" method="post" enctype="multipart/form-data" action="/expenses/claims/%d/receipt"><input type="hidden" name="_state" value="%s"><input type="file" name="receipt"><button>Upload</button></form>`, c.ID, newState(r))
			body += fmt.Sprintf(`<form name="submitForm" method="post" action="/expenses/claims/%d/submit"><input type="hidden" name="_state" value="%s"><button>Submit</button></form>`, c.ID, newState(r))
		}
		page(w, 200, fmt.Sprintf("Expense report %d", c.ID), body)
	}))
	mux.HandleFunc("POST /expenses/claims/{id}/receipt", signedIn(func(w http.ResponseWriter, r *http.Request) {
		c := find(r)
		if c == nil {
			page(w, 404, "Not found", `<p class="error">No such expense report.</p>`)
			return
		}
		if err := r.ParseMultipartForm(5 << 20); err != nil {
			e.inc("expenses.bad_upload")
			page(w, 400, "Upload failed", `<p class="error">Attach the receipt as a file (multipart/form-data, up to 5 MB).</p>`)
			return
		}
		if !useState(r, r.FormValue("_state")) {
			badState(w)
			return
		}
		f, hdr, err := r.FormFile("receipt")
		if err != nil {
			page(w, 400, "Upload failed", `<p class="error">Choose a receipt file.</p>`)
			return
		}
		defer f.Close()
		n, _ := io.Copy(io.Discard, f)
		ct := hdr.Header.Get("Content-Type")
		if ct != "application/pdf" && ct != "image/png" && ct != "image/jpeg" {
			page(w, 400, "Upload failed", `<p class="error">Receipts must be PDF, PNG or JPEG.</p>`)
			return
		}
		mu.Lock()
		c.Receipt, c.Size = hdr.Filename, int(n)
		mu.Unlock()
		e.inc("expenses.upload")
		http.Redirect(w, r, fmt.Sprintf("/expenses/claims/%d", c.ID), http.StatusFound)
	}))
	mux.HandleFunc("POST /expenses/claims/{id}/submit", signedIn(func(w http.ResponseWriter, r *http.Request) {
		c := find(r)
		if c == nil {
			page(w, 404, "Not found", `<p class="error">No such expense report.</p>`)
			return
		}
		_ = r.ParseForm()
		mu.Lock()
		status, receipt := c.Status, c.Receipt
		mu.Unlock()
		if status == "Submitted" {
			page(w, http.StatusConflict, "Already submitted", `<p class="error">This expense report was already submitted.</p>`)
			return
		}
		if !useState(r, r.PostForm.Get("_state")) {
			badState(w)
			return
		}
		if receipt == "" {
			page(w, 200, fmt.Sprintf("Expense report %d", c.ID), `<p class="error">Attach a receipt before you submit.</p>`)
			return
		}
		mu.Lock()
		c.Status = "Submitted"
		mu.Unlock()
		e.inc("expenses.submit")
		http.Redirect(w, r, fmt.Sprintf("/expenses/claims/%d", c.ID), http.StatusFound)
	}))
	return mux
}

// autoplay is a person clicking through one claim: create it, attach a
// receipt, submit it.
const autoplay = `<script>(function(){if(sessionStorage.done)return;const p=location.pathname;
if(p==='/expenses/claims'){if(!sessionStorage.started){sessionStorage.started=1;setTimeout(()=>location.href='/expenses/claims/new',100)}return}
if(p==='/expenses/claims/new'){const f=document.forms.claimForm;f.date.value='2026-11-03';f.amount.value='52.10';f.purpose.value='Home internet November';setTimeout(()=>submitForm('claimForm','save'),100);return}
const st=document.getElementById('status');if(!st)return;
if(st.textContent==='Submitted'){sessionStorage.done=1;return}
if(!document.getElementById('receipt').textContent){const dt=new DataTransfer();dt.items.add(new File(['%PDF-1.4 receipt'],'november-internet.pdf',{type:'application/pdf'}));const f=document.forms.receiptForm;f.receipt.files=dt.files;setTimeout(()=>f.submit(),100);return}
setTimeout(()=>document.forms.submitForm.submit(),100)})()</script>`

func jsStr(s string) string { return "'" + strings.ReplaceAll(s, "'", "\\'") + "'" }

// ExpireExpenses ends every expense portal session, as an idle timeout does.
func (e *Env) ExpireExpenses() { e.expireExpenses() }

// ExpenseClaims lists the portal's claims, for assertions.
func (e *Env) ExpenseClaims() []map[string]any { return e.expenseClaims() }
