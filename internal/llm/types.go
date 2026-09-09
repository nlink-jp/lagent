// Package llm defines the backend abstraction and the OpenAI-compatible
// implementation. The agent loop depends only on the Backend interface so
// tests can drive it with a scripted mock.
//
// The type shapes are ported from gem-agent internal/llm/types.go at
// be7609980022e38314268c58ca94a6517e6f5d28 (v0.74.0), ADR-0001, minus
// the Gemini wire-format fields (thought signatures, bucket URIs) that
// ADR-0002 leaves out. The backend itself is written new around
// llm-cli's client (6237a64ce4595a6e3cc5690f9fdddf6a6b407b84).
package llm

import "context"

// Role identifies who produced a message.
type Role string

// Message roles. The system prompt travels separately (the system
// message is built by the backend), not as a history message.
const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// ToolCall is one function call requested by the model.
//
// The JSON tags are load-bearing, not decoration: a Message is written
// verbatim to the session transcript and read back to resume a session,
// so these names are a persisted format.
type ToolCall struct {
	ID   string         `json:"id,omitempty"`
	Name string         `json:"name"`
	Args map[string]any `json:"args,omitempty"`
	// ArgsError is set when the model's arguments were not a JSON
	// object: the raw text could not be parsed, so Args is nil and the
	// executor returns this error to the model instead of running the
	// tool (the third of the three honest answers to a malformed model
	// output — refuse, retry, or surface — never a silent repair).
	ArgsError string `json:"args_error,omitempty"`
}

// Attachment is file or directory content pulled in by an @-reference.
// It is stored apart from Content because it is untrusted data: the
// agent wraps it in the turn's nonce tag, exactly like a tool result.
type Attachment struct {
	Ref     string `json:"ref"`
	Kind    string `json:"kind"`
	Content string `json:"content,omitempty"`
	// Data and MIME carry binary content — images. []byte round-trips
	// as base64 through the transcript, so a resumed session keeps the
	// screenshots it was looking at.
	Data []byte `json:"data,omitempty"`
	MIME string `json:"mime,omitempty"`
}

// Message is one turn in the conversation history.
type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content,omitempty"`
	// Attachments accompany a user message (@-references).
	Attachments []Attachment `json:"attachments,omitempty"`

	// Assistant-role fields.
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`

	// Tool-role fields.
	ToolName   string `json:"tool_name,omitempty"`
	ToolCallID string `json:"tool_call_id,omitempty"`
	// Denial marks a gate-denial result: content authored by lagent and
	// the operator, no one else — so the send-time wrap treats it as
	// trusted. Set in exactly one place, the executor's denial path;
	// recognizing denials by content instead would let any tool output
	// shaped like one ride unwrapped.
	Denial bool `json:"denial,omitempty"`
	// RuntimeNote is lagent's own words appended to a tool result
	// outside the nonce tag: what the runtime measured about a remote
	// tool's repeated identical failure, and the action to take.
	// Trusted by provenance exactly like Denial — set in one place, the
	// executor — never recognized by content.
	RuntimeNote string `json:"runtime_note,omitempty"`
}

// ToolDef describes one tool to the model.
type ToolDef struct {
	Name        string
	Description string
	Parameters  map[string]any
}

// Usage is one model call's token spend, in the buckets gem-usage-lens
// reads. Thoughts are reasoning tokens the server reports apart from
// Output. Cached is the share of Prompt the server says it served from
// its prefix cache, when it says so at all (LM Studio does not). ToolPrompt
// is always zero here — it counts a cloud provider's built-in tool results
// — and is written anyway so both runtimes' records have one schema
// (ADR-0002). Total is the server's own total_tokens, kept so an
// aggregator can check itself instead of undercounting quietly.
type Usage struct {
	Prompt     int `json:"prompt"`
	Output     int `json:"output"`
	Thoughts   int `json:"thoughts"`
	Cached     int `json:"cached"`
	ToolPrompt int `json:"tool_prompt"`
	Total      int `json:"total"`
}

// Empty reports a call that spent nothing — nothing to account for.
func (u Usage) Empty() bool { return u.Prompt == 0 && u.Output == 0 && u.Thoughts == 0 }

// Response is the parsed result of one model turn.
type Response struct {
	Content      string
	ToolCalls    []ToolCall
	PromptTokens int
	OutputTokens int
	// CachedTokens counts prompt tokens the server reports as served
	// from its cache; zero when the server reports nothing.
	CachedTokens int
	// FinishReason explains how the turn ended: "stop", "tool_calls",
	// or "length" — the last means the output budget ran out and
	// Content / ToolCalls are what arrived before it did. A partial
	// result is still a result; the caller decides what to do with it.
	FinishReason string
	// ThoughtTokens counts reasoning tokens, reported apart from output.
	ThoughtTokens int
	// ToolPromptTokens is structurally zero (see Usage).
	ToolPromptTokens int
	// TotalTokens is the server's own total_tokens — the checksum for
	// the buckets above.
	TotalTokens int
}

// Usage returns the call's spend as the accounting record shape.
func (r *Response) Usage() Usage {
	if r == nil {
		return Usage{}
	}
	return Usage{Prompt: r.PromptTokens, Output: r.OutputTokens,
		Thoughts: r.ThoughtTokens, Cached: r.CachedTokens,
		ToolPrompt: r.ToolPromptTokens, Total: r.TotalTokens}
}

// Truncated reports a turn cut by the output budget.
func (r *Response) Truncated() bool { return r != nil && r.FinishReason == "length" }

// Backend is one LLM provider. onText receives streamed text deltas as
// they arrive; the returned Response carries the accumulated turn.
type Backend interface {
	ChatStream(ctx context.Context, system string, messages []Message, tools []ToolDef, onText func(string)) (*Response, error)
}

// StreamEvent is turn observability: the backend reports stream
// liveness, retries, and thought summaries to whoever is watching (the
// TUI). Events are display-only — nothing here enters the history or
// the transcript.
type StreamEvent struct {
	// Kind: "chunk" (any chunk arrived — liveness), "thought" (a
	// reasoning delta; Thought carries the text), or "retry" (a backoff
	// retry was scheduled; Attempt/Max/Cause/DelayMS set).
	Kind    string
	Thought string
	Attempt int
	Max     int
	Cause   string
	DelayMS int
}
