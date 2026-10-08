package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/nlink-jp/lagent/internal/bounded"
)

// RulebookFileName is the operator's risk rulebook beside config.toml
// (ADR-0032; gem-agent ADR-0050's base layer). lagent reads it and never
// writes it: writing it is the operator's deliberate act.
const RulebookFileName = "risk-rules.md"

// RulebookCap bounds the rulebook the judge reads, in runes. It rides
// every judgment, so it pays its way; a clip is disclosed to the judge
// in the text and to the operator as a notice.
const RulebookCap = 4000

// rulebookReadCap bounds the file itself: a larger one is not a
// rulebook (gem-agent's limit).
const rulebookReadCap = 1 << 20

// RulebookPath is the rulebook's path for a config path.
func RulebookPath(cfgPath string) string {
	return filepath.Join(filepath.Dir(cfgPath), RulebookFileName)
}

// LoadRulebook reads the rulebook beside cfgPath and composes the text
// the judge reads, as gem-agent's riskbook.Compose does for its base
// layer: trimmed, under a provenance header, clipped at RulebookCap
// runes with the clip stated in the text. A missing or blank file is
// normal and returns "". clipped reports the clip so the caller can tell
// the operator too.
func LoadRulebook(cfgPath string) (text string, clipped bool, err error) {
	f, err := os.Open(RulebookPath(cfgPath))
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("risk rulebook: %w", err)
	}
	defer func() { _ = f.Close() }()
	data, more, err := bounded.ReadAll(f, rulebookReadCap)
	if err != nil {
		return "", false, fmt.Errorf("risk rulebook: %w", err)
	}
	if more {
		return "", false, fmt.Errorf("risk rulebook: %s is larger than %d bytes", RulebookPath(cfgPath), rulebookReadCap)
	}
	body := strings.TrimSpace(string(data))
	if body == "" {
		return "", false, nil
	}
	if r := []rune(body); len(r) > RulebookCap {
		body = string(r[:RulebookCap]) + fmt.Sprintf("\n[clipped: %d more runes not shown]", len(r)-RulebookCap)
		clipped = true
	}
	return "== base rules (hand-written by the operator) ==\n" + body, clipped, nil
}
