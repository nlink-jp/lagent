package tools

import (
	"path/filepath"
	"strings"
)

// networkClients are programs whose job is the network: run in the
// read lane they cannot do it, and several of them fail silently —
// `curl -s` prints nothing and exits 6, so the text-keyed denial hint
// never fires and the model retries the same lane (measured 2026-09-10:
// three identical curl calls, then "the sandbox blocks the network").
// A finite list, matched on the program's base name, rather than a
// pattern over the command text.
var networkClients = map[string]bool{
	"curl": true, "wget": true, "ssh": true, "scp": true, "sftp": true,
	"nc": true, "ncat": true, "telnet": true,
	"dig": true, "nslookup": true, "host": true, "ping": true, "traceroute": true,
}

// gitNetwork are the git subcommands that reach a remote.
var gitNetwork = map[string]bool{
	"fetch": true, "pull": true, "clone": true, "push": true, "ls-remote": true,
}

// needsNetwork reports whether the command line runs a program from
// networkClients — as any segment of a pipeline or command list — or a
// git subcommand from gitNetwork. Leading VAR=value assignments and
// the `env`, `time`, `command` and `nice` wrappers are skipped.
func needsNetwork(command string) bool {
	for _, seg := range splitCommandList(command) {
		fields := strings.Fields(seg)
		i := 0
		for i < len(fields) {
			f := fields[i]
			switch {
			case strings.Contains(f, "=") && !strings.HasPrefix(f, "=") && !strings.ContainsAny(f[:strings.Index(f, "=")], "/-"):
				i++ // an environment assignment
				continue
			case f == "env" || f == "time" || f == "command" || f == "nice" || f == "exec":
				i++
				continue
			}
			break
		}
		if i >= len(fields) {
			continue
		}
		prog := filepath.Base(fields[i])
		if networkClients[prog] {
			return true
		}
		if prog == "git" && i+1 < len(fields) && gitNetwork[fields[i+1]] {
			return true
		}
	}
	return false
}

// splitCommandList cuts a command line at the shell's list and pipe
// operators. Quoting is not honoured: an operator inside a string
// splits a segment whose first word is then not a program name, which
// only ever costs a false negative.
func splitCommandList(command string) []string {
	var out []string
	cur := strings.Builder{}
	flush := func() {
		if s := strings.TrimSpace(cur.String()); s != "" {
			out = append(out, s)
		}
		cur.Reset()
	}
	for i := 0; i < len(command); i++ {
		c := command[i]
		switch {
		case c == '|' || c == ';' || c == '\n':
			flush()
			if c == '|' && i+1 < len(command) && command[i+1] == '|' {
				i++
			}
		case c == '&' && i+1 < len(command) && command[i+1] == '&':
			flush()
			i++
		default:
			cur.WriteByte(c)
		}
	}
	flush()
	return out
}
