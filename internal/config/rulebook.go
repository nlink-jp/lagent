package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/nlink-jp/lagent/internal/bounded"
)

// RulebookFileName is the operator's risk rulebook beside config.toml
// (ADR-0032; gem-agent ADR-0050's base layer). lagent reads it and never
// writes it: writing it is the operator's deliberate act.
const RulebookFileName = "risk-rules.md"

// RulebookCap bounds the rulebook, in runes. It rides every judgment
// the model tier makes, so it pays its way; a clip is disclosed.
const RulebookCap = 4000

// rulebookReadCap bounds the read itself, in bytes: four bytes a rune
// at most, so a file within RulebookCap runes is always read whole.
const rulebookReadCap = 4 * RulebookCap

// RulebookPath is the rulebook's path for a config path.
func RulebookPath(cfgPath string) string {
	return filepath.Join(filepath.Dir(cfgPath), RulebookFileName)
}

// LoadRulebook reads the rulebook beside cfgPath. A missing file is
// normal and returns "". clipped reports that the text was cut at
// RulebookCap runes, so the caller can say so.
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
	r := []rune(string(bounded.TrimIncompleteRune(data)))
	if len(r) > RulebookCap {
		return string(r[:RulebookCap]), true, nil
	}
	return string(r), more, nil
}
