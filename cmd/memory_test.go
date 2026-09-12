package cmd

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nlink-jp/lagent/internal/memory"
	"github.com/nlink-jp/lagent/internal/tools"
)

// /remember saves a project memory by default and a global one with
// the keyword; /forget removes; /memory lists what is on disk; the
// fact keeps the operator's spacing (ADR-0013 §3).
func TestMemorySlashCommands(t *testing.T) {
	store := memoryStore{Base: t.TempDir(), Project: t.TempDir()}
	if _, _, handled := memorySlash("/skills", store); handled {
		t.Fatal("an unrelated command was handled")
	}
	out, isErr, handled := memorySlash("/remember staging-host The staging   host is quokka-7", store)
	if !handled || isErr || !strings.Contains(out, "remembered project memory \"staging-host\"") {
		t.Fatalf("remember: %q err=%v handled=%v", out, isErr, handled)
	}
	out, isErr, _ = memorySlash("/remember global lang The user writes in Japanese", store)
	if isErr || !strings.Contains(out, "remembered global memory \"lang\"") {
		t.Fatalf("remember global: %q err=%v", out, isErr)
	}
	mems, _ := store.load()
	if len(mems) != 2 || mems[0].Scope != memory.ScopeGlobal || mems[1].Content != "The staging   host is quokka-7" {
		t.Fatalf("stored: %+v", mems)
	}
	out, isErr, _ = memorySlash("/remember staging-host The staging host is quokka-8", store)
	if isErr || !strings.Contains(out, "updated project memory") {
		t.Fatalf("update: %q err=%v", out, isErr)
	}
	listing, _, _ := memorySlash("/memory", store)
	for _, want := range []string{"[project] staging-host", "[global]  lang", "quokka-8", "/forget"} {
		if !strings.Contains(listing, want) {
			t.Errorf("listing missing %q:\n%s", want, listing)
		}
	}
	if out, isErr, _ := memorySlash("/forget staging-host", store); isErr || !strings.Contains(out, "forgot project memory") {
		t.Fatalf("forget: %q err=%v", out, isErr)
	}
	if _, isErr, _ := memorySlash("/forget staging-host", store); !isErr {
		t.Error("forgetting a missing memory must be an error")
	}
	for _, bad := range []string{"/remember", "/remember onlyname", "/forget", "/forget a b", "/remember Bad-Name fact"} {
		if _, isErr, handled := memorySlash(bad, store); !handled || !isErr {
			t.Errorf("%q: handled=%v isErr=%v, want a usage error", bad, handled, isErr)
		}
	}
	off := memoryStore{}
	if out, isErr, handled := memorySlash("/remember x y", off); !handled || !isErr || !strings.Contains(out, "disabled") {
		t.Errorf("memory off: %q %v %v", out, isErr, handled)
	}
	if lines := off.factsLines(); lines != nil {
		t.Errorf("memory off must add no facts lines: %v", lines)
	}
	if lines := store.factsLines(); len(lines) < 3 || !strings.Contains(strings.Join(lines, "\n"), "[global] lang: The user writes in Japanese") {
		t.Errorf("facts lines: %v", lines)
	}
}

// The model's tools save and delete through the same store, are
// Mutating (so they reach the gate), and answer with the next-session
// caveat.
func TestMemoryToolsAreMutatingAndRoundTrip(t *testing.T) {
	store := memoryStore{Base: t.TempDir(), Project: t.TempDir()}
	reg, err := tools.New(t.TempDir(), nil, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err := registerMemoryTools(reg, store); err != nil {
		t.Fatal(err)
	}
	save, _ := reg.Get("save_memory")
	del, _ := reg.Get("delete_memory")
	if save == nil || del == nil || !save.Mutating || !del.Mutating {
		t.Fatal("memory tools must be registered and Mutating")
	}
	out, err := save.Run(context.Background(), map[string]any{"scope": "project", "name": "warm", "content": "run make warm before make check"})
	if err != nil || !strings.Contains(out, "saved project memory \"warm\"") || !strings.Contains(out, "next session") {
		t.Fatalf("save: %q %v", out, err)
	}
	mems, _ := store.load()
	if len(mems) != 1 || !strings.HasPrefix(mems[0].Path, store.Base) {
		t.Fatalf("stored: %+v", mems)
	}
	if _, err := os.Stat(mems[0].Path); err != nil {
		t.Fatal(err)
	}
	out, err = del.Run(context.Background(), map[string]any{"scope": "project", "name": "warm"})
	if err != nil || !strings.Contains(out, "deleted project memory") {
		t.Fatalf("delete: %q %v", out, err)
	}
	if _, err := del.Run(context.Background(), map[string]any{"scope": "project", "name": "warm"}); err == nil {
		t.Error("deleting a missing memory must fail")
	}
}
