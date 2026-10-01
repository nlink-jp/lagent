package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func call(t *testing.T, name, args string) (string, bool) {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(args), &m); err != nil {
		t.Fatal(err)
	}
	text, isErr, known := callTicketTool(name, m)
	if !known {
		t.Fatalf("%s: unknown tool", name)
	}
	return text, isErr
}

// TestTicketsAnswers pins the answers the mcp-load tasks expect: the
// right arguments reach them, and the common slips do not.
func TestTicketsAnswers(t *testing.T) {
	cases := []struct {
		name, tool, args string
		want             []string // substrings of a successful result
		notWant          []string
	}{
		{"list: open high ALPHA newest two", "list_tickets",
			`{"project":"ALPHA","status":"open","priority":"high","sort":"created_desc","limit":2}`,
			[]string{"ALPHA-17", "ALPHA-12"}, []string{"ALPHA-9", "ALPHA-18", "ALPHA-3"}},
		{"list: default sort is oldest first", "list_tickets",
			`{"project":"ALPHA","status":"open","priority":"high","limit":2}`,
			[]string{"ALPHA-3", "ALPHA-9"}, []string{"ALPHA-17"}},
		{"search: BETA timeout after 2026-09-01", "search_tickets",
			`{"query":"timeout","project":"BETA","created_after":"2026-09-01"}`,
			[]string{"BETA-8", "BETA-11"}, []string{"BETA-5", "ALPHA-12"}},
		{"bulk read", "get_tickets", `{"ids":["ALPHA-3","BETA-8","ALPHA-17"]}`,
			[]string{"kato", "ueda", "mori"}, nil},
		{"create: the task's ticket", "create_ticket",
			`{"project":"BETA","title":"Rotate the signing key","fields":{"priority":"critical","labels":["security","ops"],"due_date":"2026-11-30"}}`,
			[]string{`"id":"` + expectedCreateID + `"`}, nil},
		{"create: label order does not matter", "create_ticket",
			`{"project":"BETA","title":"Rotate the signing key","fields":{"priority":"critical","labels":["ops","security"],"due_date":"2026-11-30"}}`,
			[]string{expectedCreateID}, nil},
		{"create: a missing label is another ticket", "create_ticket",
			`{"project":"BETA","title":"Rotate the signing key","fields":{"priority":"critical","labels":["security"],"due_date":"2026-11-30"}}`,
			nil, []string{expectedCreateID}},
		{"close as duplicate", "update_status", `{"id":"ALPHA-12","status":"closed","resolution":"duplicate"}`,
			[]string{expectedCloseEvent}, nil},
		{"close as fixed is another event", "update_status", `{"id":"ALPHA-12","status":"closed","resolution":"fixed"}`,
			nil, []string{expectedCloseEvent}},
	}
	for _, c := range cases {
		text, isErr := call(t, c.tool, c.args)
		if isErr {
			t.Errorf("%s: refused: %s", c.name, text)
			continue
		}
		for _, w := range c.want {
			if !strings.Contains(text, w) {
				t.Errorf("%s: want %q in %s", c.name, w, text)
			}
		}
		for _, w := range c.notWant {
			if strings.Contains(text, w) {
				t.Errorf("%s: did not want %q in %s", c.name, w, text)
			}
		}
	}
}

// The answers the task files' regexes name; TestTicketsAnswers derives
// them from the server, so a change to the id formula fails here first.
const (
	expectedCreateID   = "BETA-606"
	expectedCloseEvent = "EV-4199"
)

// TestTicketsRefusals: the slips a model makes are refused with the
// marker the bench counts, naming the field.
func TestTicketsRefusals(t *testing.T) {
	cases := []struct{ tool, args, field string }{
		{"list_tickets", `{"project":"ALPHA","status":"OPEN"}`, "arguments.status"},
		{"list_tickets", `{"project":"ALPHA"}`, "arguments.status"},
		{"list_tickets", `{"project":"ALPHA","status":"open","limit":"2"}`, "arguments.limit"},
		{"list_tickets", `{"project":"ALPHA","status":"open","limit":2.5}`, "arguments.limit"},
		{"list_tickets", `{"project":"ALPHA","status":"open","order":"desc"}`, "arguments.order"},
		{"list_tickets", `{"project":"ALPHA","status":"open","limit":0}`, "arguments.limit"},
		{"search_tickets", `{"query":"timeout","created_after":"09/01/2026"}`, "arguments.created_after"},
		{"get_tickets", `{"ids":"ALPHA-3,BETA-8"}`, "arguments.ids"},
		{"get_tickets", `{"ids":[]}`, "arguments.ids"},
		{"get_ticket", `{"id":"alpha-3"}`, "arguments.id"},
		{"create_ticket", `{"project":"BETA","title":"x","priority":"critical"}`, "arguments.priority"},
		{"create_ticket", `{"project":"BETA","title":"x","fields":{"labels":["a"]}}`, "arguments.fields.priority"},
		{"create_ticket", `{"project":"BETA","title":"x","fields":{"priority":"critical","labels":"a,b"}}`, "arguments.fields.labels"},
		{"update_status", `{"id":"ALPHA-12","status":"closed"}`, "arguments.resolution"},
		{"update_status", `{"id":"ALPHA-12","status":"open","resolution":"fixed"}`, "arguments.resolution"},
		{"add_comment", `{"id":"ALPHA-12","body":"x","internal":"yes"}`, "arguments.internal"},
	}
	for _, c := range cases {
		text, isErr := call(t, c.tool, c.args)
		if !isErr || !strings.HasPrefix(text, ArgErrorMarker) || !strings.Contains(text, c.field) {
			t.Errorf("%s %s: want a refusal naming %s, got %v %s", c.tool, c.args, c.field, isErr, text)
		}
	}
}

func TestTicketsServe(t *testing.T) {
	in := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_ticket","arguments":{"id":"ALPHA-3"}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"nope","arguments":{}}}`,
	}, "\n") + "\n"
	var out bytes.Buffer
	if err := serveWith(strings.NewReader(in), &out, handleTickets); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 4 {
		t.Fatalf("want 4 responses, got %d:\n%s", len(lines), out.String())
	}
	if !strings.Contains(lines[0], "bench-mcp-tickets") {
		t.Errorf("initialize: %s", lines[0])
	}
	for _, tool := range ticketTools {
		if !strings.Contains(lines[1], `"name":"`+tool.Name+`"`) {
			t.Errorf("tools/list lacks %s", tool.Name)
		}
	}
	if !strings.Contains(lines[2], "kato") || !strings.Contains(lines[2], `"isError":false`) {
		t.Errorf("get_ticket: %s", lines[2])
	}
	if !strings.Contains(lines[3], "-32602") {
		t.Errorf("unknown tool: %s", lines[3])
	}
}
