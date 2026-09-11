package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestServeHandshakeListAndCall(t *testing.T) {
	in := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":"c3","method":"tools/call","params":{"name":"lookup_ip","arguments":{"ip":"203.0.113.9"}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"lookup_ip","arguments":{"ip":"192.0.2.1"}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"resources/list"}`,
	}, "\n") + "\n"
	var out bytes.Buffer
	if err := serve(strings.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 5 {
		t.Fatalf("expected 5 responses (the notification gets none), got %d:\n%s", len(lines), out.String())
	}
	var init struct {
		ID     int `json:"id"`
		Result struct {
			ProtocolVersion string `json:"protocolVersion"`
			Instructions    string `json:"instructions"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &init); err != nil || init.ID != 1 ||
		init.Result.ProtocolVersion != "2025-03-26" || !strings.Contains(init.Result.Instructions, "lookup_ip") {
		t.Errorf("initialize: %s (%v)", lines[0], err)
	}
	if !strings.Contains(lines[1], `"name":"lookup_ip"`) || !strings.Contains(lines[1], `"required":["ip"]`) {
		t.Errorf("tools/list: %s", lines[1])
	}
	if !strings.Contains(lines[2], `"id":"c3"`) || !strings.Contains(lines[2], `Iceland`) || !strings.Contains(lines[2], `"isError":false`) {
		t.Errorf("tools/call hit: %s", lines[2])
	}
	if !strings.Contains(lines[3], `no record`) || !strings.Contains(lines[3], `"isError":true`) {
		t.Errorf("tools/call miss: %s", lines[3])
	}
	if !strings.Contains(lines[4], `-32601`) {
		t.Errorf("unknown method: %s", lines[4])
	}
}

func TestHandleUnknownTool(t *testing.T) {
	id := json.RawMessage(`7`)
	resp := handle(request{ID: &id, Method: "tools/call", Params: json.RawMessage(`{"name":"nope","arguments":{}}`)})
	if resp["error"] == nil {
		t.Errorf("unknown tool must be a JSON-RPC error: %v", resp)
	}
}
