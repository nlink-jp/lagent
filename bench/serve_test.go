package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A fake clock that advances one second per reading: the first content
// delta is read at t=1, the end at t=2 (role-only chunks do not count).
func TestServeSSEFirstTokenSkipsTheRoleChunk(t *testing.T) {
	stream := strings.Join([]string{
		`data: {"choices":[{"delta":{"role":"assistant","content":""}}]}`,
		`data: {"choices":[{"delta":{"content":"1"}}]}`,
		`data: {"choices":[{"delta":{"content":"\n2"}}]}`,
		`data: {"choices":[{"delta":{},"finish_reason":"length"}]}`,
		`data: {"choices":[],"usage":{"prompt_tokens":1200,"completion_tokens":21,"prompt_tokens_details":{"cached_tokens":1100}},"timings":{"prompt_ms":5.0}}`,
		`data: [DONE]`,
	}, "\n")
	start := time.Unix(0, 0)
	tick := start
	now := func() time.Time { tick = tick.Add(time.Second); return tick }
	var s serveSample
	if err := serveSSE(strings.NewReader(stream), start, now, &s); err != nil {
		t.Fatal(err)
	}
	if s.FirstTokenSec != 1 || s.TotalSec != 2 {
		t.Errorf("first %v total %v, want 1 and 2", s.FirstTokenSec, s.TotalSec)
	}
	if s.PromptTokens != 1200 || s.CachedTokens != 1100 || s.CompletionTokens != 21 || s.FinishReason != "length" {
		t.Errorf("sample %+v", s)
	}
	if s.DecodeTPS != 20 { // 20 tokens after the first, in one second
		t.Errorf("decode %v, want 20", s.DecodeTPS)
	}
	if string(s.Timings) != `{"prompt_ms":5.0}` {
		t.Errorf("timings %s", s.Timings)
	}
}

// A reasoning delta is the first token too: with thinking on, the
// server has finished reading the prompt when it starts to think.
func TestServeSSEReasoningCountsAsFirstToken(t *testing.T) {
	stream := `data: {"choices":[{"delta":{"reasoning_content":"hm"}}]}` + "\n" + `data: [DONE]`
	start := time.Unix(0, 0)
	tick := start
	now := func() time.Time { tick = tick.Add(time.Second); return tick }
	var s serveSample
	if err := serveSSE(strings.NewReader(stream), start, now, &s); err != nil {
		t.Fatal(err)
	}
	if s.FirstTokenSec != 1 {
		t.Errorf("first %v", s.FirstTokenSec)
	}
}

func TestDecodeRateGuards(t *testing.T) {
	if r := decodeRate(2, 0.611196, 0.611215); r != 0 {
		t.Errorf("a two-token reply has no rate (measured: it read as 48,000 tok/s): %v", r)
	}
	if r := decodeRate(10, 2, 2); r != 0 {
		t.Errorf("no time after the first token: %v", r)
	}
}

func TestServeSizes(t *testing.T) {
	got, err := serveSizes("1k, 12k,500")
	if err != nil || len(got) != 3 || got[0] != 1000 || got[1] != 12000 || got[2] != 500 {
		t.Errorf("%v %v", got, err)
	}
	for _, bad := range []string{"", "0k", "x"} {
		if _, err := serveSizes(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

// The padding is deterministic, skips _-prefixed directories (the
// results tree), and repeats the files when they are shorter than asked.
func TestPaddingIsDeterministicAndSkipsResults(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"b.go":          "package b\n",
		"a.go":          "package a\n",
		"_results/x.go": "package secret\n",
		"notes.txt":     "not go\n",
	} {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	one, err := padding(dir, 120)
	if err != nil {
		t.Fatal(err)
	}
	two, _ := padding(dir, 120)
	if one != two || len(one) != 120 {
		t.Fatalf("len %d, deterministic %v", len(one), one == two)
	}
	if strings.Contains(one, "secret") || strings.Contains(one, "not go") {
		t.Errorf("padding read outside .go sources: %q", one)
	}
	if !strings.HasPrefix(one, "=== a.go ===\npackage a\n\n=== b.go ===\npackage b\n\n=== a.go") {
		t.Errorf("want a.go then b.go, repeated, by relative path: %q", one)
	}
}

// The summary says how a measurement ended, so a decode the model
// stopped by itself is not read as generation speed at that size.
func TestServeSummaryShowsFinishAndHidesUntimedRates(t *testing.T) {
	got := serveSummary([]serveSample{
		{Label: "decode", Size: 35000, FinishReason: "stop", CompletionTokens: 2, FirstTokenSec: 1, TotalSec: 1.0001},
		{Label: "decode", Size: 35000, FinishReason: "length", CompletionTokens: 300, FirstTokenSec: 1, TotalSec: 5, DecodeTPS: 74.75},
		{Label: "next-turn", Size: 35000, FinishReason: "stop", CompletionTokens: 2},
	})
	if !strings.Contains(got, "| decode | 35000 | 2 | 0 | length 1, stop 1 |") {
		t.Errorf("finish column missing:\n%s", got)
	}
	if !strings.Contains(got, "| next-turn | 35000 | 1 | 0 | stop 1 | 0 | 0 | 0.00 | 0.00 | – |") {
		t.Errorf("an untimed rate must read as a dash:\n%s", got)
	}
}

func TestServeReportRecountsTheRawLog(t *testing.T) {
	dir := t.TempDir()
	log := `{"label":"decode","size":1000,"rep":1,"status":200,"prompt_tokens":977,"cached_tokens":0,"completion_tokens":2,"finish_reason":"stop","first_token_s":0.6,"total_s":0.6001,"decode_tps":19990}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "requests.jsonl"), []byte(log), 0o644); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := cmdServeReport([]string{dir}, &out); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "19990") || !strings.Contains(out.String(), "| – |") {
		t.Errorf("the old rate survived the recount:\n%s", out.String())
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "summary.md")); string(b) != out.String() {
		t.Error("summary.md not rewritten")
	}
}
