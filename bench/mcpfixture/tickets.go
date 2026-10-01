package main

import (
	"encoding/json"
	"fmt"
	"hash/crc32"
	"math"
	"regexp"
	"sort"
	"strings"
)

// The "tickets" persona (ADR-0028 §Acceptance): a ticket tracker whose
// schemas mix required and optional fields, enums, bounded integers,
// arrays, a nested object and date strings, validated strictly. Every
// refusal starts with ArgErrorMarker, which the bench counts. Answers are
// reachable only through the right arguments: a create or a status
// change returns an id derived from the exact arguments, and a list
// returns a different set for a wrong filter, sort or limit.

// ArgErrorMarker opens every argument refusal; bench/stats.go counts it.
const ArgErrorMarker = "invalid arguments:"

const ticketsInstructions = "A ticket tracker for projects ALPHA and BETA: list, search, read, create and update tickets. " +
	"Arguments are validated strictly against each tool's schema."

type ticket struct {
	ID, Project, Title, Status, Priority, Assignee, Created string
}

// ticketTable is the canned data. The dates and priorities are chosen
// so that a wrong filter, sort or limit returns a different set.
var ticketTable = []ticket{
	{"ALPHA-3", "ALPHA", "Login page throws 500 on empty password", "open", "high", "kato", "2026-08-14"},
	{"ALPHA-7", "ALPHA", "Upgrade the TLS library", "in_progress", "medium", "mori", "2026-08-20"},
	{"ALPHA-9", "ALPHA", "Nightly export misses rows", "open", "high", "saito", "2026-09-02"},
	{"ALPHA-12", "ALPHA", "Search timeout on large repositories", "open", "high", "ueda", "2026-09-18"},
	{"ALPHA-15", "ALPHA", "Dark mode contrast", "open", "low", "ono", "2026-09-21"},
	{"ALPHA-17", "ALPHA", "Webhook retries flood the queue", "open", "high", "mori", "2026-09-25"},
	{"ALPHA-18", "ALPHA", "Audit log gaps after failover", "open", "critical", "saito", "2026-09-27"},
	{"BETA-2", "BETA", "Billing report rounding", "closed", "medium", "kato", "2026-07-30"},
	{"BETA-5", "BETA", "API timeout under load", "open", "high", "ono", "2026-08-28"},
	{"BETA-8", "BETA", "Gateway timeout on upload", "in_progress", "high", "ueda", "2026-09-05"},
	{"BETA-11", "BETA", "CSV import timeout for large files", "open", "medium", "saito", "2026-09-12"},
	{"BETA-13", "BETA", "Typo in the onboarding mail", "open", "low", "mori", "2026-09-15"},
}

var users = map[string]string{"kato": "Kato Ren", "mori": "Mori Aoi", "saito": "Saito Hana", "ueda": "Ueda Sora", "ono": "Ono Riku"}

func obj(props map[string]any, required ...string) map[string]any {
	if required == nil {
		required = []string{}
	}
	return map[string]any{"type": "object", "properties": props, "required": required, "additionalProperties": false}
}

func str(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }

func enum(desc string, vals ...string) map[string]any {
	return map[string]any{"type": "string", "enum": vals, "description": desc}
}

func pat(desc, pattern string) map[string]any {
	return map[string]any{"type": "string", "pattern": pattern, "description": desc}
}

const (
	idPattern   = `^(ALPHA|BETA)-[0-9]+$`
	datePattern = `^[0-9]{4}-[0-9]{2}-[0-9]{2}$`
)

type ticketTool struct {
	Name, Description string
	Schema            map[string]any
}

var ticketTools = []ticketTool{
	{"list_projects", "List the projects and their keys.", obj(map[string]any{})},
	{"list_tickets", "List a project's tickets filtered by status (and optionally priority and assignee), sorted by creation date.", obj(map[string]any{
		"project":  pat("project key, e.g. ALPHA", `^[A-Z]+$`),
		"status":   enum("ticket status", "open", "in_progress", "closed"),
		"priority": enum("only tickets of this priority", "low", "medium", "high", "critical"),
		"assignee": str("only tickets assigned to this username"),
		"sort":     enum("order by creation date; default created_asc", "created_asc", "created_desc"),
		"limit":    map[string]any{"type": "integer", "minimum": 1, "maximum": 50, "description": "maximum number of tickets; default 10"},
	}, "project", "status")},
	{"search_tickets", "Full-text search over ticket titles, optionally within one project and after a date.", obj(map[string]any{
		"query":         str("words to find in the title"),
		"project":       pat("project key", `^[A-Z]+$`),
		"created_after": pat("only tickets created after this date, YYYY-MM-DD", datePattern),
	}, "query")},
	{"get_ticket", "Read one ticket by id.", obj(map[string]any{"id": pat("ticket id, e.g. ALPHA-3", idPattern)}, "id")},
	{"get_tickets", "Read several tickets in one call.", obj(map[string]any{
		"ids": map[string]any{"type": "array", "minItems": 1, "items": pat("ticket id", idPattern), "description": "ticket ids"},
	}, "ids")},
	{"create_ticket", "Create a ticket; returns its id.", obj(map[string]any{
		"project": pat("project key", `^[A-Z]+$`),
		"title":   str("one-line title"),
		"body":    str("optional description"),
		"fields": obj(map[string]any{
			"priority": enum("ticket priority", "low", "medium", "high", "critical"),
			"labels":   map[string]any{"type": "array", "items": str("label"), "description": "labels to attach"},
			"due_date": pat("due date, YYYY-MM-DD", datePattern),
		}, "priority"),
	}, "project", "title", "fields")},
	{"update_status", "Change a ticket's status; closing requires a resolution. Returns the change event's id.", obj(map[string]any{
		"id":         pat("ticket id", idPattern),
		"status":     enum("new status", "open", "in_progress", "closed"),
		"resolution": enum("required when status is closed", "fixed", "wontfix", "duplicate"),
	}, "id", "status")},
	{"add_comment", "Add a comment to a ticket.", obj(map[string]any{
		"id":       pat("ticket id", idPattern),
		"body":     str("comment text"),
		"internal": map[string]any{"type": "boolean", "description": "visible to staff only"},
	}, "id", "body")},
	{"get_user", "Read a user's display name by username.", obj(map[string]any{"username": str("username, e.g. kato")}, "username")},
}

func handleTickets(req request) map[string]any {
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
			"serverInfo":      map[string]any{"name": "bench-mcp-tickets", "version": "0"},
			"instructions":    ticketsInstructions,
		}
	case "tools/list":
		var list []map[string]any
		for _, t := range ticketTools {
			list = append(list, map[string]any{"name": t.Name, "description": t.Description, "inputSchema": t.Schema})
		}
		resp["result"] = map[string]any{"tools": list}
	case "tools/call":
		var p struct {
			Name string         `json:"name"`
			Args map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			resp["result"] = toolText(ArgErrorMarker+" arguments are not a JSON object", true)
			break
		}
		text, isErr, known := callTicketTool(p.Name, p.Args)
		if !known {
			resp["error"] = map[string]any{"code": -32602, "message": "unknown tool " + p.Name}
			break
		}
		resp["result"] = toolText(text, isErr)
	case "ping":
		resp["result"] = map[string]any{}
	default:
		resp["error"] = map[string]any{"code": -32601, "message": "method not found: " + req.Method}
	}
	return resp
}

func toolText(text string, isErr bool) map[string]any {
	return map[string]any{"content": []map[string]any{{"type": "text", "text": text}}, "isError": isErr}
}

func asJSON(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}

// callTicketTool validates and answers one call; known is false for a
// name the server does not have.
func callTicketTool(name string, args map[string]any) (text string, isErr bool, known bool) {
	var tool *ticketTool
	for i := range ticketTools {
		if ticketTools[i].Name == name {
			tool = &ticketTools[i]
		}
	}
	if tool == nil {
		return "", false, false
	}
	if args == nil {
		args = map[string]any{}
	}
	if err := validate(tool.Schema, args, "arguments"); err != nil {
		return ArgErrorMarker + " " + err.Error(), true, true
	}
	switch name {
	case "list_projects":
		return asJSON([]map[string]string{{"key": "ALPHA", "name": "Alpha platform"}, {"key": "BETA", "name": "Beta services"}}), false, true
	case "list_tickets":
		var out []ticket
		for _, t := range ticketTable {
			if t.Project != args["project"] || t.Status != args["status"] {
				continue
			}
			if v, ok := args["priority"].(string); ok && t.Priority != v {
				continue
			}
			if v, ok := args["assignee"].(string); ok && t.Assignee != v {
				continue
			}
			out = append(out, t)
		}
		desc := args["sort"] == "created_desc"
		sort.SliceStable(out, func(i, j int) bool {
			if desc {
				return out[i].Created > out[j].Created
			}
			return out[i].Created < out[j].Created
		})
		limit := 10
		if v, ok := args["limit"].(float64); ok {
			limit = int(v)
		}
		if len(out) > limit {
			out = out[:limit]
		}
		return asJSON(ticketsJSON(out)), false, true
	case "search_tickets":
		q := strings.ToLower(args["query"].(string))
		var out []ticket
		for _, t := range ticketTable {
			if !strings.Contains(strings.ToLower(t.Title), q) {
				continue
			}
			if v, ok := args["project"].(string); ok && t.Project != v {
				continue
			}
			if v, ok := args["created_after"].(string); ok && t.Created <= v {
				continue
			}
			out = append(out, t)
		}
		return asJSON(ticketsJSON(out)), false, true
	case "get_ticket":
		t, ok := findTicket(args["id"].(string))
		if !ok {
			return "no ticket " + args["id"].(string), true, true
		}
		return asJSON(ticketsJSON([]ticket{t})[0]), false, true
	case "get_tickets":
		var out []ticket
		for _, raw := range args["ids"].([]any) {
			t, ok := findTicket(raw.(string))
			if !ok {
				return "no ticket " + raw.(string), true, true
			}
			out = append(out, t)
		}
		return asJSON(ticketsJSON(out)), false, true
	case "create_ticket":
		f := args["fields"].(map[string]any)
		var labels []string
		if raw, ok := f["labels"].([]any); ok {
			for _, l := range raw {
				labels = append(labels, strings.ToLower(l.(string)))
			}
		}
		sort.Strings(labels)
		due, _ := f["due_date"].(string)
		key := strings.Join([]string{args["project"].(string), args["title"].(string), f["priority"].(string), strings.Join(labels, ","), due}, "|")
		id := fmt.Sprintf("%s-%d", args["project"], 100+crc32.ChecksumIEEE([]byte(key))%900)
		return asJSON(map[string]any{"id": id, "created": true}), false, true
	case "update_status":
		if args["status"] == "closed" {
			if _, ok := args["resolution"]; !ok {
				return ArgErrorMarker + " arguments.resolution: required when status is closed", true, true
			}
		} else if _, ok := args["resolution"]; ok {
			return ArgErrorMarker + " arguments.resolution: allowed only when status is closed", true, true
		}
		if _, ok := findTicket(args["id"].(string)); !ok {
			return "no ticket " + args["id"].(string), true, true
		}
		res, _ := args["resolution"].(string)
		key := strings.Join([]string{args["id"].(string), args["status"].(string), res}, "|")
		event := fmt.Sprintf("EV-%04d", crc32.ChecksumIEEE([]byte(key))%10000)
		return asJSON(map[string]any{"id": args["id"], "status": args["status"], "resolution": res, "event": event}), false, true
	case "add_comment":
		if _, ok := findTicket(args["id"].(string)); !ok {
			return "no ticket " + args["id"].(string), true, true
		}
		return asJSON(map[string]any{"id": args["id"], "comment": "added"}), false, true
	case "get_user":
		name, ok := users[args["username"].(string)]
		if !ok {
			return "no user " + args["username"].(string), true, true
		}
		return asJSON(map[string]any{"username": args["username"], "display_name": name}), false, true
	}
	return "", false, false
}

func findTicket(id string) (ticket, bool) {
	for _, t := range ticketTable {
		if t.ID == id {
			return t, true
		}
	}
	return ticket{}, false
}

func ticketsJSON(ts []ticket) []map[string]string {
	out := []map[string]string{}
	for _, t := range ts {
		out = append(out, map[string]string{"id": t.ID, "project": t.Project, "title": t.Title,
			"status": t.Status, "priority": t.Priority, "assignee": t.Assignee, "created": t.Created})
	}
	return out
}

// validate checks v against the subset of JSON Schema the tools use:
// object (properties, required, additionalProperties false), string
// (enum, pattern), integer (minimum, maximum), boolean, array (items,
// minItems). Strict on purpose: the bench measures argument discipline.
func validate(schema map[string]any, v any, path string) error {
	switch schema["type"] {
	case "object":
		m, ok := v.(map[string]any)
		if !ok {
			return fmt.Errorf("%s: want an object", path)
		}
		props, _ := schema["properties"].(map[string]any)
		var keys []string
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sub, ok := props[k].(map[string]any)
			if !ok {
				return fmt.Errorf("%s.%s: unknown property", path, k)
			}
			if err := validate(sub, m[k], path+"."+k); err != nil {
				return err
			}
		}
		req, _ := schema["required"].([]string)
		for _, r := range req {
			if _, ok := m[r]; !ok {
				return fmt.Errorf("%s.%s: required", path, r)
			}
		}
	case "string":
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("%s: want a string", path)
		}
		if vals, ok := schema["enum"].([]string); ok {
			found := false
			for _, e := range vals {
				found = found || e == s
			}
			if !found {
				return fmt.Errorf("%s: %q is not one of %s", path, s, strings.Join(vals, ", "))
			}
		}
		if p, ok := schema["pattern"].(string); ok && !regexp.MustCompile(p).MatchString(s) {
			return fmt.Errorf("%s: %q does not match %s", path, s, p)
		}
	case "integer":
		f, ok := v.(float64)
		if !ok || f != math.Trunc(f) {
			return fmt.Errorf("%s: want an integer", path)
		}
		if lo, ok := schema["minimum"].(int); ok && f < float64(lo) {
			return fmt.Errorf("%s: below %d", path, lo)
		}
		if hi, ok := schema["maximum"].(int); ok && f > float64(hi) {
			return fmt.Errorf("%s: above %d", path, hi)
		}
	case "boolean":
		if _, ok := v.(bool); !ok {
			return fmt.Errorf("%s: want a boolean", path)
		}
	case "array":
		a, ok := v.([]any)
		if !ok {
			return fmt.Errorf("%s: want an array", path)
		}
		if n, ok := schema["minItems"].(int); ok && len(a) < n {
			return fmt.Errorf("%s: at least %d items", path, n)
		}
		if items, ok := schema["items"].(map[string]any); ok {
			for i, e := range a {
				if err := validate(items, e, fmt.Sprintf("%s[%d]", path, i)); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
