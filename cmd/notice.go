// Ported from gem-agent cmd/notice.go at 386657fdbddba8703cb7d9fe2b2d979365db7e77
// (gem-agent ADR-0104, this runtime's ADR-0038).

package cmd

import "github.com/nlink-jp/lagent/internal/tui"

// noticeWriter is the TUI's waiting-dialog notifier for a [tui].notify
// value (ADR-0038): it writes that value's fixed bytes through inject,
// which lands them between two renderer flushes. nil for "off", so the
// TUI schedules nothing at all.
func noticeWriter(mode string, inject func(string) error) func() {
	seq := tui.Notice(mode)
	if seq == "" {
		return nil
	}
	return func() { _ = inject(seq) }
}
