package deck

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	"github.com/abhijeet-oxide/mcpdeck/internal/lint"
	"github.com/abhijeet-oxide/mcpdeck/internal/store"
	"github.com/abhijeet-oxide/mcpdeck/internal/tool"
)

// LintInput optionally includes an unsaved tool, so the editor can show what
// saving it would do to the toolkit (GUIDE-2).
type LintInput struct {
	Draft string `json:"draft"`
}

// LintResult is the toolkit report, plus the draft's own findings.
type LintResult struct {
	lint.Report
	Draft      []lint.Finding `json:"draft"`
	DraftError string         `json:"draftError,omitempty"`
}

func (d *Deck) lintTool(ctx context.Context, v store.ToolVersion) lint.Tool {
	t := lint.Tool{Name: v.Name, Description: v.Description, UseWhen: v.UseWhen, DontUseWhen: v.DontUseWhen,
		InputSchema: v.InputSchema, Source: v.Source, Calls: v.Calls, Access: v.Access,
		Methods: d.callMethods(ctx, v.Calls), Annotations: d.annotations(v)}
	for _, r := range v.TestResults {
		if r.Tokens > t.OutputTokens {
			t.OutputTokens = r.Tokens
		}
	}
	return t
}

// LintToolkit checks a toolkit against the MCP guidelines (GUIDE-2).
func (d *Deck) LintToolkit(ctx context.Context, ref string, in LintInput) (*LintResult, error) {
	tk, err := d.toolkitByRef(ctx, ref)
	if err != nil {
		return nil, err
	}
	var tools []lint.Tool
	draftName := ""
	out := &LintResult{Draft: []lint.Finding{}}
	var draft *lint.Tool
	if in.Draft != "" {
		def, err := tool.Load(in.Draft)
		if err != nil && def.Name == "" {
			out.DraftError = err.Error()
		} else {
			calls, _ := tool.UsedCalls(in.Draft, d.clients(ctx, nil))
			draft = &lint.Tool{Name: def.Name, Description: def.Description, UseWhen: def.UseWhen, DontUseWhen: def.DontUseWhen,
				InputSchema: def.InputSchema, Source: in.Draft, Calls: calls, Access: d.accessOf(ctx, in.Draft, calls),
				Methods: d.callMethods(ctx, calls), Annotations: def.Annotations}
			draftName = def.Name
		}
	}
	ts, _ := d.Store.ToolkitTools(ctx, tk.ID)
	for _, t := range ts {
		vs, _ := d.Store.Versions(ctx, t.ID)
		if len(vs) == 0 || (draft != nil && vs[0].Name == draftName) {
			continue
		}
		tools = append(tools, d.lintTool(ctx, vs[0]))
	}
	if draft != nil {
		tools = append(tools, *draft)
	}
	out.Report = lint.Toolkit(tools)
	if draft == nil {
		// The real tools/list an agent receives, so every page shows one number.
		out.ListTokens = d.ListTokens(ctx, tk, "read_write", false)
	}
	if draft != nil {
		kept := []lint.Finding{}
		for _, f := range out.Findings {
			if f.Tool == draftName {
				out.Draft = append(out.Draft, f)
			} else {
				kept = append(kept, f)
			}
		}
		out.Findings = kept
	}
	return out, nil
}

// LintOutput analyses a response or tool output (GUIDE-3).
func (d *Deck) LintOutput(raw json.RawMessage) (*lint.OutputReport, error) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, badRequest("only JSON can be analysed")
	}
	r := lint.Output(v)
	return &r, nil
}

// callMethods are the HTTP methods of the requests a tool calls.
func (d *Deck) callMethods(ctx context.Context, calls []string) []string {
	byFull := map[string]string{}
	for _, cl := range d.clients(ctx, nil) {
		for _, r := range cl.Requests {
			byFull[cl.API.Name+"."+r.Name] = strings.ToUpper(r.Method)
		}
	}
	var out []string
	for _, k := range calls {
		if m, ok := byFull[k]; ok {
			out = append(out, m)
		} else if parts := strings.Fields(k); len(parts) >= 2 {
			out = append(out, strings.ToUpper(parts[1]))
		}
	}
	return out
}

// annotations are the hints a saved version's file declares (cached by version).
func (d *Deck) annotations(v store.ToolVersion) *lint.Annotations {
	if a, ok := annCache.Load(v.ID); ok {
		return a.(*lint.Annotations)
	}
	var a *lint.Annotations
	if strings.Contains(v.Source, "Hint") || strings.Contains(v.Source, "annotations") {
		if def, err := tool.Load(v.Source); err == nil {
			a = def.Annotations
		}
	}
	annCache.Store(v.ID, a)
	return a
}

var annCache sync.Map

// SaveFindings lints one tool as it would be saved into a toolkit (LINT-2).
func (d *Deck) saveFindings(ctx context.Context, tk store.Toolkit, v store.ToolVersion, def tool.Definition) []lint.Finding {
	lt := lint.Tool{Name: v.Name, Description: v.Description, UseWhen: v.UseWhen, DontUseWhen: v.DontUseWhen,
		InputSchema: v.InputSchema, Source: v.Source, Calls: v.Calls, Access: v.Access,
		Methods: d.callMethods(ctx, v.Calls), Annotations: def.Annotations}
	for _, r := range v.TestResults {
		if r.Tokens > lt.OutputTokens {
			lt.OutputTokens = r.Tokens
		}
	}
	n := 1
	if ts, err := d.Store.ToolkitTools(ctx, tk.ID); err == nil {
		n = len(ts) + 1
	}
	return lint.ToolDef(lt, n)
}
