package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nlink-jp/lagent/internal/llm"
	"github.com/nlink-jp/lagent/internal/tools"
)

// savedFiles converts the intake's records to the transcript's shape,
// naming each file as the file tools accept it in this session's work
// directory (ADR-0039 §7.1). The name is fixed here, with the work
// directory the result was saved in, so a later note never re-derives it
// against another one.
func savedFiles(in []tools.Saved, workDir string) []llm.SavedFile {
	if len(in) == 0 {
		return nil
	}
	out := make([]llm.SavedFile, len(in))
	for i, s := range in {
		out[i] = llm.SavedFile{Path: s.Path, Bytes: s.Bytes, SHA256: s.SHA256, Error: s.Err}
		if s.Path != "" {
			if ref := tools.WorkDirName(workDir, s.Path); ref != s.Path {
				out[i].Ref = ref
			}
		}
	}
	return out
}

// runtimeSuffix is what the runtime appends to a tool result outside
// the nonce tag at send time, in this order: the runtime note of
// gem-agent ADR-0075 §3 (a remote tool's repeated identical failure) and
// the note of what was saved (ADR-0039 §1, §3). Provenance is the field;
// a result whose text merely looks like either note stays wrapped. It is
// the one definition the wrap, the round budget and the compaction
// estimate share, so the bytes the model receives are the bytes counted.
func runtimeSuffix(m llm.Message) string {
	var b strings.Builder
	if m.RuntimeNote != "" {
		b.WriteString("\n\n" + m.RuntimeNote)
	}
	if note := savedNote(m.Saved); note != "" {
		b.WriteString("\n\n" + note)
	}
	return b.String()
}

// savedNote is lagent's own line about what it saved for a result
// (ADR-0039 §1, §3; ported from gem-agent ADR-0105 at edd8e60). It
// rides outside the nonce tag, like gem-agent ADR-0075's runtime note,
// because it is the runtime's word: a server can print a line shaped
// like it inside the result, and that line stays wrapped.
// It offers the route — a copy by write_file `from` — at the moment the
// result arrives, and says so plainly when there is nothing to copy.
//
// A result split across files (the inline blocks, a spilled block, the
// rest past the budget, an image) is never called whole: each line names
// its part, and copying the whole result means copying every part
// (independent review — under ADR-0034's allotment a split is common, and
// a part announced as "whole" is the lost rows ADR-0039 exists to stop).
func savedNote(files []llm.SavedFile) string {
	var lines []string
	n := len(files)
	for i, f := range files {
		what := "this result"
		if n > 1 {
			what = fmt.Sprintf("part %d of %d of this result", i+1, n)
		}
		if f.Path == "" {
			lines = append(lines, fmt.Sprintf(
				"[lagent could not save %s (%s), so there is no file to copy]", what, f.Error))
			continue
		}
		name := f.Ref
		if name == "" {
			name = f.Path
		}
		// The call is spelled out (ADR-0039 §7.2): measured, the model
		// reached for mkdir and cp first in 18 of 20 runs, and the words
		// name what makes them unnecessary rather than forbid them.
		from := jsonString(name)
		if n > 1 {
			lines = append(lines, fmt.Sprintf(
				`[lagent saved %s as %s (%d bytes, sha256 %s). To keep the whole result as evidence, copy every part: write_file {"path": "<destination>", "from": %s} for each]`,
				what, name, f.Bytes, f.SHA256, from))
			continue
		}
		lines = append(lines, fmt.Sprintf(
			`[lagent saved this result whole as %s (%d bytes, sha256 %s). To keep it as evidence, copy the file instead of re-typing it: write_file {"path": "<destination>", "from": %s} copies it byte for byte and creates the parent directories — no shell command is needed]`,
			name, f.Bytes, f.SHA256, from))
	}
	return strings.Join(lines, "\n")
}

// jsonString is s as a JSON string literal, as the model would write it
// in the call the note spells out — without the HTML escaping
// encoding/json applies by default, which would print a path's `&` or
// `<` as \u0026 and \u003c.
func jsonString(s string) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	return strings.TrimSuffix(b.String(), "\n")
}
