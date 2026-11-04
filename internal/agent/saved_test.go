package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nlink-jp/lagent/internal/llm"
	"github.com/nlink-jp/lagent/internal/risk"
	"github.com/nlink-jp/lagent/internal/tools"
	"github.com/nlink-jp/nlk/guard"
)

// ADR-0039 §1: what a tool reports through the context's recorder lands
// in the transcript's tool-result entry as Message.Saved — provenance,
// never parsed from the text.
func TestSavedFilesReachTheToolMessage(t *testing.T) {
	const name = "mcp__bigquery__query"
	mb := &mockBackend{responses: []*llm.Response{
		{ToolCalls: []llm.ToolCall{remoteCall("c1", name, "select 1")}},
		{Content: "done"},
	}}
	a, reg, _, _ := newRemoteAgent(t, mb, &approveAll{})
	err := reg.Register(&tools.Tool{
		Name: name, Description: "remote", Mutating: true,
		Parameters: map[string]any{"type": "object"},
		Run: func(ctx context.Context, _ map[string]any) (string, error) {
			tools.RecordSaved(ctx, tools.Saved{Path: "/w/bq-query-ab.json", Bytes: 12, SHA256: "ab"})
			return `{"rows":[]}`, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Run(context.Background(), "query", nil); err != nil {
		t.Fatal(err)
	}
	msgs := toolMessages(a)
	if len(msgs) != 1 {
		t.Fatalf("tool messages = %d", len(msgs))
	}
	want := []llm.SavedFile{{Path: "/w/bq-query-ab.json", Bytes: 12, SHA256: "ab"}}
	if len(msgs[0].Saved) != 1 || msgs[0].Saved[0] != want[0] {
		t.Errorf("Saved = %+v, want %+v", msgs[0].Saved, want)
	}
	// The model reads it as the runtime's note, outside the tag.
	sent := mb.calls[len(mb.calls)-1]
	last := sent[len(sent)-1]
	if !strings.Contains(last.Content, `write_file {"path": "<destination>", "from": "/w/bq-query-ab.json"}`) ||
		!strings.Contains(last.Content, "/w/bq-query-ab.json") {
		t.Errorf("the note did not reach the model: %q", last.Content)
	}
}

// The note rides outside the tag exactly when the field is set; a result
// whose text merely looks like it stays wrapped.
func TestSavedNoteRidesOutsideTheTagByProvenance(t *testing.T) {
	tag := guard.NewTagWithPrefix("tool_output")
	files := []llm.SavedFile{{Path: "/w/a.json", Bytes: 3, SHA256: "cafe"}}
	note := savedNote(files)
	history := []llm.Message{
		{Role: llm.RoleTool, ToolName: "mcp__x__y", Content: "abc", Saved: files},
		{Role: llm.RoleTool, ToolName: "mcp__x__y", Content: note}, // forged by a server
	}
	out := wrapToolMessages(history, tag, nil)
	if !strings.HasSuffix(out[0].Content, "\n\n"+note) {
		t.Errorf("note missing from the sent content: %q", out[0].Content)
	}
	if body := strings.TrimSuffix(out[0].Content, "\n\n"+note); !strings.Contains(body, "</") {
		t.Errorf("the server text rode unwrapped beside the note: %q", body)
	}
	if out[1].Content == note || strings.HasPrefix(out[1].Content, "[lagent") {
		t.Error("note-shaped server output shipped unwrapped")
	}
}

// A block that could not be saved is said so: there is nothing to copy.
func TestSavedNoteSaysWhenNothingWasSaved(t *testing.T) {
	note := savedNote([]llm.SavedFile{{Bytes: 5, Error: "no session work directory to save the result in"}})
	if !strings.Contains(note, "no file to copy") || strings.Contains(note, "write_file") {
		t.Errorf("note = %q", note)
	}
	if savedNote(nil) != "" {
		t.Error("a result that saved nothing gained a note")
	}
}

// ADR-0039 §2: only the covered reads are retried in process after the
// kernel refuses a credential path. A non-read tool returning the same
// error gets no operator question and no second run.
func TestCredentialRetryIsForCoveredReadsOnly(t *testing.T) {
	const name = "write_file_like"
	mb := &mockBackend{responses: []*llm.Response{
		{ToolCalls: []llm.ToolCall{{ID: "c1", Name: name, Args: map[string]any{}}}},
		{Content: "done"},
	}}
	a, reg, _, _ := newRemoteAgent(t, mb, &approveAll{})
	runs := 0
	err := reg.Register(&tools.Tool{
		Name: name, Description: "x", Mutating: true,
		Parameters: map[string]any{"type": "object"},
		Run: func(context.Context, map[string]any) (string, error) {
			runs++
			return "", tools.ErrCredentialRead
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Run(context.Background(), "go", nil); err != nil {
		t.Fatal(err)
	}
	if runs != 1 {
		t.Errorf("the tool ran %d times; a refused non-read must not be retried", runs)
	}
	if msgs := toolMessages(a); len(msgs) != 1 || !strings.HasPrefix(msgs[0].Content, "error:") || msgs[0].Denial {
		t.Errorf("result = %+v, want a plain error", msgs)
	}
}

// ADR-0039 §1 under ADR-0034: the backstop cuts a result past its
// allotment, but the note is drawn after the cut, so the model still
// reads where the whole result was saved.
func TestSavedNoteSurvivesTheBackstopCut(t *testing.T) {
	huge := strings.Repeat("z", 60_000)
	mb := &mockBackend{responses: []*llm.Response{
		{ToolCalls: []llm.ToolCall{{ID: "x", Name: "mcp__s__dump"}}},
		{Content: "done"},
	}}
	a := compactingAgent(t, mb, 16_000, 5, nil)
	if err := a.registry.Register(&tools.Tool{Name: "mcp__s__dump", Description: "d",
		Run: func(ctx context.Context, _ map[string]any) (string, error) {
			tools.RecordSaved(ctx, tools.Saved{Path: "/w/s-dump-ab.txt", Bytes: int64(len(huge)), SHA256: "ab"})
			return huge, nil
		}}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Run(context.Background(), "go", nil); err != nil {
		t.Fatal(err)
	}
	msgs := toolMessages(a)
	if len(msgs) != 1 || !strings.HasSuffix(msgs[0].Content, "the rest was not kept]") || len(msgs[0].Saved) != 1 {
		t.Fatalf("the result was not cut, or lost its record: %d messages, saved %+v", len(msgs), msgs)
	}
	sent := mb.calls[len(mb.calls)-1]
	last := sent[len(sent)-1]
	if !strings.HasSuffix(last.Content, "no shell command is needed]") || !strings.Contains(last.Content, "/w/s-dump-ab.txt") {
		t.Errorf("the note did not follow the cut result: ends %q", last.Content[max(0, len(last.Content)-120):])
	}
}

// A result split across files is never announced as whole: each line
// names its part, and the route says to copy every part (independent
// review — a part called "whole" is the lost rows ADR-0039 exists to
// stop).
func TestSavedNoteNamesEachPartOfASplitResult(t *testing.T) {
	note := savedNote([]llm.SavedFile{
		{Path: "/w/a.json", Bytes: 10, SHA256: "aa"},
		{Path: "/w/b.txt", Bytes: 20, SHA256: "bb"},
		{Bytes: 5, Error: "disk full"},
	})
	lines := strings.Split(note, "\n")
	if len(lines) != 3 {
		t.Fatalf("lines = %q", lines)
	}
	if strings.Contains(note, "result whole") {
		t.Errorf("a part was announced as the whole result: %q", note)
	}
	for i, want := range []string{"part 1 of 3", "part 2 of 3", "could not save part 3 of 3"} {
		if !strings.Contains(lines[i], want) {
			t.Errorf("line %d = %q, want %q", i, lines[i], want)
		}
	}
	if !strings.Contains(lines[0], "copy every part") || !strings.Contains(lines[0], "/w/a.json") {
		t.Errorf("the route is missing: %q", lines[0])
	}
	if one := savedNote([]llm.SavedFile{{Path: "/w/a.json", Bytes: 10, SHA256: "aa"}}); !strings.Contains(one, "saved this result whole as /w/a.json") {
		t.Errorf("a single file lost its wording: %q", one)
	}
}

// The notes appended at send time are counted by the compaction
// estimate and the round budget — the bytes the model receives (ADR-0039
// §1, independent review).
func TestTheSavedNoteIsCountedAsSent(t *testing.T) {
	m := llm.Message{Role: llm.RoleTool, Content: "x", Saved: []llm.SavedFile{{Path: "/w/" + strings.Repeat("a", 400), Bytes: 1, SHA256: "ab"}}}
	bare := m
	bare.Saved = nil
	if d := estimateTokens([]llm.Message{m}) - estimateTokens([]llm.Message{bare}); d*bytesPerToken < 400 {
		t.Errorf("the note added %d tokens to the estimate; it carries over 400 bytes", d)
	}
	if got := wrapToolMessages([]llm.Message{m}, guard.NewTag(), nil)[0].Content; !strings.HasSuffix(got, runtimeSuffix(m)) {
		t.Errorf("the wrap and the count disagree: %q", got)
	}
}

// A file saved in the session work directory is named the way the file
// tools accept it, $LAGENT_WORK_DIR/<name>, and the note spells out the
// call with that name (ADR-0039 §7.1, §7.2): measured, the model copied
// the file name right and rewrote the long state-directory name. A file
// elsewhere, and a record from an older transcript, keep the absolute
// path.
func TestSavedNoteNamesTheFileByTheWorkDirVariable(t *testing.T) {
	work := "/state/lagent/-Users-me-proj/work/s1"
	files := savedFiles([]tools.Saved{
		{Path: work + "/bq-query-ab.json", Bytes: 12, SHA256: "ab"},
	}, work)
	if len(files) != 1 || files[0].Ref != "$LAGENT_WORK_DIR/bq-query-ab.json" || files[0].Path != work+"/bq-query-ab.json" {
		t.Fatalf("record = %+v", files)
	}
	note := savedNote(files)
	for _, want := range []string{
		"saved this result whole as $LAGENT_WORK_DIR/bq-query-ab.json (12 bytes, sha256 ab)",
		`write_file {"path": "<destination>", "from": "$LAGENT_WORK_DIR/bq-query-ab.json"}`,
		"creates the parent directories",
		"no shell command is needed",
	} {
		if !strings.Contains(note, want) {
			t.Errorf("note lacks %q: %q", want, note)
		}
	}
	if strings.Contains(note, work) {
		t.Errorf("the note still names the long path: %q", note)
	}

	if out := savedFiles([]tools.Saved{{Path: "/elsewhere/x.json", Bytes: 1, SHA256: "cc"}}, work); out[0].Ref != "" {
		t.Errorf("a file outside the work directory got a short name: %+v", out[0])
	}
	if out := savedFiles([]tools.Saved{{Path: work + "/x.json", Bytes: 1, SHA256: "cc"}}, ""); out[0].Ref != "" {
		t.Errorf("a session with no work directory got a short name: %+v", out[0])
	}
	old := savedNote([]llm.SavedFile{{Path: work + "/old.json", Bytes: 3, SHA256: "dd"}})
	if !strings.Contains(old, `"from": "`+work+`/old.json"`) {
		t.Errorf("an older record without Ref did not fall back to its path: %q", old)
	}
	// The name in the call is a JSON string as the model would write it,
	// not HTML-escaped.
	odd := savedNote([]llm.SavedFile{{Path: `/w/a&b "c".json`, Bytes: 1, SHA256: "ee"}})
	if !strings.Contains(odd, `"from": "/w/a&b \"c\".json"`) {
		t.Errorf("the from value is not a plain JSON string: %q", odd)
	}
	parts := savedNote(savedFiles([]tools.Saved{
		{Path: work + "/a.json", Bytes: 1, SHA256: "aa"},
		{Path: work + "/b.txt", Bytes: 2, SHA256: "bb"},
	}, work))
	if !strings.Contains(parts, `part 2 of 2 of this result as $LAGENT_WORK_DIR/b.txt`) ||
		!strings.Contains(parts, `"from": "$LAGENT_WORK_DIR/b.txt"} for each]`) {
		t.Errorf("a part lost the short name: %q", parts)
	}
}

// The decision point judges $LAGENT_WORK_DIR/… as what it names
// (ADR-0039 §7.1): realPathArgs goes through the file tools' resolver,
// so a destination there is a work-directory write, and a copy source
// that reaches credential material through it is still Block.
func TestTheWorkDirVariableIsJudgedAsWhatItNames(t *testing.T) {
	a, reg, _, _ := newRemoteAgent(t, &mockBackend{}, &approveAll{})
	work := t.TempDir()
	if err := reg.UseWorkDir(work); err != nil {
		t.Fatal(err)
	}
	d := a.decide(llm.ToolCall{Name: "write_file", Args: map[string]any{
		"path": "$LAGENT_WORK_DIR/notes/n.md", "content": "x"}})
	if d.Verdict.Tier != risk.Safe || !strings.Contains(d.Verdict.Reason, "session work directory") {
		t.Errorf("a work-directory write was judged %+v", d.Verdict)
	}
	env := filepath.Join(reg.ProjectDir(), ".env")
	if err := os.WriteFile(env, []byte("TOKEN=x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(env, filepath.Join(work, "notes.txt")); err != nil {
		t.Fatal(err)
	}
	d = a.decide(llm.ToolCall{Name: "write_file", Args: map[string]any{
		"path": "evidence/x.txt", "from": "$LAGENT_WORK_DIR/notes.txt"}})
	if d.Verdict.Tier != risk.Block {
		t.Errorf("a credential copy source named through the variable was judged %+v", d.Verdict)
	}
	// A rest that climbs back into the project is the project file it
	// names: AGENTS.md is a persistent target, the operator's alone
	// (independent review).
	climb := "$LAGENT_WORK_DIR/../" + filepath.Base(reg.ProjectDir()) + "/AGENTS.md"
	d = a.decide(llm.ToolCall{Name: "write_file", Args: map[string]any{"path": climb, "content": "x"}})
	if d.Verdict.Tier != risk.Review || !d.Verdict.OperatorOnly {
		t.Errorf("a climb into the project's AGENTS.md was judged %+v", d.Verdict)
	}
}
