package tools

import (
	"context"
	"fmt"
)

// The allotment (ADR-0034 §2) is the runtime's upper bound on the bytes
// one call returns inline, below the tool's own cap: one round's results
// share a budget measured against the context window, and each call is
// handed its part before it runs. In-process it rides the call's
// context; the sandboxed file child, which a context value never
// reaches, is handed it explicitly (FileChildFunc's caller copies it
// into the request).

type allotKey struct{}

// WithAllot returns ctx carrying an allotment of n bytes. A negative n
// is clamped to zero: an allotment is a bound, and a call with none
// left still returns its marker and route.
func WithAllot(ctx context.Context, n int) context.Context {
	return context.WithValue(ctx, allotKey{}, max(n, 0))
}

// Allot reports the allotment ctx carries; ok is false when the runtime
// set none (no known window), and the tool's own cap applies alone.
func Allot(ctx context.Context) (n int, ok bool) {
	n, ok = ctx.Value(allotKey{}).(int)
	return n, ok
}

// capFor is the inline bound a tool applies: its own cap, or the
// allotment when that is smaller.
func capFor(ctx context.Context, own int) int {
	if n, ok := Allot(ctx); ok && n < own {
		return n
	}
	return own
}

// narrowRoute is the route a navigation tool's cut names: these tools
// have no paging, so the way to the rest is a narrower question.
const narrowRoute = " — narrow the path or pattern to see the rest"

// truncateNarrow is truncate with narrowRoute: the cut of a listing or
// a search, whose rest is reached by asking for less.
func truncateNarrow(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	cut := cutRunes(s, limit)
	return cut + fmt.Sprintf("\n[output truncated: %d of %d bytes shown%s]", len(cut), len(s), narrowRoute)
}

// CutUnkept is the runtime's backstop (ADR-0034 §2): a result still
// longer than its allotment — from a tool that did not apply it — is
// cut, and the marker says the rest was not kept, because no route to
// it exists.
func CutUnkept(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	cut := cutRunes(s, max(limit, 0))
	return cut + fmt.Sprintf("\n[output truncated by lagent: %d of %d bytes shown; the rest was not kept]", len(cut), len(s))
}
