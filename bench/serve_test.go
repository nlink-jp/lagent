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
		`data: {"choices":[],"usage":{"prompt_tokens":1200,"completion_tokens":11,"prompt_tokens_details":{"cached_tokens":1100}},"timings":{"prompt_ms":5.0}}`,
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
	if s.PromptTokens != 1200 || s.CachedTokens != 1100 || s.CompletionTokens != 11 || s.FinishReason != "length" {
		t.Errorf("sample %+v", s)
	}
	if s.DecodeTPS != 10 { // 10 tokens after the first, in one second
		t.Errorf("decode %v, want 10", s.DecodeTPS)
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
	if r := decodeRate(1, 1, 2); r != 0 {
		t.Errorf("one token has no rate: %v", r)
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
