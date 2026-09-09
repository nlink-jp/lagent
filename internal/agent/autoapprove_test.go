package agent

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nlink-jp/lagent/internal/llm"
	"github.com/nlink-jp/lagent/internal/risk"
	"github.com/nlink-jp/lagent/internal/tools"
)

// autoBackend answers tool rounds from a script and risk-eval rounds
// (identified by the absence of tool definitions) from a fixed verdict.
type autoBackend struct {
	responses   []*llm.Response
	verdict     string
	verdictErr  error
	evals       []string // the payloads the risk evaluator saw
	evalSystems []string // the system prompts it saw (gem-agent ADR-0038 variants)
}

func (b *autoBackend) ChatStream(ctx context.Context, system string, msgs []llm.Message, defs []llm.ToolDef, onText func(string)) (*llm.Response, error) {
	// The auto ceiling's per-turn question is a side call like the risk
	// tier's (gem-agent ADR-0080 §2). Answered false so it changes nothing here;
	// the tests that exercise it use their own backend.
	if len(defs) == 0 && strings.Contains(system, "changes nothing") {
		return &llm.Response{Content: `{"read_only": false}`}, nil
	}
	if len(defs) == 0 && strings.Contains(system, "security reviewer") {
		if b.verdictErr != nil {
			return nil, b.verdictErr
		}
		if len(msgs) > 0 {
			b.evals = append(b.evals, msgs[0].Content)
			b.evalSystems = append(b.evalSystems, system)
		}
		return &llm.Response{Content: b.verdict}, nil
	}
	if len(b.responses) == 0 {
		return &llm.Response{Content: "(exhausted)"}, nil
	}
	r := b.responses[0]
	b.responses = b.responses[1:]
	return r, nil
}

type recordingGate struct{ asked []string }

func (g *recordingGate) ApproveLift(name, detail, purpose, reason string) (bool, string) {
	g.asked = append(g.asked, name+"|"+detail+"|"+reason)
	return false, ""
}

func (g *recordingGate) Approve(name, detail, purpose, reason string, mustPrompt bool) (bool, bool, string) {
	g.asked = append(g.asked, name+"|"+detail+"|"+reason)
	return false, false, "" // deny: tests assert on whether the gate was reached
}

func newAutoAgent(t *testing.T, b *autoBackend, gate Approver) (*Agent, *tools.Registry, *[]AutoDecision) {
	t.Helper()
	reg, err := tools.New(t.TempDir(),
		func(ctx context.Context, c string) *exec.Cmd { return exec.CommandContext(ctx, "/bin/bash", "-c", c) },
		5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	var decisions []AutoDecision
	a := New(Options{
		Backend: b, Registry: reg, Gate: gate, System: "sys", MaxTurns: 5,
		AutoApprove:    true,
		OnAutoDecision: func(tc llm.ToolCall, d AutoDecision) { decisions = append(decisions, d) },
	})
	return a, reg, &decisions
}

func writeCall(path string) llm.ToolCall {
	return llm.ToolCall{ID: "c1", Name: "write_file",
		Args: map[string]any{"path": path, "content": "x"}}
}

func shellCall(command string) llm.ToolCall {
	return llm.ToolCall{ID: "c1", Name: "shell_exec", Args: map[string]any{"command": command}}
}

func TestAutoSafeRunsWithoutModelOrGate(t *testing.T) {
	b := &autoBackend{responses: []*llm.Response{
		{ToolCalls: []llm.ToolCall{writeCall("src/main.go")}},
		{Content: "done"},
	}}
	gate := &recordingGate{}
	a, reg, decisions := newAutoAgent(t, b, gate)

	if _, err := a.Run(context.Background(), "write it", nil); err != nil {
		t.Fatal(err)
	}
	if len(gate.asked) != 0 {
		t.Errorf("safe call should not reach the gate: %v", gate.asked)
	}
	if len(b.evals) != 0 {
		t.Error("safe call must not spend a model round")
	}
	if d := (*decisions)[0]; !d.Approved || d.Tier != risk.Safe || d.ModelConsulted {
		t.Errorf("decision = %+v", d)
	}
	if _, err := os.Stat(filepath.Join(reg.ProjectDir(), "src/main.go")); err != nil {
		t.Errorf("tool did not run: %v", err)
	}
}

// TestAutoBlockNeverConsultsModel is the gem-agent ADR-0004 floor: a Block verdict
// reaches the human even if the model would have approved.
func TestAutoBlockNeverConsultsModel(t *testing.T) {
	b := &autoBackend{
		responses: []*llm.Response{
			{ToolCalls: []llm.ToolCall{shellCall("rm -rf /")}},
			{Content: "ok"},
		},
		verdict: `{"approve": true, "confidence": 1.0, "reason": "looks fine"}`,
	}
	gate := &recordingGate{}
	a, _, decisions := newAutoAgent(t, b, gate)

	if _, err := a.Run(context.Background(), "clean up", nil); err != nil {
		t.Fatal(err)
	}
	if len(b.evals) != 0 {
		t.Error("block tier must not consult the model at all")
	}
	if len(gate.asked) != 1 {
		t.Fatalf("block tier must reach the human gate: %v", gate.asked)
	}
	// The escalation must name the tier that objected as well as why:
	// "blocked by rule" reads differently from a model judgment call.
	if !strings.Contains(gate.asked[0], "delete") || !strings.Contains(gate.asked[0], "blocked by rule") {
		t.Errorf("escalation should carry tier and reason: %q", gate.asked[0])
	}
	if d := (*decisions)[0]; d.Approved || d.Tier != risk.Block || d.ModelConsulted {
		t.Errorf("decision = %+v", d)
	}
}

// TestOrdinaryPromptCarriesNoReason: with auto mode off there is no
// escalation to explain, so the prompt must not invent one.
func TestOrdinaryPromptCarriesNoReason(t *testing.T) {
	b := &autoBackend{responses: []*llm.Response{
		{ToolCalls: []llm.ToolCall{writeCall("src/main.go")}},
		{Content: "ok"},
	}}
	gate := &recordingGate{}
	a, _, _ := newAutoAgent(t, b, gate)
	a.SetAutoApprove(false)

	if _, err := a.Run(context.Background(), "write", nil); err != nil {
		t.Fatal(err)
	}
	if len(gate.asked) != 1 {
		t.Fatal("auto off must still ask")
	}
	if !strings.HasSuffix(gate.asked[0], "|") {
		t.Errorf("ordinary prompt should carry an empty reason: %q", gate.asked[0])
	}
}

func TestAutoOffKeepsEveryMutatingCallGated(t *testing.T) {
	b := &autoBackend{responses: []*llm.Response{
		{ToolCalls: []llm.ToolCall{writeCall("src/main.go")}},
		{Content: "ok"},
	}}
	gate := &recordingGate{}
	a, _, _ := newAutoAgent(t, b, gate)
	a.SetAutoApprove(false)

	if _, err := a.Run(context.Background(), "write", nil); err != nil {
		t.Fatal(err)
	}
	if len(gate.asked) != 1 {
		t.Error("with auto mode off, even a safe mutating call must ask")
	}
	if len(b.evals) != 0 {
		t.Error("auto mode off must not spend model rounds")
	}
}

func TestAutoToggle(t *testing.T) {
	b := &autoBackend{}
	a, _, _ := newAutoAgent(t, b, &recordingGate{})
	if !a.AutoApprove() {
		t.Fatal("constructed with AutoApprove: true")
	}
	a.SetAutoApprove(false)
	if a.AutoApprove() {
		t.Fatal("toggle off failed")
	}
}

// TestEmptyResponseErrorNamesTheCause: "the model returned nothing" is
// not actionable — a blocked prompt, an exhausted output budget, and a
// safety stop look identical without the reason the API reported.
func TestEmptyResponseErrorNamesTheCause(t *testing.T) {
	cases := []struct {
		resp llm.Response
		want string
	}{
		{llm.Response{FinishReason: "length", ThoughtTokens: 4096}, "output limit"},
		{llm.Response{FinishReason: "content_filter"}, "content_filter"},
		{llm.Response{}, "no usable response"},
	}
	for _, c := range cases {
		resp := c.resp
		if err := emptyResponseError(&resp); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%+v -> %v, want mention of %q", c.resp, err, c.want)
		}
	}
}

// Review is the operator's: with no model tier here, a Review verdict
// goes to the gate, and the backend is never asked for a judgment.
func TestAutoReviewGoesToTheGateWithoutAModelCall(t *testing.T) {
	b := &autoBackend{responses: []*llm.Response{
		{ToolCalls: []llm.ToolCall{{ID: "c", Name: "shell_exec", Args: map[string]any{"command": "go test ./...", "access": "write"}}}},
		{Content: "ok"},
	}, verdict: `{"approve": true, "confidence": 0.99, "reason": "would approve"}`}
	gate := &recordingGate{}
	a, _, decisions := newAutoAgent(t, b, gate)
	if _, err := a.Run(context.Background(), "test", nil); err != nil {
		t.Fatal(err)
	}
	if len(b.evals) != 0 {
		t.Fatalf("the backend was consulted as a risk reviewer: %q", b.evals)
	}
	if len(*decisions) != 1 || (*decisions)[0].Approved || (*decisions)[0].Tier != risk.Review || (*decisions)[0].ModelConsulted {
		t.Fatalf("decision = %+v, want an unapproved Review with no model", *decisions)
	}
	if len(gate.asked) != 1 || !strings.Contains(gate.asked[0], "auto-approve escalated") {
		t.Fatalf("gate.asked = %v", gate.asked)
	}
}
