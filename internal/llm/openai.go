package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/nlink-jp/nlk/backoff"
)

// OpenAI is the backend for any server speaking the OpenAI-compatible
// chat/completions API: LM Studio, Ollama, or another. The HTTP layer
// is stdlib only and the SSE reader is hand-written (the llm-cli
// approach; org supply-chain policy — no SDK, no community client).
//
// The provider name selects one thing: where ContextWindow asks. The
// conversation itself is sent the same way to every provider.
type OpenAI struct {
	baseURL  string // ".../v1", no trailing slash
	origin   string // scheme://host[:port], for the provider-native probes
	model    string
	apiKey   string
	provider string
	http     *http.Client
	// observer receives StreamEvents; nil = nobody watching. Set once
	// at startup, before any turn — deliberately immutable.
	observer func(StreamEvent)
	// traceDir, when set (LAGENT_LLM_TRACE), receives every request
	// body and the raw SSE reply as files: the only way to read an odd
	// completion — an empty one, a swallowed tool call — as the server
	// sent it. Off by default; a trace write failure is reported as the
	// turn's error rather than silently dropping the trace.
	traceDir string
	traceSeq atomic.Int64
	// reasoningEffort rides every request as `reasoning_effort` when
	// set (ADR-0009); the vocabulary is the server's.
	reasoningEffort string
}

// SetReasoningEffort sets the `reasoning_effort` every request carries;
// empty sends nothing. Call before the first turn.
func (o *OpenAI) SetReasoningEffort(effort string) { o.reasoningEffort = effort }

// TraceEnv names the directory that receives request/response traces.
const TraceEnv = "LAGENT_LLM_TRACE"

// Providers ContextWindow knows how to ask.
const (
	ProviderLMStudio = "lmstudio"
	ProviderOllama   = "ollama"
	ProviderOpenAI   = "openai"
)

// NewOpenAI creates the backend. baseURL is the API root the server
// documents (LM Studio: http://localhost:1234/v1); a base without the
// /v1 suffix is accepted and completed. apiKey may be empty — local
// servers need none. provider is one of the Provider constants.
func NewOpenAI(baseURL, model, apiKey, provider string) (*OpenAI, error) {
	u, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("llm: base_url %q is not an absolute URL", baseURL)
	}
	switch provider {
	case ProviderLMStudio, ProviderOllama, ProviderOpenAI:
	default:
		return nil, fmt.Errorf("llm: provider %q is not one of lmstudio, ollama, openai", provider)
	}
	if model == "" {
		return nil, errors.New("llm: model is empty")
	}
	base := u.String()
	if !strings.HasSuffix(base, "/v1") {
		base += "/v1"
	}
	return &OpenAI{
		baseURL:  base,
		origin:   u.Scheme + "://" + u.Host,
		model:    model,
		apiKey:   apiKey,
		provider: provider,
		// No client-wide timeout: a turn legitimately runs for minutes
		// on a local model, and the context carries the cancel. Dial
		// and header timeouts still bound a server that never answers.
		http: &http.Client{Transport: &http.Transport{
			DialContext:           (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
			ResponseHeaderTimeout: 10 * time.Minute,
		}},
		traceDir: os.Getenv(TraceEnv),
	}, nil
}

// traceFiles opens the request and response trace files for one
// attempt, or returns nils when tracing is off.
func (o *OpenAI) traceFiles() (reqW, respW *os.File, err error) {
	if o.traceDir == "" {
		return nil, nil, nil
	}
	if err := os.MkdirAll(o.traceDir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("llm trace: %w", err)
	}
	stamp := fmt.Sprintf("%s-%03d", time.Now().Format("20060102-150405.000"), o.traceSeq.Add(1))
	reqW, err = os.Create(filepath.Join(o.traceDir, stamp+"-request.json"))
	if err != nil {
		return nil, nil, fmt.Errorf("llm trace: %w", err)
	}
	respW, err = os.Create(filepath.Join(o.traceDir, stamp+"-response.sse"))
	if err != nil {
		_ = reqW.Close()
		return nil, nil, fmt.Errorf("llm trace: %w", err)
	}
	return reqW, respW, nil
}

// SetObserver installs the turn-observability sink. Call before the
// first turn; events fire from the streaming goroutine.
func (o *OpenAI) SetObserver(fn func(StreamEvent)) { o.observer = fn }

func (o *OpenAI) observe(ev StreamEvent) {
	if o.observer != nil {
		o.observer(ev)
	}
}

// Model names the model every call goes to.
func (o *OpenAI) Model() string { return o.model }

// Provider names the server kind ContextWindow asks.
func (o *OpenAI) Provider() string { return o.provider }

// APIError is a non-2xx answer from the server, body included: a local
// server's 400 names the field it rejected, and that text is the only
// diagnostic there is.
type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string {
	body := strings.TrimSpace(e.Body)
	if len(body) > 600 {
		body = body[:600] + "…"
	}
	return fmt.Sprintf("API error %d: %s", e.Status, body)
}

// --- wire types --------------------------------------------------------

type wireRequest struct {
	Model         string             `json:"model"`
	Messages      []wireMessage      `json:"messages"`
	Tools         []wireTool         `json:"tools,omitempty"`
	Stream        bool               `json:"stream"`
	StreamOptions *wireStreamOptions `json:"stream_options,omitempty"`
	// ReasoningEffort is the operator's `[llm].reasoning_effort`,
	// verbatim; absent when unset (ADR-0009).
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
}

type wireStreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

// wireMessage serialises content as a string when there are no parts
// and as a part list otherwise (the multimodal shape).
type wireMessage struct {
	Role       string         `json:"role"`
	Content    string         `json:"-"`
	Parts      []wirePart     `json:"-"`
	ToolCalls  []wireToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}

func (m wireMessage) MarshalJSON() ([]byte, error) {
	type alias struct {
		Role       string         `json:"role"`
		Content    any            `json:"content"`
		ToolCalls  []wireToolCall `json:"tool_calls,omitempty"`
		ToolCallID string         `json:"tool_call_id,omitempty"`
	}
	a := alias{Role: m.Role, ToolCalls: m.ToolCalls, ToolCallID: m.ToolCallID}
	if len(m.Parts) > 0 {
		a.Content = m.Parts
	} else {
		a.Content = m.Content
	}
	return json.Marshal(a)
}

type wirePart struct {
	Type     string        `json:"type"`
	Text     string        `json:"text,omitempty"`
	ImageURL *wireImageURL `json:"image_url,omitempty"`
}

type wireImageURL struct {
	URL string `json:"url"`
}

type wireToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function wireFunction `json:"function"`
}

type wireFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type wireTool struct {
	Type     string      `json:"type"`
	Function wireToolDef `json:"function"`
}

type wireToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters"`
}

// streamChunk is one SSE data payload. Every field is optional on the
// wire; the final usage chunk carries no choices at all.
type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
			// LM Studio and vLLM name the reasoning delta
			// reasoning_content; some servers say reasoning. Both are
			// display-only here.
			ReasoningContent string `json:"reasoning_content"`
			Reasoning        string `json:"reasoning"`
			ToolCalls        []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *wireUsage `json:"usage"`
}

type wireUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
	PromptDetails    *struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"prompt_tokens_details"`
	CompletionDetails *struct {
		ReasoningTokens int `json:"reasoning_tokens"`
	} `json:"completion_tokens_details"`
}

// --- request building --------------------------------------------------

// buildMessages converts the history to the wire shape. Tool-call ids
// pair an assistant's calls with the tool results that follow; a
// history whose ids are empty (a transcript from a server that sent
// none) gets synthetic ids, the same on both sides of the pair.
func buildMessages(system string, messages []Message) ([]wireMessage, error) {
	var out []wireMessage
	if system != "" {
		out = append(out, wireMessage{Role: "system", Content: system})
	}
	var pendingIDs []string // ids of the last assistant turn's calls, consumed in order
	synth := 0
	for _, m := range messages {
		switch m.Role {
		case RoleAssistant:
			pendingIDs = nil
			if m.Content == "" && len(m.ToolCalls) == 0 {
				continue // nothing to say; an empty message is a wasted turn
			}
			w := wireMessage{Role: "assistant", Content: m.Content}
			for _, tc := range m.ToolCalls {
				id := tc.ID
				if id == "" {
					synth++
					id = fmt.Sprintf("call_%d", synth)
				}
				pendingIDs = append(pendingIDs, id)
				args := "{}"
				if tc.Args != nil {
					b, err := json.Marshal(tc.Args)
					if err != nil {
						return nil, fmt.Errorf("llm: tool call %s arguments: %w", tc.Name, err)
					}
					args = string(b)
				}
				w.ToolCalls = append(w.ToolCalls, wireToolCall{ID: id, Type: "function",
					Function: wireFunction{Name: tc.Name, Arguments: args}})
			}
			out = append(out, w)
		case RoleTool:
			id := m.ToolCallID
			if id == "" && len(pendingIDs) > 0 {
				id = pendingIDs[0]
			}
			if len(pendingIDs) > 0 {
				pendingIDs = pendingIDs[1:]
			}
			out = append(out, wireMessage{Role: "tool", ToolCallID: id, Content: m.Content})
			// The wire has no image slot on a tool message. An image a
			// tool returned (an MCP screenshot) follows as a user
			// message that says where it came from.
			var parts []wirePart
			for _, att := range m.Attachments {
				if p, ok := imagePart(att); ok {
					parts = append(parts, p)
				}
			}
			if len(parts) > 0 {
				label := fmt.Sprintf("[image returned by tool %s]", m.ToolName)
				out = append(out, wireMessage{Role: "user", Parts: append([]wirePart{{Type: "text", Text: label}}, parts...)})
			}
		default: // RoleUser
			pendingIDs = nil
			var parts []wirePart
			if m.Content != "" {
				parts = append(parts, wirePart{Type: "text", Text: m.Content})
			}
			for _, att := range m.Attachments {
				if len(att.Data) == 0 {
					continue // text attachments are flattened into Content by the agent
				}
				p, ok := imagePart(att)
				if !ok {
					return nil, fmt.Errorf("llm: attachment %s (%s) is not an image; this backend sends images only", att.Ref, att.MIME)
				}
				parts = append(parts, p)
			}
			if len(parts) == 0 {
				continue
			}
			if len(parts) == 1 && parts[0].Type == "text" {
				out = append(out, wireMessage{Role: "user", Content: parts[0].Text})
			} else {
				out = append(out, wireMessage{Role: "user", Parts: parts})
			}
		}
	}
	return out, nil
}

func imagePart(att Attachment) (wirePart, bool) {
	if len(att.Data) == 0 || !strings.HasPrefix(att.MIME, "image/") {
		return wirePart{}, false
	}
	return wirePart{Type: "image_url", ImageURL: &wireImageURL{
		URL: "data:" + att.MIME + ";base64," + base64.StdEncoding.EncodeToString(att.Data)}}, true
}

func buildTools(tools []ToolDef) []wireTool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]wireTool, 0, len(tools))
	for _, t := range tools {
		params := t.Parameters
		if params == nil {
			params = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		out = append(out, wireTool{Type: "function", Function: wireToolDef{
			Name: t.Name, Description: t.Description, Parameters: params}})
	}
	return out
}

// --- the turn ----------------------------------------------------------

const maxStreamAttempts = 5

// ChatStream sends the conversation and streams the answer. Text deltas
// go to onText as they arrive; tool calls are assembled from their
// deltas and returned whole. A transient failure (429, 5xx, a dropped
// connection) retries with backoff, but only while nothing has been
// consumed from the stream: a retry after emitted text would duplicate
// output, and after a captured call would duplicate the call.
func (o *OpenAI) ChatStream(ctx context.Context, system string, messages []Message, tools []ToolDef, onText func(string)) (*Response, error) {
	msgs, err := buildMessages(system, messages)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(wireRequest{
		Model:           o.model,
		Messages:        msgs,
		Tools:           buildTools(tools),
		Stream:          true,
		StreamOptions:   &wireStreamOptions{IncludeUsage: true},
		ReasoningEffort: o.reasoningEffort,
	})
	if err != nil {
		return nil, fmt.Errorf("llm: marshal request: %w", err)
	}

	bo := backoff.New(backoff.WithBase(500*time.Millisecond), backoff.WithMax(15*time.Second))
	var lastErr error
	for attempt := 0; attempt < maxStreamAttempts; attempt++ {
		if attempt > 0 {
			delay := bo.Duration(attempt - 1)
			// Deliberate waiting must not look like a hang: the observer
			// shows the retry and its cause.
			o.observe(StreamEvent{Kind: "retry", Attempt: attempt + 1, Max: maxStreamAttempts,
				Cause: retryCause(lastErr), DelayMS: int(delay.Milliseconds())})
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		resp, consumed, streamErr := o.stream(ctx, body, onText)
		if streamErr == nil {
			return resp, nil
		}
		if consumed || ctx.Err() != nil || !transient(streamErr) {
			return nil, fmt.Errorf("llm: %w", streamErr)
		}
		lastErr = streamErr
	}
	return nil, fmt.Errorf("llm: %d attempts exhausted: %w", maxStreamAttempts, lastErr)
}

// stream performs one attempt. consumed reports whether any
// content-bearing chunk reached the caller — the retry disarm.
func (o *OpenAI) stream(ctx context.Context, body []byte, onText func(string)) (resp *Response, consumed bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if o.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+o.apiKey)
	}
	reqTrace, respTrace, err := o.traceFiles()
	if err != nil {
		return nil, false, err
	}
	if reqTrace != nil {
		_, werr := reqTrace.Write(body)
		if cerr := reqTrace.Close(); werr == nil {
			werr = cerr
		}
		if werr != nil {
			_ = respTrace.Close()
			return nil, false, fmt.Errorf("llm trace: %w", werr)
		}
		defer func() {
			if cerr := respTrace.Close(); err == nil && cerr != nil {
				err = fmt.Errorf("llm trace: %w", cerr)
			}
		}()
	}
	hr, err := o.http.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = hr.Body.Close() }()
	if hr.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(hr.Body, 64<<10))
		if respTrace != nil {
			_, _ = respTrace.Write(b)
		}
		return nil, false, &APIError{Status: hr.StatusCode, Body: string(b)}
	}

	acc := newAccumulator()
	var bodyReader io.Reader = hr.Body
	if respTrace != nil {
		bodyReader = io.TeeReader(hr.Body, respTrace)
	}
	sc := bufio.NewScanner(bodyReader)
	sc.Buffer(make([]byte, 0, 64<<10), 16<<20) // a whole write_file argument can ride one line
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			break
		}
		// Every chunk is a heartbeat: a usage-only or empty chunk
		// proves liveness even though it displays nothing.
		o.observe(StreamEvent{Kind: "chunk"})
		var ch streamChunk
		if err := json.Unmarshal([]byte(payload), &ch); err != nil {
			return nil, consumed, fmt.Errorf("stream chunk: %w", err)
		}
		if acc.fold(ch, onText, o.thoughtSink()) {
			consumed = true
		}
	}
	if err := sc.Err(); err != nil {
		return nil, consumed, fmt.Errorf("stream read: %w", err)
	}
	r, err := acc.response()
	if err != nil {
		return nil, consumed, err
	}
	return r, consumed, nil
}

func (o *OpenAI) thoughtSink() func(string) {
	if o.observer == nil {
		return nil
	}
	return func(s string) { o.observe(StreamEvent{Kind: "thought", Thought: s}) }
}

// accumulator folds stream chunks into one Response. Tool calls arrive
// as indexed deltas — the id and name on the first, the arguments
// possibly split over several — and are assembled by index.
type accumulator struct {
	text   strings.Builder
	calls  []*partialCall
	byIdx  map[int]*partialCall
	finish string
	usage  *wireUsage
}

type partialCall struct {
	id   string
	name string
	args strings.Builder
}

func newAccumulator() *accumulator { return &accumulator{byIdx: map[int]*partialCall{}} }

// fold applies one chunk and reports whether it carried content (text
// or a tool-call delta) — metadata-only chunks must not disarm the
// transient-error retry.
func (a *accumulator) fold(ch streamChunk, onText, onThought func(string)) bool {
	content := false
	if ch.Usage != nil {
		a.usage = ch.Usage
	}
	for _, c := range ch.Choices {
		if c.FinishReason != "" {
			a.finish = c.FinishReason
		}
		if t := c.Delta.ReasoningContent + c.Delta.Reasoning; t != "" && onThought != nil {
			onThought(t)
		}
		if c.Delta.Content != "" {
			content = true
			a.text.WriteString(c.Delta.Content)
			if onText != nil {
				onText(c.Delta.Content)
			}
		}
		for _, d := range c.Delta.ToolCalls {
			content = true
			pc, ok := a.byIdx[d.Index]
			if !ok {
				pc = &partialCall{}
				a.byIdx[d.Index] = pc
				a.calls = append(a.calls, pc)
			}
			if d.ID != "" {
				pc.id = d.ID
			}
			if d.Function.Name != "" {
				pc.name += d.Function.Name
			}
			pc.args.WriteString(d.Function.Arguments)
		}
	}
	return content
}

func (a *accumulator) response() (*Response, error) {
	r := &Response{Content: a.text.String(), FinishReason: a.finish}
	for _, pc := range a.calls {
		if pc.name == "" {
			return nil, errors.New("stream: a tool call arrived without a function name")
		}
		tc := ToolCall{ID: pc.id, Name: pc.name}
		raw := strings.TrimSpace(pc.args.String())
		if raw == "" {
			raw = "{}"
		}
		var args map[string]any
		if err := json.Unmarshal([]byte(raw), &args); err != nil {
			// The model's arguments were not a JSON object. Surface it
			// to the model through the executor rather than guessing.
			tc.ArgsError = fmt.Sprintf("arguments are not a JSON object: %v", err)
		} else {
			tc.Args = args
		}
		r.ToolCalls = append(r.ToolCalls, tc)
	}
	if u := a.usage; u != nil {
		r.PromptTokens = u.PromptTokens
		r.OutputTokens = u.CompletionTokens
		r.TotalTokens = u.TotalTokens
		if u.PromptDetails != nil {
			r.CachedTokens = u.PromptDetails.CachedTokens
		}
		if u.CompletionDetails != nil {
			r.ThoughtTokens = u.CompletionDetails.ReasoningTokens
			// The server folds reasoning into completion_tokens; the
			// record keeps the buckets apart so they add up to Total.
			r.OutputTokens -= r.ThoughtTokens
			if r.OutputTokens < 0 {
				r.OutputTokens = 0
			}
		}
	}
	return r, nil
}

// transient reports whether a failed attempt may be retried: a
// rate-limit or server-side status, or a connection that never
// delivered a response.
func transient(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		switch apiErr.Status {
		case 429, 500, 502, 503, 504:
			return true
		}
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	var opErr *net.OpError
	return errors.As(err, &opErr) || errors.Is(err, io.ErrUnexpectedEOF)
}

// retryCause reduces an error to the short token the status line
// shows ("429", "503", "connection", or "error").
func retryCause(err error) string {
	if err == nil {
		return "error"
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return fmt.Sprint(apiErr.Status)
	}
	var netErr net.Error
	if errors.As(err, &netErr) || errors.Is(err, io.ErrUnexpectedEOF) {
		return "connection"
	}
	return "error"
}

// --- context window ----------------------------------------------------

// ContextWindow asks the provider for the loaded model's context length.
// LM Studio answers on its native /api/v0/models/<id> (loaded length
// first, the model's maximum as the fallback); Ollama on /api/show
// (num_ctx from the parameters when the model sets one, else the
// architecture's context_length). A plain OpenAI-compatible server has
// no such endpoint: the window must be configured.
func (o *OpenAI) ContextWindow(ctx context.Context) (int, error) {
	switch o.provider {
	case ProviderLMStudio:
		return o.lmStudioWindow(ctx)
	case ProviderOllama:
		return o.ollamaWindow(ctx)
	default:
		return 0, errors.New("llm: provider openai has no context-length endpoint; set [model].context_window")
	}
}

func (o *OpenAI) getJSON(ctx context.Context, method, u string, body []byte, v any) error {
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rd)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if o.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+o.apiKey)
	}
	hr, err := o.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = hr.Body.Close() }()
	b, err := io.ReadAll(io.LimitReader(hr.Body, 4<<20))
	if err != nil {
		return err
	}
	if hr.StatusCode != http.StatusOK {
		return &APIError{Status: hr.StatusCode, Body: string(b)}
	}
	return json.Unmarshal(b, v)
}

func (o *OpenAI) lmStudioWindow(ctx context.Context) (int, error) {
	var info struct {
		Loaded int `json:"loaded_context_length"`
		Max    int `json:"max_context_length"`
	}
	u := o.origin + "/api/v0/models/" + url.PathEscape(o.model)
	if err := o.getJSON(ctx, http.MethodGet, u, nil, &info); err != nil {
		return 0, fmt.Errorf("llm: LM Studio model info: %w", err)
	}
	switch {
	case info.Loaded > 0:
		return info.Loaded, nil
	case info.Max > 0:
		return info.Max, nil
	}
	return 0, fmt.Errorf("llm: LM Studio reports no context length for %s", o.model)
}

func (o *OpenAI) ollamaWindow(ctx context.Context) (int, error) {
	var info struct {
		Parameters string         `json:"parameters"`
		ModelInfo  map[string]any `json:"model_info"`
	}
	body, _ := json.Marshal(map[string]string{"model": o.model})
	if err := o.getJSON(ctx, http.MethodPost, o.origin+"/api/show", body, &info); err != nil {
		return 0, fmt.Errorf("llm: Ollama model info: %w", err)
	}
	// A model file's num_ctx is the length the server actually loads.
	for _, line := range strings.Split(info.Parameters, "\n") {
		f := strings.Fields(line)
		if len(f) == 2 && f[0] == "num_ctx" {
			var n int
			if _, err := fmt.Sscanf(f[1], "%d", &n); err == nil && n > 0 {
				return n, nil
			}
		}
	}
	for k, v := range info.ModelInfo {
		if strings.HasSuffix(k, ".context_length") {
			if n, ok := v.(float64); ok && n > 0 {
				return int(n), nil
			}
		}
	}
	return 0, fmt.Errorf("llm: Ollama reports no context length for %s", o.model)
}
