package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"
)

// instructionsServer is a stand-in that publishes text when it is opened:
// in the legacy era in its initialize result, in the modern era in its
// server/discover result.
func instructionsServer(modern bool, text string) spawnFunc {
	return func() (io.WriteCloser, io.ReadCloser, func(), error) {
		inR, inW := io.Pipe()
		outR, outW := io.Pipe()
		go func() {
			sc := bufio.NewScanner(inR)
			sc.Buffer(make([]byte, 64*1024), scannerMax)
			for sc.Scan() {
				var msg message
				if json.Unmarshal(sc.Bytes(), &msg) != nil || msg.ID == nil || msg.Method == "" {
					continue
				}
				var result any
				switch msg.Method {
				case "server/discover":
					if !modern {
						refuseDiscover(outW, *msg.ID)
						continue
					}
					result = map[string]any{
						"supportedVersions": []string{"2026-07-28"},
						"capabilities":      map[string]any{"tools": map[string]any{}},
						"instructions":      text,
						"_meta": map[string]any{
							"io.modelcontextprotocol/serverInfo": map[string]any{"name": "fake", "version": "0"},
						},
					}
				case "initialize":
					result = map[string]any{
						"protocolVersion": legacyVersion,
						"capabilities":    map[string]any{},
						"serverInfo":      map[string]any{"name": "fake", "version": "0"},
						"instructions":    text,
					}
				default:
					result, _ = stdResult(msg.Method)
				}
				resp, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": *msg.ID, "result": result})
				_, _ = outW.Write(append(resp, '\n'))
			}
		}()
		return inW, outR, func() { _ = inR.Close(); _ = outW.Close() }, nil
	}
}

// The text a server publishes reaches Instructions in both eras, trimmed,
// and clipped to 2000 runes with "…" when it is longer (ADR-0056 §5).
func TestInstructionsInBothEras(t *testing.T) {
	exact := strings.Repeat("あ", instructionsCap)
	cases := []struct {
		name, text, want string
	}{
		{"trimmed", "  Look up an address.\n", "Look up an address."},
		{"exactly the cap", exact, exact},
		{"over the cap", exact + "い", exact + "…"},
		{"none", "", ""},
	}
	for _, era := range []struct {
		name   string
		modern bool
	}{{"legacy", false}, {"modern", true}} {
		for _, tc := range cases {
			t.Run(era.name+"/"+tc.name, func(t *testing.T) {
				c := newClient("fake", instructionsServer(era.modern, tc.text), 2*time.Second, 2*time.Second, "test", "")
				defer c.Close()
				if _, err := c.ListTools(context.Background()); err != nil {
					t.Fatal(err)
				}
				if got := c.Instructions(); got != tc.want {
					t.Errorf("Instructions() = %q (%d runes), want %q (%d runes)",
						clipForLog(got), len([]rune(got)), clipForLog(tc.want), len([]rune(tc.want)))
				}
			})
		}
	}
}

func clipForLog(s string) string {
	if r := []rune(s); len(r) > 20 {
		return string(r[:20]) + "..."
	}
	return s
}
