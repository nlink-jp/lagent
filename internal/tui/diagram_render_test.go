package tui

import (
	"image"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nlink-jp/lagent/internal/diagram"
	"github.com/nlink-jp/lagent/internal/termimg"
)

// replyModel is a plain-styled model at width 80 with the real renderer.
func replyModel(opts Options) *Model {
	opts.Theme = "notty"
	m := New(opts)
	return &m
}

// replyText is what renderReply sends to scrollback, joined as emitted.
func replyText(segs []Segment) string {
	parts := make([]string, len(segs))
	for i, s := range segs {
		parts[i] = s.Text
	}
	return strings.Join(parts, "\n")
}

// emPicture is a blank picture w x h terminal lines at the diagram scale.
func emPicture(w, h int) image.Image {
	return image.NewRGBA(image.Rect(0, 0, w*termimg.DiagramPxPerEm*12/10, h*termimg.DiagramPxPerEm*12/10))
}

// fakePicture draws every fence as a 20 x 10 line picture, one that says
// "tall" as 20 x 100, and refuses one that says so.
func fakePicture(src string) (image.Image, string, bool) {
	switch {
	case strings.Contains(src, "refuse"):
		return nil, "syntax error: refused", true
	case strings.Contains(src, "tall"):
		return emPicture(20, 100), "", true
	}
	return emPicture(20, 10), "", true
}

// Where the session draws images, a fence becomes a picture in a
// declared box (ADR-0025): one em of diagram text per terminal line, the
// width from the cell's aspect, blank lines around it as between
// paragraphs, and its rows told to the counter.
func TestReplyDrawsPicture(t *testing.T) {
	m := replyModel(Options{Images: termimg.ITerm2, Picture: fakePicture,
		CellAspect: func() (float64, bool) { return 1.86, true }})
	m.aspect = 1.86 // as the first size report reads it
	segs := m.renderReply("before\n\n```mermaid\nflowchart TD\n  A --> B\n```\n\nafter")
	if len(segs) != 5 || segs[1].Text != "" || segs[3].Text != "" {
		t.Fatalf("segments: %+v", segs)
	}
	pic := segs[2]
	if pic.Rows != 10 || !strings.HasPrefix(pic.Text, "\x1b]1337;File=") || !strings.Contains(pic.Text, "width=37;height=10;") {
		t.Errorf("picture segment: rows %d, %q", pic.Rows, pic.Text[:min(80, len(pic.Text))])
	}
	if !strings.Contains(segs[0].Text, "before") || !strings.Contains(segs[4].Text, "after") {
		t.Errorf("text around the picture: %q, %q", segs[0].Text, segs[4].Text)
	}
}

// A session without an image protocol draws the fence as box art whatever
// Picture holds (ADR-0027 §1); a refusal where images draw shows the
// source with the note, never art.
func TestReplyPictureOnlyWhereImagesDraw(t *testing.T) {
	src := "```mermaid\nflowchart TD\n  A[alpha] --> B[beta]\n```"
	out := replyText(replyModel(Options{Picture: fakePicture}).renderReply(src))
	if strings.Contains(out, "A[alpha]") || strings.Contains(out, "\x1b]1337") || !strings.Contains(out, "┌") || !strings.Contains(out, "alpha") {
		t.Errorf("no protocol: want box art, got\n%s", out)
	}
	refused := "```mermaid\nflowchart TD\n  A[refuse] --> B\n```"
	out = replyText(replyModel(Options{Images: termimg.Kitty, Picture: fakePicture}).renderReply(refused))
	if !strings.Contains(out, "A[refuse]") || strings.Contains(out, "┌") || !strings.Contains(out, "diagram shown as source: syntax error: refused") {
		t.Errorf("refusal: want the source and the note, got\n%s", out)
	}
}

// A picture that cannot become payloads after the fence was replaced —
// here, over the byte limit once encoded — shows the source with the
// note: the picture never vanishes with its source.
func TestReplyPictureFailureShowsSource(t *testing.T) {
	noisy := func(string) (image.Image, string, bool) {
		img := image.NewRGBA(image.Rect(0, 0, 1200, 1200))
		r := rand.New(rand.NewSource(1))
		r.Read(img.Pix)
		return img, "", true
	}
	m := replyModel(Options{Images: termimg.ITerm2, Picture: noisy})
	out := replyText(m.renderReply("```mermaid\nflowchart TD\n  A --> B\n```"))
	if !strings.Contains(out, "A --> B") || !strings.Contains(out, "diagram shown as source: the picture is over") {
		t.Errorf("want the source and the note, got\n%.300s", out)
	}
}

// A picture taller than half the screen is drawn as bands of at most half
// the screen, back to back, their rows adding up to the whole: kitty clips
// a picture taller than the screen (measured, gem-agent ADR-0092 §4).
func TestTallPictureIsDrawnInBands(t *testing.T) {
	m := replyModel(Options{Images: termimg.Kitty, Picture: fakePicture})
	m.height = 40
	segs := m.renderReply("```mermaid\nflowchart TD\n  tall --> B\n```")
	rows, cols := 0, ""
	rc := regexp.MustCompile(`r=(\d+),c=(\d+)`)
	for _, s := range segs {
		if s.Rows == 0 || s.Rows > 20 || !strings.HasPrefix(s.Text, "\x1b_G") {
			t.Fatalf("segment %+.60v: want kitty bands of at most 20 rows, back to back", s)
		}
		// What the terminal is told is what the counter is told, and every
		// band has the picture's columns.
		m := rc.FindStringSubmatch(s.Text)
		if m == nil || m[1] != strconv.Itoa(s.Rows) || (cols != "" && m[2] != cols) {
			t.Fatalf("band declares %q to the terminal, %d rows to the counter, columns before %q", m, s.Rows, cols)
		}
		cols = m[2]
		rows += s.Rows
	}
	if len(segs) != 5 || rows != 100 {
		t.Errorf("%d bands, %d rows; want 5 and 100", len(segs), rows)
	}
}

// A streamed reply with a diagram flushes as ONE write, the picture's
// declared rows told to the counter and the lines that follow it after
// it (ADR-0025 §2): the flush a tool call triggers mid-reply included.
func TestFlushedPictureIsCounted(t *testing.T) {
	c := &capture{}
	m := New(Options{Theme: "notty", Images: termimg.ITerm2, Picture: fakePicture, Printer: c.printer,
		RenderFactory: func(int) func(string) string { return func(s string) string { return s } }})
	m.live.WriteString("text\n\n```mermaid\nflowchart TD\n  A --> B\n```")
	before := m.hold.printed
	m.emitAfterLive(m.takeLive(), "tail")
	if len(c.printed) != 1 {
		t.Fatalf("%d writes, want one", len(c.printed))
	}
	out := c.printed[0]
	pic, tail := strings.Index(out, "\x1b]1337;File="), strings.Index(out, "tail")
	if pic < 0 || tail < pic {
		t.Errorf("want the picture, then the tail: %q", out)
	}
	// "text", a blank line, the picture's 10 rows, "tail".
	if got := m.hold.printed - before; got != 13 {
		t.Errorf("counted %d rows, want 13", got)
	}
}

// The cell's aspect is read at every size report (ADR-0025 §3), and a
// terminal that reports no pixels leaves the last reading.
func TestSizeReportReadsCellAspect(t *testing.T) {
	aspect, ok := 1.86, true
	m := New(Options{Theme: "notty", Printer: (&capture{}).printer,
		CellAspect: func() (float64, bool) { return aspect, ok }})
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = next.(Model)
	if m.aspect != 1.86 {
		t.Fatalf("aspect after the first size report = %v", m.aspect)
	}
	aspect, ok = 0, false
	next, _ = m.Update(tea.WindowSizeMsg{Width: 90, Height: 40})
	if got := next.(Model).aspect; got != 1.86 {
		t.Errorf("a report without pixels changed the aspect to %v", got)
	}
}

// iTerm2 gets a tall picture whole: it scrolls one correctly, and bands
// there showed seams and a missing corner (measured, gem-agent ADR-0092 §4).
func TestTallPictureIsWholeOnITerm2(t *testing.T) {
	m := replyModel(Options{Images: termimg.ITerm2, Picture: fakePicture})
	m.height = 40
	segs := m.renderReply("```mermaid\nflowchart TD\n  tall --> B\n```")
	if len(segs) != 1 || segs[0].Rows != 100 {
		t.Errorf("%d segments (first %d rows), want one of 100", len(segs), segs[0].Rows)
	}
}

// The byte limit is on the bands together: each band of this noisy tall
// picture is under 2 MiB, the whole is not.
func TestBandsShareTheByteLimit(t *testing.T) {
	noisy := func(string) (image.Image, string, bool) {
		img := image.NewRGBA(image.Rect(0, 0, 600, 3000))
		r := rand.New(rand.NewSource(1))
		r.Read(img.Pix)
		return img, "", true
	}
	m := replyModel(Options{Images: termimg.Kitty, Picture: noisy})
	m.width, m.height = 200, 20
	out := replyText(m.renderReply("```mermaid\nflowchart TD\n  A --> B\n```"))
	if !strings.Contains(out, "diagram shown as source: the picture is over") {
		t.Errorf("want the source and the note, got\n%.200s", out)
	}
}

// A reply that renders to nothing prints nothing, as before the reply was
// split: a comment or a link definition alone is not a blank line
// (independent review; the terminal without images is unchanged).
func TestReplyRenderingToNothingPrintsNothing(t *testing.T) {
	for _, images := range []termimg.Protocol{termimg.None, termimg.ITerm2} {
		c := &capture{}
		m := New(Options{Theme: "notty", Images: images, Picture: fakePicture, Printer: c.printer})
		for _, reply := range []string{"<!-- c -->", "[a]: http://example.com"} {
			m.live.WriteString(reply)
			before := m.hold.printed
			m.emitAfterLive(m.takeLive())
			if got := m.hold.printed - before; got != 0 || len(c.printed) != 0 {
				t.Errorf("images %v, %q: %d rows, writes %q", images, reply, got, c.printed)
			}
		}
	}
}

// Box art reaches scrollback as the engine drew it, past glamour (which
// wraps code lines at spaces and would shear a wide drawing), and held to
// no escapes (ADR-0027 §2–§3).
func TestReplyArtBypassesTheRenderer(t *testing.T) {
	md := "```mermaid\ngraph LR\n  A[Parse config] --> B[Resolve project] --> C[Connect MCP] --> D[Discover skills] --> E[Build prompt] --> F[Start TUI]\n```\n"
	var art string
	for _, seg := range diagram.Split(md, nil) {
		if seg.Art {
			art = seg.Text
		}
	}
	if art == "" {
		t.Fatal("no art segment for the wide chain")
	}
	if out := replyText(replyModel(Options{}).renderReply(md)); !strings.Contains(out, art) {
		t.Fatalf("wide art did not pass through verbatim at width 80:\n%s", out)
	}
	hostile := "```mermaid\nsequenceDiagram\n  A->>B: x\x1b]0;PWN\x07y\n```\n"
	for _, seg := range diagram.Split(hostile, nil) {
		if seg.Art {
			t.Fatalf("the engine drew a control character:\n%q", seg.Text)
		}
	}
	if got := inertArt("┌─┐\x1b]0;PWN\x07"); strings.ContainsRune(got, 0x1b) || strings.ContainsRune(got, 0x07) {
		t.Errorf("inertArt kept an escape: %q", got)
	}
}
