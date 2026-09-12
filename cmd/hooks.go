package cmd

import (
	"time"

	"github.com/nlink-jp/lagent/internal/config"
	"github.com/nlink-jp/lagent/internal/hooks"
)

// hookEntries converts the configured pre-tool hooks into the runner's
// shape (ADR-0012).
func hookEntries(es []config.HookEntry) []hooks.Hook {
	if len(es) == 0 {
		return nil
	}
	hs := make([]hooks.Hook, 0, len(es))
	for _, e := range es {
		hs = append(hs, hooks.Hook{
			Matcher: e.Matcher, Command: e.Command,
			Timeout: time.Duration(e.TimeoutSec) * time.Second,
		})
	}
	return hs
}
