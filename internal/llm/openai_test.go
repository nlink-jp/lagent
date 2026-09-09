package llm

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// sse writes one data: line per payload and a [DONE] terminator.
func sse(w http.ResponseWriter, payloads ...string) {
	w.Header().Set("Content-Type", "text/event-stream")
	for _, p := range payloads {
		fmt.Fprintf(w, "data: %s\n\n", p)
	}
	fmt.Fprint(w, "data: [DONE]\n\n")
}

func newTestBackend(t *testing.T, srv *httptest.Server, provider string) *OpenAI {
	t.Helper()
	o, err := NewOpenAI(srv.URL+"/v1", "test-model", "", provider)
	if err != nil {
		t.Fatal(err)
	}
	return o
}

func TestNewOpenAIValidatesAndCompletesTheBase(t *testing.T) {
	o, err := NewOpenAI("http://localhost:1234", "m", "", ProviderLMStudio)
	if err != nil {
		t.Fatal(err)
	}
	if o.baseURL != "http://localhost:1234/v1" || o.origin != "http://localhost:1234" {
		t.Fatalf("base %q origin %q", o.baseURL, o.origin)
	}
	o, err = NewOpenAI("http://localhost:1234/v1/", "m", "", ProviderOllama)
	if err != nil || o.baseURL != "http://localhost:1234/v1" {
		t.Fatalf("trailing slash: %v %q", err, o.baseURL)
	}
	for _, bad := range [][3]string{{"localhost:1234", "m", ProviderLMStudio}, {"http://h/v1", "", ProviderLMStudio}, {"http://h/v1", "m", "vertex"}} {
		if _, err := NewOpenAI(bad[0], bad[1], "", bad[2]); err == nil {
			t.Errorf("NewOpenAI(%q, %q, %q) accepted", bad[0], bad[1], bad[2])
		}
	}
}

func TestBuildMessagesShapesEveryRole(t *testing.T) {
	hist := []Message{
		{Role: RoleUser, Content: "look", Attachments: []Attachment{
			{Ref: "@a.txt", Kind: "file", Content: "flattened by the agent, not sent here"},
			{Ref: "@shot.png", Kind: "image", Data: []byte{1, 2}, MIME: "image/png"},
		}},
		{Role: RoleAssistant, Content: "", ToolCalls: []ToolCall{
			{Name: "read_file", Args: map[string]any{"path": "a"}},
			{Name: "list_dir"},
		}},
		{Role: RoleTool, ToolName: "read_file", Content: "A"},
		{Role: RoleTool, ToolName: "list_dir", Content: "B", Attachments: []Attachment{{Ref: "img", Data: []byte{9}, MIME: "image/jpeg"}}},
		{Role: RoleAssistant}, // empty: dropped
		{Role: RoleAssistant, Content: "done"},
	}
	msgs, err := buildMessages("SYS", hist)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(msgs)
	var got []map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	roles := []string{}
	for _, m := range got {
		roles = append(roles, m["role"].(string))
	}
	want := "system user assistant tool tool user assistant"
	if strings.Join(roles, " ") != want {
		t.Fatalf("roles %v, want %q", roles, want)
	}
	if got[0]["content"] != "SYS" {
		t.Errorf("system content %v", got[0]["content"])
	}
	parts := got[1]["content"].([]any)
	if len(parts) != 2 || parts[0].(map[string]any)["type"] != "text" || parts[1].(map[string]any)["type"] != "image_url" {
		t.Errorf("user parts %v", parts)
	}
	if u := parts[1].(map[string]any)["image_url"].(map[string]any)["url"].(string); !strings.HasPrefix(u, "data:image/png;base64,") {
		t.Errorf("image url %q", u)
	}
	calls := got[2]["tool_calls"].([]any)
	c0 := calls[0].(map[string]any)
	c1 := calls[1].(map[string]any)
	if c0["id"] != "call_1" || c1["id"] != "call_2" {
		t.Errorf("synthetic ids %v %v", c0["id"], c1["id"])
	}
	f0 := c0["function"].(map[string]any)
	if f0["name"] != "read_file" || f0["arguments"] != `{"path":"a"}` {
		t.Errorf("function %v", f0)
	}
	if c1["function"].(map[string]any)["arguments"] != "{}" {
		t.Errorf("nil args must serialise as {}: %v", c1)
	}
	if got[3]["tool_call_id"] != "call_1" || got[4]["tool_call_id"] != "call_2" {
		t.Errorf("tool results paired %v / %v", got[3]["tool_call_id"], got[4]["tool_call_id"])
	}
	if got[3]["content"] != "A" {
		t.Errorf("tool content %v", got[3]["content"])
	}
	// The image a tool returned rides a following user message.
	img := got[5]["content"].([]any)
	if len(img) != 2 || !strings.Contains(img[0].(map[string]any)["text"].(string), "list_dir") {
		t.Errorf("tool image message %v", img)
	}
	if got[6]["content"] != "done" {
		t.Errorf("final assistant %v", got[6])
	}
}

func TestBuildMessagesKeepsExplicitIDs(t *testing.T) {
	hist := []Message{
		{Role: RoleAssistant, ToolCalls: []ToolCall{{ID: "975939834", Name: "x"}}},
		{Role: RoleTool, ToolName: "x", ToolCallID: "975939834", Content: "r"},
	}
	msgs, err := buildMessages("", hist)
	if err != nil {
		t.Fatal(err)
	}
	if msgs[0].ToolCalls[0].ID != "975939834" || msgs[1].ToolCallID != "975939834" {
		t.Fatalf("ids rewritten: %+v", msgs)
	}
}

func TestBuildMessagesRefusesNonImageBinary(t *testing.T) {
	_, err := buildMessages("", []Message{{Role: RoleUser, Content: "x", Attachments: []Attachment{{Ref: "@a.pdf", Data: []byte{1}, MIME: "application/pdf"}}}})
	if err == nil || !strings.Contains(err.Error(), "@a.pdf") {
		t.Fatalf("err = %v", err)
	}
}

func TestBuildToolsFillsMissingParameters(t *testing.T) {
	w := buildTools([]ToolDef{{Name: "t"}})
	if w[0].Function.Parameters["type"] != "object" {
		t.Fatalf("parameters %v", w[0].Function.Parameters)
	}
	if buildTools(nil) != nil {
		t.Fatal("no tools must serialise as absent")
	}
}

func TestChatStreamAssemblesTextThoughtToolCallsAndUsage(t *testing.T) {
	var gotReq wireRequest
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &gotReq); err != nil {
			t.Errorf("request body: %v", err)
		}
		sse(w,
			`{"choices":[{"index":0,"delta":{"role":"assistant","reasoning_content":"hmm "},"finish_reason":null}]}`,
			`{"choices":[{"index":0,"delta":{"content":"Hel"},"finish_reason":null}]}`,
			`{"choices":[{"index":0,"delta":{"content":"lo"},"finish_reason":null}]}`,
			`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"c1","type":"function","function":{"name":"read_file","arguments":""}}]},"finish_reason":null}]}`,
			`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":1,"id":"c2","type":"function","function":{"name":"list_dir","arguments":"{\"path\":"}}]},"finish_reason":null}]}`,
			`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"path\":\"/etc/hosts\"}"}}]},"finish_reason":null}]}`,
			`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":1,"function":{"arguments":"\"/tmp\"}"}}]},"finish_reason":null}]}`,
			`{"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
			`{"choices":[],"usage":{"prompt_tokens":100,"completion_tokens":40,"total_tokens":140,"completion_tokens_details":{"reasoning_tokens":7}}}`,
		)
	}))
	defer srv.Close()
	o, err := NewOpenAI(srv.URL+"/v1", "test-model", "secret", ProviderLMStudio)
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var events []StreamEvent
	o.SetObserver(func(ev StreamEvent) { mu.Lock(); events = append(events, ev); mu.Unlock() })
	var text strings.Builder
	resp, err := o.ChatStream(context.Background(), "SYS", []Message{{Role: RoleUser, Content: "hi"}},
		[]ToolDef{{Name: "read_file", Parameters: map[string]any{"type": "object"}}}, func(s string) { text.WriteString(s) })
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer secret" {
		t.Errorf("Authorization %q", gotAuth)
	}
	if !gotReq.Stream || gotReq.StreamOptions == nil || !gotReq.StreamOptions.IncludeUsage {
		t.Errorf("request must stream with usage: %+v", gotReq)
	}
	if gotReq.Model != "test-model" || len(gotReq.Tools) != 1 || gotReq.Tools[0].Function.Name != "read_file" {
		t.Errorf("request %+v", gotReq)
	}
	if text.String() != "Hello" || resp.Content != "Hello" {
		t.Errorf("text %q / %q", text.String(), resp.Content)
	}
	if resp.FinishReason != "tool_calls" || len(resp.ToolCalls) != 2 {
		t.Fatalf("finish %q calls %+v", resp.FinishReason, resp.ToolCalls)
	}
	if resp.ToolCalls[0].ID != "c1" || resp.ToolCalls[0].Name != "read_file" || resp.ToolCalls[0].Args["path"] != "/etc/hosts" {
		t.Errorf("call 0 %+v", resp.ToolCalls[0])
	}
	if resp.ToolCalls[1].ID != "c2" || resp.ToolCalls[1].Args["path"] != "/tmp" {
		t.Errorf("call 1 (arguments split over chunks) %+v", resp.ToolCalls[1])
	}
	u := resp.Usage()
	if u.Prompt != 100 || u.Output != 33 || u.Thoughts != 7 || u.Total != 140 || u.ToolPrompt != 0 || u.Cached != 0 {
		t.Errorf("usage %+v", u)
	}
	if u.Prompt+u.Output+u.Thoughts+u.ToolPrompt != u.Total {
		t.Errorf("buckets do not add up to total: %+v", u)
	}
	mu.Lock()
	defer mu.Unlock()
	kinds := map[string]int{}
	thought := ""
	for _, ev := range events {
		kinds[ev.Kind]++
		thought += ev.Thought
	}
	if kinds["chunk"] != 9 || kinds["thought"] != 1 || thought != "hmm " || kinds["retry"] != 0 {
		t.Errorf("events %v thought %q", kinds, thought)
	}
}

func TestChatStreamKeepsAPartialTurnOnLength(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sse(w,
			`{"choices":[{"index":0,"delta":{"content":"partial"},"finish_reason":null}]}`,
			`{"choices":[{"index":0,"delta":{},"finish_reason":"length"}]}`,
			`{"choices":[],"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}}`,
		)
	}))
	defer srv.Close()
	o := newTestBackend(t, srv, ProviderLMStudio)
	resp, err := o.ChatStream(context.Background(), "", []Message{{Role: RoleUser, Content: "go"}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Truncated() || resp.Content != "partial" {
		t.Fatalf("truncated turn lost: %+v", resp)
	}
}

func TestChatStreamSurfacesMalformedArguments(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sse(w,
			`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"c","function":{"name":"write_file","arguments":"{\"path\": \"a\", \"content\": \"unterminated"}}]},"finish_reason":null}]}`,
			`{"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
		)
	}))
	defer srv.Close()
	o := newTestBackend(t, srv, ProviderLMStudio)
	resp, err := o.ChatStream(context.Background(), "", []Message{{Role: RoleUser, Content: "go"}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	tc := resp.ToolCalls[0]
	if tc.Args != nil || tc.ArgsError == "" || tc.Name != "write_file" {
		t.Fatalf("malformed arguments must surface, not be repaired: %+v", tc)
	}
}

func TestChatStreamRetriesTransientBeforeAnyContent(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls <= 2 {
			http.Error(w, "busy", http.StatusServiceUnavailable)
			return
		}
		sse(w, `{"choices":[{"index":0,"delta":{"content":"ok"},"finish_reason":"stop"}]}`)
	}))
	defer srv.Close()
	o := newTestBackend(t, srv, ProviderLMStudio)
	var retries []StreamEvent
	o.SetObserver(func(ev StreamEvent) {
		if ev.Kind == "retry" {
			retries = append(retries, ev)
		}
	})
	resp, err := o.ChatStream(context.Background(), "", []Message{{Role: RoleUser, Content: "x"}}, nil, nil)
	if err != nil || resp.Content != "ok" {
		t.Fatalf("resp %+v err %v", resp, err)
	}
	if calls != 3 || len(retries) != 2 || retries[0].Cause != "503" || retries[1].Attempt != 3 || retries[1].Max != maxStreamAttempts {
		t.Fatalf("calls %d retries %+v", calls, retries)
	}
}

func TestChatStreamDoesNotRetryAClientErrorOrAConsumedStream(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Error(w, `{"error":"unknown field x"}`, http.StatusBadRequest)
	}))
	defer srv.Close()
	o := newTestBackend(t, srv, ProviderLMStudio)
	_, err := o.ChatStream(context.Background(), "", []Message{{Role: RoleUser, Content: "x"}}, nil, nil)
	var apiErr *APIError
	if err == nil || !errorsAs(err, &apiErr) || apiErr.Status != 400 || calls != 1 {
		t.Fatalf("400 must fail once: calls %d err %v", calls, err)
	}
	if !strings.Contains(err.Error(), "unknown field x") {
		t.Errorf("the server's own words must survive: %v", err)
	}

	// A stream that delivered text and then died is not retried: a
	// retry would duplicate what the operator already saw.
	calls = 0
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"half\"},\"finish_reason\":null}]}\n\n")
		w.(http.Flusher).Flush()
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Fatal("no hijacker")
		}
		conn, _, _ := hj.Hijack()
		_ = conn.Close()
	}))
	defer srv2.Close()
	o2 := newTestBackend(t, srv2, ProviderLMStudio)
	var seen string
	_, err = o2.ChatStream(context.Background(), "", []Message{{Role: RoleUser, Content: "x"}}, nil, func(s string) { seen += s })
	if err == nil || calls != 1 || seen != "half" {
		t.Fatalf("consumed stream must not retry: calls %d seen %q err %v", calls, seen, err)
	}
}

func errorsAs(err error, target **APIError) bool {
	for err != nil {
		if e, ok := err.(*APIError); ok {
			*target = e
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}

func TestContextWindowPerProvider(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v0/models/google/gemma-4-26b-a4b-qat":
			fmt.Fprint(w, `{"id":"google/gemma-4-26b-a4b-qat","max_context_length":262144,"loaded_context_length":32768}`)
		case r.Method == http.MethodPost && r.URL.Path == "/api/show":
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["model"] == "with-num-ctx" {
				fmt.Fprint(w, `{"parameters":"num_ctx 16384\nstop \"x\"","model_info":{"gemma4.context_length":131072}}`)
			} else {
				fmt.Fprint(w, `{"parameters":"","model_info":{"gemma4.context_length":131072}}`)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	o, _ := NewOpenAI(srv.URL+"/v1", "google/gemma-4-26b-a4b-qat", "", ProviderLMStudio)
	if n, err := o.ContextWindow(context.Background()); err != nil || n != 32768 {
		t.Errorf("lmstudio: %d %v (loaded length wins)", n, err)
	}
	o, _ = NewOpenAI(srv.URL, "with-num-ctx", "", ProviderOllama)
	if n, err := o.ContextWindow(context.Background()); err != nil || n != 16384 {
		t.Errorf("ollama num_ctx: %d %v", n, err)
	}
	o, _ = NewOpenAI(srv.URL, "arch-only", "", ProviderOllama)
	if n, err := o.ContextWindow(context.Background()); err != nil || n != 131072 {
		t.Errorf("ollama context_length: %d %v", n, err)
	}
	o, _ = NewOpenAI(srv.URL, "m", "", ProviderOpenAI)
	if _, err := o.ContextWindow(context.Background()); err == nil || !strings.Contains(err.Error(), "context_window") {
		t.Errorf("openai must ask for the config key: %v", err)
	}
	o, _ = NewOpenAI(srv.URL, "missing", "", ProviderLMStudio)
	if _, err := o.ContextWindow(context.Background()); err == nil {
		t.Error("a 404 must be an error, not a zero window")
	}
}

// The scanner buffer must hold a whole write_file argument on one SSE
// line: a local server sends the arguments as one chunk (measured).
func TestChatStreamAcceptsALargeSingleLineArgument(t *testing.T) {
	big := strings.Repeat("x", 2<<20)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		args, _ := json.Marshal(map[string]string{"path": "a", "content": big})
		payload, _ := json.Marshal(map[string]any{"choices": []map[string]any{{"index": 0, "delta": map[string]any{
			"tool_calls": []map[string]any{{"index": 0, "id": "c", "function": map[string]any{"name": "write_file", "arguments": string(args)}}}},
			"finish_reason": "tool_calls"}}})
		bw := bufio.NewWriter(w)
		fmt.Fprintf(bw, "data: %s\n\ndata: [DONE]\n\n", payload)
		_ = bw.Flush()
	}))
	defer srv.Close()
	o := newTestBackend(t, srv, ProviderLMStudio)
	resp, err := o.ChatStream(context.Background(), "", []Message{{Role: RoleUser, Content: "x"}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := resp.ToolCalls[0].Args["content"].(string); len(got) != len(big) {
		t.Fatalf("argument truncated: %d of %d", len(got), len(big))
	}
}
