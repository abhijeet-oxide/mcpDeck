package tool

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/abhijeet-oxide/mcpdeck/internal/proxy"
)

const claimsPage = `<html><body><h1>Expense reports</h1>
<table id="claims"><tr><th>Report</th><th>Amount</th><th>Status</th></tr>
<tr><td><a href="/expenses/claims/1002">1002</a></td><td>52.10</td><td>Draft</td></tr>
<tr><td><a href="/expenses/claims/1001">1001</a></td><td>45.00</td><td>Submitted</td></tr></table>
<form name="claimForm" method="post" action="/expenses/claims">
<input type="hidden" name="_state" value="s1"><input type="hidden" name="event" value="">
<label>Amount <input name="amount" value="1"></label>
<select name="category"><option>Internet</option><option selected value="m">Mobile</option></select>
<input type="checkbox" name="tag" value="a" checked><input type="checkbox" name="tag" value="b" checked><input type="checkbox" name="tag" value="c">
<textarea name="note"> hi </textarea><button name="go">Save</button></form>
<form name="receiptForm" method="post" enctype="multipart/form-data" action="/r"><input type="file" name="receipt"></form>
<script>var secret = 1</script></body></html>`

// REQ: TOOL-12
func TestParseForms(t *testing.T) {
	fs := ParseForms(claimsPage)
	if len(fs) != 2 {
		t.Fatalf("forms %d", len(fs))
	}
	f := fs[0]
	if f.Name != "claimForm" || f.Method != "POST" || f.Action != "/expenses/claims" || f.Enctype != "application/x-www-form-urlencoded" {
		t.Fatalf("form %+v", f)
	}
	if f.Hidden["_state"] != "s1" || f.Values["category"] != "m" || f.Values["note"] != "hi" {
		t.Fatalf("values %+v hidden %+v", f.Values, f.Hidden)
	}
	if tags, ok := f.Values["tag"].([]any); !ok || len(tags) != 2 {
		t.Fatalf("repeated checked fields %#v", f.Values["tag"])
	}
	if _, posted := f.Values["go"]; posted {
		t.Fatal("buttons aren't posted")
	}
	if fs[1].Enctype != "multipart/form-data" || fs[1].Fields[0].Type != "file" {
		t.Fatalf("upload form %+v", fs[1])
	}
}

// REQ: TOOL-12
func TestParseTablesAndText(t *testing.T) {
	ts := ParseTables(claimsPage)
	if len(ts) != 1 || len(ts[0]) != 2 {
		t.Fatalf("tables %+v", ts)
	}
	if r := ts[0][0]; r["Report"] != "1002" || r["Status"] != "Draft" || r["href"] != "/expenses/claims/1002" {
		t.Fatalf("row %+v", r)
	}
	txt := PageText(claimsPage)
	if !strings.Contains(txt, "Expense reports") || strings.Contains(txt, "secret") {
		t.Fatalf("text %q", txt)
	}
}

type fakeFiles struct{}

func (fakeFiles) ListFiles(context.Context, string) ([]proxy.FileInfo, error) {
	return []proxy.FileInfo{{Name: "november.pdf", Size: 10}}, nil
}

const submitTool = `export const tool = { name: "submit_claim", description: "Submits a claim.", confirm: true };
export default async function () {
  const page = await crm.getAccount.raw({});
  const f = html.forms(` + "`<form><input type=hidden name=_state value=s9></form>`" + `)[0];
  const [file] = await files.list("crm");
  await confirm({ title: "Submit claim", fields: { amount: "52.10", receipt: file.name } });
  await crm.getAccount({ ...f.values, receipt: files.attach(file.name) });
  return { ok: true };
}`

// REQ: CONF-1
func TestConfirm_MarksLaterCallsConfirmed(t *testing.T) {
	s := &fakeSender{}
	e := env(s)
	e.Files = fakeFiles{}
	var asked string
	e.Confirm = func(_ context.Context, title string, fields any) error {
		asked = title
		return nil
	}
	r := Run(context.Background(), submitTool, nil, e)
	if r.Error != "" {
		t.Fatal(r.Error)
	}
	if asked != "Submit claim" {
		t.Fatalf("asked %q", asked)
	}
	if len(s.calls) != 2 || !s.calls[0].NeedConfirm || s.calls[0].Confirmed || !s.calls[1].Confirmed {
		t.Fatalf("calls %+v", s.calls)
	}
	f := s.calls[1].Args.Flat
	if f["_state"] != "s9" {
		t.Fatalf("form values not passed: %+v", f)
	}
	if ref, _ := f["receipt"].(map[string]any); ref["$file"] != "november.pdf" {
		t.Fatalf("file reference %+v", f["receipt"])
	}
}

// REQ: CONF-1
func TestConfirm_DeclineSendsNothing(t *testing.T) {
	s := &fakeSender{}
	e := env(s)
	e.Files = fakeFiles{}
	e.Confirm = func(context.Context, string, any) error { return errors.New("the user declined") }
	r := Run(context.Background(), submitTool, nil, e)
	if !strings.Contains(r.Error, "declined") {
		t.Fatalf("error %q", r.Error)
	}
	if len(s.calls) != 1 {
		t.Fatalf("calls after decline: %d", len(s.calls))
	}
}
