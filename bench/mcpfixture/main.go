// Command bench-mcp is the bench's stdio MCP server (ADR-0006 §3): one
// tool, lookup_ip, answering from a canned table, so the MCP task class
// needs neither the network nor an operator's server. Protocol: JSON-RPC
// over stdio, one message per line, the subset lagent's client speaks
// (initialize, notifications/initialized, tools/list, tools/call).
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func main() {
	if err := serve(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "bench-mcp:", err)
		os.Exit(1)
	}
}

type request struct {
	ID     *json.RawMessage `json:"id"`
	Method string           `json:"method"`
	Params json.RawMessage  `json:"params"`
}

// records is the canned table. The addresses are documentation ranges
// (RFC 5737), so no real host is ever described.
var records = map[string]map[string]any{
	"203.0.113.9":  {"ip": "203.0.113.9", "country": "Iceland", "asn": 64500, "org": "Example Net"},
	"198.51.100.7": {"ip": "198.51.100.7", "country": "Uruguay", "asn": 64501, "org": "Example Transit"},
}

const instructions = "Answers where an IP address is located: call lookup_ip with the address and read country, asn and org from the result."

func serve(in io.Reader, out io.Writer) error {
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	w := bufio.NewWriter(out)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var req request
		if err := json.Unmarshal(line, &req); err != nil || req.ID == nil {
			continue // a notification, or noise
		}
		resp := handle(req)
		data, err := json.Marshal(resp)
		if err != nil {
			return err
		}
		if _, err := w.Write(append(data, '\n')); err != nil {
			return err
		}
		if err := w.Flush(); err != nil {
			return err
		}
	}
	return sc.Err()
}

// handle answers one request; the id is echoed as received.
func handle(req request) map[string]any {
	resp := map[string]any{"jsonrpc": "2.0", "id": req.ID}
	switch req.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(req.Params, &p)
		if p.ProtocolVersion == "" {
			p.ProtocolVersion = "2024-11-05"
		}
		resp["result"] = map[string]any{
			"protocolVersion": p.ProtocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "bench-mcp", "version": "0"},
			"instructions":    instructions,
		}
	case "tools/list":
		resp["result"] = map[string]any{"tools": []map[string]any{{
			"name":        "lookup_ip",
			"description": "Look up the country, ASN and organisation of an IP address.",
			"inputSchema": map[string]any{
				"type":       "object",
				"properties": map[string]any{"ip": map[string]any{"type": "string", "description": "IPv4 address"}},
				"required":   []string{"ip"},
			},
		}}}
	case "tools/call":
		var p struct {
			Name string `json:"name"`
			Args struct {
				IP string `json:"ip"`
			} `json:"arguments"`
		}
		_ = json.Unmarshal(req.Params, &p)
		if p.Name != "lookup_ip" {
			resp["error"] = map[string]any{"code": -32602, "message": "unknown tool " + p.Name}
			break
		}
		rec, ok := records[p.Args.IP]
		var text string
		if ok {
			data, _ := json.Marshal(rec)
			text = string(data)
		} else {
			text = fmt.Sprintf(`{"ip":%q,"error":"no record"}`, p.Args.IP)
		}
		resp["result"] = map[string]any{
			"content": []map[string]any{{"type": "text", "text": text}},
			"isError": !ok,
		}
	case "ping":
		resp["result"] = map[string]any{}
	default:
		resp["error"] = map[string]any{"code": -32601, "message": "method not found: " + req.Method}
	}
	return resp
}
