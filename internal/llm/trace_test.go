package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTraceWritesRequestAndRawReply: with LAGENT_LLM_TRACE set, one
// attempt leaves the request body and the reply as the server sent it
// — every SSE line, the [DONE] terminator included — so an empty
// completion can be read back byte for byte.
func TestTraceWritesRequestAndRawReply(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sse(w,
			`{"choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}`,
			`{"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
			`{"choices":[],"usage":{"prompt_tokens":10,"completion_tokens":3,"total_tokens":13}}`,
		)
	}))
	defer srv.Close()
	dir := filepath.Join(t.TempDir(), "trace", "nested")
	t.Setenv(TraceEnv, dir)
	o, err := NewOpenAI(srv.URL+"/v1", "test-model", "", ProviderLMStudio)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := o.ChatStream(context.Background(), "sys", []Message{{Role: RoleUser, Content: "hi"}}, nil, nil); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("trace dir was not created: %v", err)
	}
	var req, resp string
	for _, e := range entries {
		data, rerr := os.ReadFile(filepath.Join(dir, e.Name()))
		if rerr != nil {
			t.Fatal(rerr)
		}
		switch {
		case strings.HasSuffix(e.Name(), "-request.json"):
			req = string(data)
		case strings.HasSuffix(e.Name(), "-response.sse"):
			resp = string(data)
		}
	}
	if !strings.Contains(req, `"model":"test-model"`) || !strings.Contains(req, `"content":"hi"`) {
		t.Errorf("request trace: %s", req)
	}
	if !strings.Contains(resp, `"finish_reason":"stop"`) || !strings.Contains(resp, "data: [DONE]") {
		t.Errorf("response trace must be the raw stream: %s", resp)
	}
	if len(entries) != 2 {
		t.Errorf("one attempt leaves two files, got %d", len(entries))
	}
}

func TestTraceOffLeavesNothing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sse(w, `{"choices":[{"index":0,"delta":{"content":"ok"},"finish_reason":"stop"}]}`)
	}))
	defer srv.Close()
	t.Setenv(TraceEnv, "")
	o := newTestBackend(t, srv, ProviderLMStudio)
	if _, err := o.ChatStream(context.Background(), "sys", []Message{{Role: RoleUser, Content: "hi"}}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if o.traceDir != "" {
		t.Error("trace must be off when the variable is empty")
	}
}
