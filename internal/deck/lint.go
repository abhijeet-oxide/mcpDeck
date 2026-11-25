package deck

import (
	"context"
	"encoding/json"

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

func lintTool(v store.ToolVersion) lint.Tool {
	t := lint.Tool{Name: v.Name, Description: v.Description, UseWhen: v.UseWhen, DontUseWhen: v.DontUseWhen,
		InputSchema: v.InputSchema, Source: v.Source, Calls: v.Calls, Access: v.Access}
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
				InputSchema: def.InputSchema, Source: in.Draft, Calls: calls}
			draftName = def.Name
		}
	}
	ts, _ := d.Store.ToolkitTools(ctx, tk.ID)
	for _, t := range ts {
		vs, _ := d.Store.Versions(ctx, t.ID)
		if len(vs) == 0 || (draft != nil && vs[0].Name == draftName) {
			continue
		}
		tools = append(tools, lintTool(vs[0]))
	}
	if draft != nil {
		tools = append(tools, *draft)
	}
	out.Report = lint.Toolkit(tools)
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
