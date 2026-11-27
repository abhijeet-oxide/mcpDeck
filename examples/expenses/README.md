# Expenses: a web app with no API

Three tools for a server-rendered expense portal that signs in through
company single sign-on. They were built the way the [Build with an
agent](../../docs/agents.md#web-apps-without-an-api) flow builds them: the
agent ran `record_workflow`, a person signed in and clicked through one
claim, and the agent wrote these tools against the recorded requests.

| Tool | Access | What it does |
|---|---|---|
| `list_claims` | read | Reads the reports table with `html.tables` |
| `list_receipts` | read | Lists the receipts folder with `files.list` (names only) |
| `submit_claim` | write, `confirm: true` | Fetches each form's one-time state with `html.forms`, creates the report, uploads the receipt with `files.attach`, submits, after the person confirms |

The scenario suite runs them against the mock portal in
`e2e/mockenv/expenses.go` (`e2e/scenarios/webapp_test.go`).
