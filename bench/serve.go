package main

// bench serve measures the model server itself, not the runtime: how
// long a prompt of a given size takes to read cold, how much of that a
// resend or a next turn saves, how fast it generates at that size, and
// what two streams at once add up to. It is the instrument a backend
// decision quotes (ADR-0031); the task bench above measures the agent.
//
// Every request is written to <out>/requests.jsonl as it lands, so a
// summary can be recounted from the raw log.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// serveSample is one request as measured.
type serveSample struct {
	Label            string  `json:"label"`  // cold, resend, next-turn, decode, conv-N, pair-N
	Size             int     `json:"size"`   // the size class asked for, in tokens
	Rep              int     `json:"rep"`    // repetition, from 1
	Status           int     `json:"status"` // HTTP status; 0 when the request never got one
	Error            string  `json:"error,omitempty"`
	PromptTokens     int     `json:"prompt_tokens"`
	CachedTokens     int     `json:"cached_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	FinishReason     string  `json:"finish_reason"`
	FirstTokenSec    float64 `json:"first_token_s"` // request start to the first content/reasoning/tool delta
	TotalSec         float64 `json:"total_s"`
	// DecodeTPS is completion tokens after the first, over the time
	// after the first: generation speed with the prompt read excluded.
	DecodeTPS float64 `json:"decode_tps"`
	// Timings is the server's own accounting when it sends one
	// (mlx-serve does, beside usage); kept raw.
	Timings json.RawMessage `json:"timings,omitempty"`
}

// decodeRate is completion tokens after the first over the seconds
// after the first. Zero when there is nothing to divide.
func decodeRate(completion int, firstSec, totalSec float64) float64 {
	if completion < 2 || totalSec <= firstSec {
		return 0
	}
	return float64(completion-1) / (totalSec - firstSec)
}

// serveSSE reads a chat/completions stream and fills the sample. now
// is the clock (injected for tests); start is when the request left.
func serveSSE(r io.Reader, start time.Time, now func() time.Time, s *serveSample) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), 8<<20)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var ch struct {
			Choices []struct {
				Delta struct {
					Content          string            `json:"content"`
					ReasoningContent string            `json:"reasoning_content"`
					Reasoning        string            `json:"reasoning"`
					ToolCalls        []json.RawMessage `json:"tool_calls"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
			Usage *struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				Details          *struct {
					Cached int `json:"cached_tokens"`
				} `json:"prompt_tokens_details"`
			} `json:"usage"`
			Timings json.RawMessage `json:"timings"`
		}
		if err := json.Unmarshal([]byte(data), &ch); err != nil {
			return fmt.Errorf("stream: %w", err)
		}
		for _, c := range ch.Choices {
			d := c.Delta
			if s.FirstTokenSec == 0 && (d.Content != "" || d.ReasoningContent != "" || d.Reasoning != "" || len(d.ToolCalls) > 0) {
				s.FirstTokenSec = now().Sub(start).Seconds()
			}
			if c.FinishReason != "" {
				s.FinishReason = c.FinishReason
			}
		}
		if u := ch.Usage; u != nil {
			s.PromptTokens = u.PromptTokens
			s.CompletionTokens = u.CompletionTokens
			if u.Details != nil {
				s.CachedTokens = u.Details.Cached
			}
		}
		if len(ch.Timings) > 0 && string(ch.Timings) != "null" {
			s.Timings = ch.Timings
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	s.TotalSec = now().Sub(start).Seconds()
	s.DecodeTPS = decodeRate(s.CompletionTokens, s.FirstTokenSec, s.TotalSec)
	return nil
}

// padding builds about `chars` characters of real source text from the
// files under root (sorted, so every run reads the same text), each
// file headed by its path. Real code is what an agent's context holds;
// a repeated sentence would compress into a cache or a tokenizer in a
// way no session does.
func padding(root string, chars int) (string, error) {
	var files []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && strings.HasPrefix(d.Name(), "_") {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(p, ".go") {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(files)
	var b strings.Builder
	for len(files) > 0 && b.Len() < chars {
		for _, f := range files {
			if b.Len() >= chars {
				break
			}
			body, err := os.ReadFile(f)
			if err != nil {
				return "", err
			}
			rel, _ := filepath.Rel(root, f)
			fmt.Fprintf(&b, "=== %s ===\n%s\n", filepath.ToSlash(rel), body)
		}
	}
	if len(files) == 0 {
		return "", fmt.Errorf("no .go files under %s", root)
	}
	out := b.String()
	if len(out) > chars {
		out = out[:chars]
	}
	return out, nil
}

type serveClient struct {
	base, model, effort string
	http                *http.Client
}

type serveMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func (c *serveClient) do(ctx context.Context, msgs []serveMsg, maxTokens int, s *serveSample) {
	body := map[string]any{
		"model": c.model, "messages": msgs, "stream": true,
		"stream_options": map[string]bool{"include_usage": true},
		"temperature":    0,
	}
	if maxTokens > 0 {
		body["max_tokens"] = maxTokens
	}
	if c.effort != "" {
		body["reasoning_effort"] = c.effort
	}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		s.Error = err.Error()
		return
	}
	req.Header.Set("Content-Type", "application/json")
	start := time.Now()
	resp, err := c.http.Do(req)
	if err != nil {
		s.Error = err.Error()
		s.TotalSec = time.Since(start).Seconds()
		return
	}
	defer func() { _ = resp.Body.Close() }()
	s.Status = resp.StatusCode
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2000))
		s.Error = strings.TrimSpace(string(b))
		s.TotalSec = time.Since(start).Seconds()
		return
	}
	if err := serveSSE(resp.Body, start, time.Now, s); err != nil {
		s.Error = err.Error()
	}
}

// serveSizes parses "1k,12k,35k" into token counts.
func serveSizes(spec string) ([]int, error) {
	var out []int
	for _, f := range splitList(spec) {
		mult := 1
		if strings.HasSuffix(f, "k") {
			mult, f = 1000, strings.TrimSuffix(f, "k")
		}
		n, err := strconv.Atoi(f)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("size %q is not a positive count", f)
		}
		out = append(out, n*mult)
	}
	if len(out) == 0 {
		return nil, errors.New("no sizes")
	}
	return out, nil
}

// charsPerToken converts a size class to padding characters: this
// repository's Go source came to 2.8 characters a token with the Qwen
// 3.6 tokenizer (measured, 2026-10-07). Each sample records the real
// count, so the class is a target, not a claim.
const charsPerToken = 2.8

const decodeAsk = "Count upward from 1, one number per line, and keep going until you are stopped. Numbers only."

func cmdServe(args []string, out io.Writer) error {
	fsx := flag.NewFlagSet("serve", flag.ContinueOnError)
	base := fsx.String("base-url", "", "OpenAI-compatible API root, e.g. http://localhost:11234/v1 (required)")
	model := fsx.String("model", "", "model id as the server lists it (required)")
	label := fsx.String("label", "", "a name for this server/model in the results (required)")
	sizesSpec := fsx.String("sizes", "1k,12k,35k,69k", "prompt size classes, in tokens")
	reps := fsx.Int("reps", 2, "repetitions of every measurement")
	effort := fsx.String("reasoning-effort", "none", `sent as reasoning_effort; "none" keeps thinking out of the decode figure, "" sends nothing`)
	decodeTokens := fsx.Int("decode-tokens", 300, "max_tokens of the decode measurement")
	convs := fsx.Int("convs", 5, "distinct conversations to alternate between for the reuse measurement (0 skips it)")
	pair := fsx.Bool("pair", true, "measure two decode streams at once")
	src := fsx.String("source", ".", "directory whose .go files pad the prompts")
	outDir := fsx.String("out", "", "results directory (default bench/_results/serve-<label>-<timestamp>)")
	timeout := fsx.Duration("timeout", 15*time.Minute, "deadline per request")
	if err := fsx.Parse(args); err != nil {
		return err
	}
	if *base == "" || *model == "" || *label == "" {
		return errors.New("--base-url, --model and --label are required")
	}
	sizes, err := serveSizes(*sizesSpec)
	if err != nil {
		return err
	}
	maxChars := int(float64(sizes[len(sizes)-1])*charsPerToken) + 4096
	text, err := padding(*src, maxChars)
	if err != nil {
		return err
	}
	if *outDir == "" {
		*outDir = filepath.Join("bench", "_results", "serve-"+*label+"-"+time.Now().Format("20060102-150405"))
	}
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		return err
	}
	logf, err := os.OpenFile(filepath.Join(*outDir, "requests.jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = logf.Close() }()
	meta, _ := json.Marshal(map[string]any{"label": *label, "base_url": *base, "model": *model,
		"sizes": sizes, "reps": *reps, "reasoning_effort": *effort, "started": time.Now().Format(time.RFC3339)})
	if err := os.WriteFile(filepath.Join(*outDir, "meta.json"), meta, 0o644); err != nil {
		return err
	}

	c := &serveClient{base: strings.TrimRight(*base, "/"), model: *model, effort: *effort, http: &http.Client{}}
	var mu sync.Mutex
	var all []serveSample
	record := func(s serveSample) {
		mu.Lock()
		defer mu.Unlock()
		all = append(all, s)
		line, _ := json.Marshal(s)
		_, _ = logf.Write(append(line, '\n'))
		fmt.Fprintf(out, "%-10s %6d rep%d  prompt %6d cached %6d  first %7.2fs  total %7.2fs  decode %6.1f tok/s  %s %s\n",
			s.Label, s.Size, s.Rep, s.PromptTokens, s.CachedTokens, s.FirstTokenSec, s.TotalSec, s.DecodeTPS, s.FinishReason, s.Error)
	}
	run := func(lbl string, size, rep int, msgs []serveMsg, maxTok int) serveSample {
		ctx, cancel := context.WithTimeout(context.Background(), *timeout)
		defer cancel()
		s := serveSample{Label: lbl, Size: size, Rep: rep}
		c.do(ctx, msgs, maxTok, &s)
		record(s)
		return s
	}
	// The run's nonce opens every system prompt: no prefix this run
	// sends can match one an earlier run left in the server's cache.
	runNonce := time.Now().UnixNano()
	system := func(size, rep, conv int) string {
		n := int(float64(size) * charsPerToken)
		if n > len(text) {
			n = len(text)
		}
		return fmt.Sprintf("Session %d-%d-%d-%d. You are a coding assistant; the repository source follows.\n\n%s",
			runNonce, size, rep, conv, text[:n])
	}
	const question = "In one sentence: what does this code base do?"
	for _, size := range sizes {
		for rep := 1; rep <= *reps; rep++ {
			sys := system(size, rep, 0)
			first := []serveMsg{{"system", sys}, {"user", question}}
			cold := run("cold", size, rep, first, 64)
			if cold.Error != "" {
				fmt.Fprintf(out, "size %d failed cold; skipping its other measurements\n", size)
				break
			}
			run("resend", size, rep, first, 64)
			next := append(append([]serveMsg{}, first...),
				serveMsg{"assistant", "It is a sandboxed agent runtime."},
				serveMsg{"user", "Name one package in it, in one word."})
			run("next-turn", size, rep, next, 64)
			run("decode", size, rep, []serveMsg{{"system", sys}, {"user", decodeAsk}}, *decodeTokens)
		}
	}
	// Reuse across conversations: N distinct prefixes of the middle size
	// read once, then each asked again — does the server still hold
	// the first when the last has been read?
	if *convs > 0 {
		size := sizes[len(sizes)/2]
		for rep := 1; rep <= *reps; rep++ {
			for pass := 0; pass < 2; pass++ {
				for k := 1; k <= *convs; k++ {
					lbl := fmt.Sprintf("conv%d-%s", k, map[int]string{0: "read", 1: "back"}[pass])
					run(lbl, size, rep, []serveMsg{{"system", system(size, rep, k)}, {"user", question}}, 32)
				}
			}
		}
	}
	if *pair {
		size := sizes[0]
		for rep := 1; rep <= *reps; rep++ {
			var wg sync.WaitGroup
			for k := 1; k <= 2; k++ {
				wg.Add(1)
				go func(k int) {
					defer wg.Done()
					run(fmt.Sprintf("pair-%d", k), size, rep, []serveMsg{{"system", system(size, rep, 100+k)}, {"user", decodeAsk}}, *decodeTokens)
				}(k)
			}
			wg.Wait()
		}
	}
	summary := serveSummary(all)
	fmt.Fprint(out, "\n"+summary)
	return os.WriteFile(filepath.Join(*outDir, "summary.md"), []byte(summary), 0o644)
}

// serveSummary folds the samples into one table: per measurement and
// size, the medians a decision quotes, and how many requests failed.
func serveSummary(all []serveSample) string {
	type key struct {
		label string
		size  int
	}
	groups := map[key][]serveSample{}
	var order []key
	for _, s := range all {
		k := key{s.Label, s.Size}
		if _, ok := groups[k]; !ok {
			order = append(order, k)
		}
		groups[k] = append(groups[k], s)
	}
	var b strings.Builder
	b.WriteString("| measurement | size | n | failed | prompt tok | cached tok | first token s (med) | total s (med) | decode tok/s (med) |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|---|\n")
	for _, k := range order {
		g := groups[k]
		var prompt, cached, first, total, dec []float64
		failed := 0
		for _, s := range g {
			if s.Error != "" {
				failed++
				continue
			}
			prompt = append(prompt, float64(s.PromptTokens))
			cached = append(cached, float64(s.CachedTokens))
			first = append(first, s.FirstTokenSec)
			total = append(total, s.TotalSec)
			if s.DecodeTPS > 0 {
				dec = append(dec, s.DecodeTPS)
			}
		}
		fmt.Fprintf(&b, "| %s | %d | %d | %d | %.0f | %.0f | %.2f | %.2f | %.1f |\n",
			k.label, k.size, len(g), failed, median(prompt), median(cached), median(first), median(total), median(dec))
	}
	return b.String()
}
