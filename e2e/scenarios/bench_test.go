package scenarios

import (
	"context"
	"encoding/json"
	"flag"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/abhijeet-oxide/mcpdeck/e2e/mockenv"
	"github.com/abhijeet-oxide/mcpdeck/internal/tokens"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// The token benchmark in the README (NFR-9). Two ways to give an agent the
// same systems: one generated tool per read endpoint that returns the whole
// response, or one task tool per question. It counts what the agent reads:
// server instructions, tools/list and every tool result, in cl100k_base.
//
//	go test ./e2e/scenarios -run 'TestScenarios/benchmark' -args -bench-out ../../bench/results.json
//
// Without -bench-out it checks that the committed numbers still reproduce.

var benchOut = flag.String("bench-out", "", "write the token benchmark to this file instead of comparing with bench/results.json")

type benchSide struct {
	Tools      int `json:"tools"`
	ListTokens int `json:"listTokens"`
	Calls      int `json:"calls"`
	CallTokens int `json:"callTokens"`
	Total      int `json:"total"`
}

type benchQuestion struct {
	Question  string    `json:"question"`
	Endpoints benchSide `json:"endpoints"`
	Tasks     benchSide `json:"tasks"`
	SavingPct int       `json:"savingPct"`
}

type benchResults struct {
	Encoding  string          `json:"encoding"`
	Method    string          `json:"method"`
	Questions []benchQuestion `json:"questions"`
	Builder   struct {
		Tools      int `json:"tools"`
		ListTokens int `json:"listTokens"`
	} `json:"builder"`
}

func mcpSession(t *testing.T, path, token string) (*sdk.ClientSession, int, int) {
	t.Helper()
	cl := sdk.NewClient(&sdk.Implementation{Name: "bench", Version: "1"}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cs, err := cl.Connect(ctx, &sdk.StreamableClientTransport{Endpoint: h.base + path, HTTPClient: &http.Client{Transport: bearerRT{token, http.DefaultTransport}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	list, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	return cs, len(list.Tools), tokens.Count(cs.InitializeResult().Instructions) + tokens.Count(asJSON(list.Tools))
}

func callText(t *testing.T, cs *sdk.ClientSession, name string, args map[string]any) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	res, err := cs.CallTool(ctx, &sdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("call %s: %v", name, err)
	}
	var parts []string
	for _, c := range res.Content {
		if tc, ok := c.(*sdk.TextContent); ok {
			parts = append(parts, tc.Text)
		}
	}
	text := strings.Join(parts, "\n")
	if res.IsError {
		t.Fatalf("%s failed: %s", name, clip(text, 400))
	}
	return text
}

func side(tools, list int, results []string) benchSide {
	s := benchSide{Tools: tools, ListTokens: list, Calls: len(results)}
	for _, r := range results {
		s.CallTokens += tokens.Count(r)
	}
	s.Total = s.ListTokens + s.CallTokens
	return s
}

func question(q string, ep, tk benchSide) benchQuestion {
	return benchQuestion{Question: q, Endpoints: ep, Tasks: tk, SavingPct: int(math.Round(100 * (1 - float64(tk.Total)/float64(ep.Total))))}
}

func init() {
	add("benchmark", "token benchmark reproduces the committed numbers", func(t *testing.T) {
		// Fresh mock servers, so other scenarios' writes don't change the data.
		env, err := mockenv.Start()
		if err != nil {
			t.Fatal(err)
		}
		defer env.Close()
		h.importURL(t, "benchAnchore", env.URLs["anchore"]+"/v2/swagger.json")
		an := h.apiByName(t, "benchAnchore")
		h.setVar(t, an.ID, "username", mockenv.AnchoreUser, false)
		h.setVar(t, an.ID, "password", mockenv.AnchorePass, true)
		h.importURL(t, "benchJira", env.URLs["jira"]+"/openapi.json")
		ji := h.apiByName(t, "benchJira")
		h.setVar(t, ji.ID, "username", mockenv.JiraEmail, false)
		h.setVar(t, ji.ID, "password", mockenv.JiraToken, true)

		// Endpoint style: one generated tool per read request, whole response.
		ep := kit(t, "benchAnchore", "benchJira")
		names := map[string]string{}
		for _, a := range []apiDetail{h.apiByName(t, "benchAnchore"), h.apiByName(t, "benchJira")} {
			prefix := map[string]string{"benchAnchore": "anchore_", "benchJira": "jira_"}[a.Name]
			sort.Slice(a.Requests, func(i, j int) bool { return a.Requests[i].Name < a.Requests[j].Name })
			for _, r := range a.Requests {
				if r.Method != "GET" && !strings.Contains(r.Name, "Jql") {
					continue
				}
				name := prefix + strings.ToLower(snake(r.Name))
				if len(name) > 64 {
					name = name[:64]
				}
				h.mustCLI(t, "tool", "generate", ep, "--from", a.Name+"."+r.Name, "--name", name)
				names[a.Name+"."+r.Name] = name
			}
		}
		for _, need := range []string{"benchAnchore.listImages", "benchAnchore.getImageVulnerabilitiesByType", "benchJira.searchForIssuesUsingJql"} {
			if names[need] == "" {
				t.Fatalf("no endpoint tool for %s; generated %v", need, names)
			}
		}

		// Task style: one tool per question.
		tk := kit(t, "benchAnchore", "benchJira")
		h.saveTool(t, tk, `export const tool = {
  name: "find_fixable_criticals",
  description: "Critical OS vulnerabilities of one image that have a fix, highest CVSS first: CVE, package and fixed version.",
  useWhen: "Someone asks what to patch in an image",
  inputSchema: { type: "object", properties: { image_tag: { type: "string", description: "Full tag, e.g. docker.io/library/nginx:1.27" } }, required: ["image_tag"] },
};

export default async function (input: any) {
  const imgs = await benchAnchore.listImages({});
  const im = imgs.items.find((i: any) => i.image_detail[0].full_tag === input.image_tag);
  if (!im) throw new Error("No image tagged " + input.image_tag + "; check the full tag including the registry");
  const v = await benchAnchore.getImageVulnerabilitiesByType({ image_digest: im.image_digest, vtype: "os" });
  return v.vulnerabilities.filter((x: any) => x.severity === "Critical" && x.fix !== "None")
    .sort((a: any, b: any) => b.nvd_data[0].cvss_v3.base_score - a.nvd_data[0].cvss_v3.base_score)
    .map((x: any) => ({ cve: x.vuln, pkg: x.package_name, fix: x.fix }));
}
`)
		h.saveTool(t, tk, `export const tool = {
  name: "open_ops_issues",
  description: "Open issues in a Jira project: key, summary, status and assignee, newest first.",
  useWhen: "Someone asks what is open in a project",
  inputSchema: { type: "object", properties: { project_key: { type: "string", description: "Jira project key, e.g. OPS" }, limit: { type: "integer", description: "How many, default 20" } }, required: ["project_key"] },
};

export default async function (input: any) {
  const r = await benchJira.searchForIssuesUsingJql({ jql: "project = " + input.project_key + " AND statusCategory != Done", maxResults: input.limit || 20 });
  return { total: r.total, issues: r.issues.map((i: any) => ({ key: i.key, summary: i.fields.summary, status: i.fields.status.name, assignee: i.fields.assignee ? i.fields.assignee.displayName : null })) };
}
`)

		epCS, epTools, epList := mcpSession(t, "/mcp/"+ep, h.connect(t, ep, "read", false))
		tkCS, tkTools, tkList := mcpSession(t, "/mcp/"+tk, h.connect(t, tk, "read", false))
		var got benchResults
		got.Encoding = "cl100k_base"
		got.Method = "server instructions + tools/list + every tool result the agent reads; mock systems from e2e/mockenv"

		// Q1: the endpoint agent lists images, finds the digest, then fetches all OS vulnerabilities.
		const tag = "docker.io/library/nginx:1.27"
		imgs := callText(t, epCS, names["benchAnchore.listImages"], map[string]any{})
		var list struct {
			Items []struct {
				Digest string `json:"image_digest"`
				Detail []struct {
					Tag string `json:"full_tag"`
				} `json:"image_detail"`
			} `json:"items"`
		}
		if err := json.Unmarshal([]byte(imgs), &list); err != nil {
			t.Fatalf("listImages output isn't JSON: %v: %s", err, clip(imgs, 300))
		}
		digest := ""
		for _, it := range list.Items {
			if len(it.Detail) > 0 && it.Detail[0].Tag == tag {
				digest = it.Digest
			}
		}
		expect(t, digest != "", "no %s in %s", tag, clip(imgs, 300))
		vulns := callText(t, epCS, names["benchAnchore.getImageVulnerabilitiesByType"], map[string]any{"image_digest": digest, "vtype": "os"})
		ans := callText(t, tkCS, "find_fixable_criticals", map[string]any{"image_tag": tag})
		expect(t, strings.Contains(ans, "CVE-"), "task tool found no CVEs: %s", ans)
		got.Questions = append(got.Questions, question("Which critical OS vulnerabilities in nginx:1.27 have a fix?",
			side(epTools, epList, []string{imgs, vulns}), side(tkTools, tkList, []string{ans})))

		// Q2: open issues in one Jira project.
		search := callText(t, epCS, names["benchJira.searchForIssuesUsingJql"], map[string]any{"jql": "project = OPS AND statusCategory != Done", "maxResults": 20})
		ans = callText(t, tkCS, "open_ops_issues", map[string]any{"project_key": "OPS"})
		got.Questions = append(got.Questions, question("What is open in the OPS project?",
			side(epTools, epList, []string{search}), side(tkTools, tkList, []string{ans})))

		// The builder endpoint's fixed surface, for agents that write tools.
		_, got.Builder.Tools, got.Builder.ListTokens = mcpSession(t, "/mcp/"+tk+"/build", h.connect(t, tk, "read", true))

		b, _ := json.MarshalIndent(got, "", "  ")
		t.Logf("benchmark:\n%s", b)
		if *benchOut != "" {
			if err := os.WriteFile(*benchOut, append(b, '\n'), 0o644); err != nil {
				t.Fatal(err)
			}
			return
		}
		raw, err := os.ReadFile("../../bench/results.json")
		if err != nil {
			t.Fatalf("no committed benchmark; run make bench: %v", err)
		}
		var want benchResults
		if err := json.Unmarshal(raw, &want); err != nil {
			t.Fatal(err)
		}
		near := func(a, b int) bool { return math.Abs(float64(a-b)) <= 0.02*math.Max(float64(b), 50) }
		expect(t, len(want.Questions) == len(got.Questions), "questions changed; run make bench")
		for i, q := range got.Questions {
			w := want.Questions[i]
			expect(t, near(q.Endpoints.Total, w.Endpoints.Total) && near(q.Tasks.Total, w.Tasks.Total),
				"%q: got %d vs %d tokens, committed %d vs %d; run make bench and commit bench/results.json",
				q.Question, q.Endpoints.Total, q.Tasks.Total, w.Endpoints.Total, w.Tasks.Total)
		}
		expect(t, near(got.Builder.ListTokens, want.Builder.ListTokens), "builder surface is %d tokens, committed %d; run make bench", got.Builder.ListTokens, want.Builder.ListTokens)
	})
}

// snake turns listImages into list_images.
func snake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteByte('_')
			}
			r += 'a' - 'A'
		}
		if r == '-' || r == '.' || r == '/' {
			r = '_'
		}
		b.WriteRune(r)
	}
	return b.String()
}
