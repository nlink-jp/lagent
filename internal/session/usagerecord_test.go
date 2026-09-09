package session

import (
	"encoding/json"
	"strings"
	"testing"
)

// tool_prompt is written ALWAYS, zero included. gem-usage-lens reads a
// missing key as "derive the bucket from the remainder" and a zero as
// "trust it"; omitempty would fold the two into one. The key sits before
// total — addends before the checksum — so the line reads as the
// equation it is.
func TestUsageRecordAlwaysWritesToolPrompt(t *testing.T) {
	b, err := json.Marshal(UsageRecord{Source: UsageMain, Model: "m",
		Prompt: 10, Output: 2, Thoughts: 3, Cached: 4, Total: 15})
	if err != nil {
		t.Fatal(err)
	}
	line := string(b)
	if !strings.Contains(line, `"tool_prompt":0`) {
		t.Errorf("a zero tool_prompt was omitted: %s", line)
	}
	if strings.Index(line, `"tool_prompt"`) > strings.Index(line, `"total"`) {
		t.Errorf("tool_prompt should precede total: %s", line)
	}

	// The four-bucket shape round-trips intact, a non-zero tool_prompt
	// included: lagent never writes one, but the record type is the
	// lens's shape for both runtimes and must carry it.
	in := UsageRecord{Source: UsageMain, Model: "m",
		Prompt: 1200, Output: 900, Thoughts: 40, ToolPrompt: 7000, Total: 9140}
	b, _ = json.Marshal(in)
	var out UsageRecord
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out != in {
		t.Errorf("round trip = %+v, want %+v", out, in)
	}
	if out.Prompt+out.Output+out.Thoughts+out.ToolPrompt != out.Total {
		t.Errorf("checksum broken on %s", b)
	}
}
