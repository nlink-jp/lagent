// Package diagram draws mermaid fences in a reply as pictures where the
// terminal draws images (ADR-0025), and as box art where it does not
// (ADR-0027). It is a view-layer concern and nothing else: the model is
// never told about it, the transcript keeps the source the model wrote,
// and a fence that cannot be drawn right is shown as source with a
// one-line note.
//
// Both come from mermaid-render, from the same parse: the art is the
// picture's layout on a grid of cells, checked on the grid every time.
// Art bypasses the Markdown renderer — glamour word-wraps code-block
// lines at spaces, which shears box art — so Split returns it as its own
// segments for the TUI to emit verbatim.
//
// Ported from gem-agent internal/diagram at 8d7c78085a84b9b0d97948ea9bdcb238751e934c (v0.85.1), ADR-0001:
// the fence scanner, the note and the picture path. The art path is
// gem-agent ADR-0095's (mermaid-render's RenderText); mermaid-ascii, its
// translation table and its guards were never ported.
package diagram

import (
	"errors"
	"fmt"
	"image"
	"regexp"
	"strings"

	"github.com/charmbracelet/x/ansi"
	mr "github.com/nlink-jp/mermaid-render"
	"github.com/nlink-jp/mermaid-render/raster"
)

var fenceOpen = regexp.MustCompile("^(`{3,}|~{3,})\\s*([A-Za-z0-9_+-]*)")

// Segment is one run of a reply: markdown for the Markdown renderer,
// finished box art the TUI must emit verbatim (Art), or a picture. Source
// is a picture's fence as the model wrote it, so a failure after this
// point still shows the source (WithNote) — the picture never disappears
// together with it.
type Segment struct {
	Text   string
	Art    bool
	Img    image.Image
	Source string
}

// Picture draws one mermaid source as an image. attempted is false for a
// diagram type it does not draw; otherwise why is empty on success and
// names the refusal.
type Picture func(src string) (img image.Image, why string, attempted bool)

// WithNote is a fence shown as source with the one-line note that says
// why it is not drawn: blank lines on both sides, so the note is its own
// paragraph, never a prefix of what follows.
func WithNote(source, why string) string {
	return source + "\n\n*diagram shown as source: " + noteSafe(why) + "*\n"
}

// Split partitions markdown around its ```mermaid blocks and draws them
// with pic. A block pic refuses keeps its fence and gains a note saying
// why — for the reader; the model never sees the screen. A diagram type
// pic does not draw passes through untouched and note-free: a gantt in
// the chat is not an error. A ```mermaid line that is CONTENT of an
// enclosing fence (an example inside a ````markdown block) is data, not a
// diagram — a closing fence carries no info string, so a labeled opener
// inside an open fence can never be its close. An unclosed fence (a reply
// cut off mid-diagram) is text. Without a pic (the session draws no
// images) a fence is drawn as box art instead (ADR-0027); nothing falls
// back from a picture to art.
func Split(markdown string, pic Picture) []Segment {
	if !strings.Contains(strings.ToLower(markdown), "mermaid") {
		return []Segment{{Text: markdown}}
	}
	lines := strings.Split(markdown, "\n")
	var segs []Segment
	var md []string
	flush := func() {
		if len(md) > 0 {
			segs = append(segs, Segment{Text: strings.Join(md, "\n")})
			md = nil
		}
	}
	enclosing := "" // opener of the non-mermaid fence we are inside
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if enclosing != "" {
			md = append(md, line)
			if closesFence(line, enclosing) {
				enclosing = ""
			}
			continue
		}
		m := fenceOpen.FindStringSubmatch(line)
		if m == nil {
			md = append(md, line)
			continue
		}
		if !strings.EqualFold(m[2], "mermaid") {
			enclosing = m[1]
			md = append(md, line)
			continue
		}
		fence := m[1]
		end := -1
		for j := i + 1; j < len(lines); j++ {
			if closesFence(lines[j], fence) {
				end = j
				break
			}
		}
		if end < 0 {
			md = append(md, line)
			continue
		}
		src := strings.Join(lines[i+1:end], "\n")
		block := strings.Join(lines[i:end+1], "\n")
		if pic == nil {
			art, why, attempted := render(src)
			switch {
			case attempted && why == "":
				flush()
				segs = append(segs, Segment{Text: art, Art: true})
			case attempted:
				md = append(md, WithNote(block, why))
			default:
				md = append(md, block)
			}
			i = end
			continue
		}
		img, why, attempted := drawPicture(pic, src)
		switch {
		case attempted && why == "" && img != nil:
			flush()
			segs = append(segs, Segment{Img: img, Source: block})
		case attempted:
			if why == "" {
				why = "the renderer returned no picture"
			}
			md = append(md, WithNote(block, why))
		default:
			md = append(md, block)
		}
		i = end
	}
	flush()
	return segs
}

// drawPicture calls pic, turning a panic into a refusal: the engine runs
// on the UI's update path, and a defect in it must cost a picture, not the
// session.
func drawPicture(pic Picture, src string) (img image.Image, why string, attempted bool) {
	defer func() {
		if r := recover(); r != nil {
			img, why, attempted = nil, fmt.Sprintf("the diagram renderer failed: %v", r), true
		}
	}()
	return pic(src)
}

// closesFence reports whether line closes a fence opened by opener:
// the same character, at least as long, and nothing else on the line.
func closesFence(line, opener string) bool {
	if !strings.HasPrefix(line, opener) {
		return false
	}
	t := strings.TrimSpace(line)
	return t == strings.Repeat(opener[:1], len(t))
}

// noteSafe keeps a reason from breaking the note's emphasis markup —
// a renderer error can contain any character.
var noteSafe = strings.NewReplacer("*", "＊", "_", "＿", "`", "'").Replace

// render draws one mermaid source as box art (ADR-0027). attempted is
// false for a diagram type the engine does not draw as text (a gantt in
// the chat is not an error). For an attempted draw, why is empty on
// success and names the refusal otherwise — wrongness only, never size.
// A panic in the engine is a refusal: it runs on the UI's update path,
// and a defect in it must cost a drawing, not the session.
func render(src string) (art, why string, attempted bool) {
	defer func() {
		if r := recover(); r != nil {
			art, why, attempted = "", fmt.Sprintf("the diagram renderer failed: %v", r), true
		}
	}()
	d, err := mr.Parse(src)
	if err == nil {
		art, err = raster.RenderText(d, raster.TextOptions{Width: cellWidth})
	}
	if err != nil {
		var e *mr.Error
		if errors.As(err, &e) && e.Kind == mr.UnsupportedType {
			return "", "", false
		}
		return "", strings.ReplaceAll(err.Error(), "\n", " "), true
	}
	return art, "", true
}

// cellWidth is a character's width as the TUI measures it, so the art's
// columns and the rest of the screen agree.
func cellWidth(r rune) int { return ansi.StringWidth(string(r)) }
