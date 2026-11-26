package deck

import "testing"

// REQ: TOOL-10, FLOW-1
func TestResultName(t *testing.T) {
	for in, want := range map[string]string{
		"listInvoices": "invoices", "getCustomerSummary": "customerSummary", "remindInvoice": "remindInvoice",
		"get": "get", "getInput": "result", "searchIssues": "issues", "do": "do",
	} {
		if got := resultName(in); got != want {
			t.Errorf("%s: got %s, want %s", in, got, want)
		}
	}
}
