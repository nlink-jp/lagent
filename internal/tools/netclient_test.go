package tools

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/nlink-jp/lagent/internal/sandbox"
)

func TestNeedsNetworkIsAFiniteListOfPrograms(t *testing.T) {
	for _, yes := range []string{
		"curl -s https://ifconfig.me",
		"/usr/bin/curl -s https://ifconfig.me",
		"wget -q -O- https://example.com",
		"dig +short example.com",
		"ls; curl -s https://x",
		"echo x | nc -w1 example.com 80",
		"cd /tmp && curl -s https://x",
		"HTTPS_PROXY=http://p:3128 curl -s https://x",
		"env FOO=1 curl https://x",
		"git fetch origin",
		"git pull",
		"git ls-remote origin",
		"ls || wget https://x",
	} {
		if !needsNetwork(yes) {
			t.Errorf("needsNetwork(%q) = false", yes)
		}
	}
	for _, no := range []string{
		"ls -la",
		"grep -r curl .",
		"cat curl.txt",
		"git status",
		"git log --oneline",
		"echo 'curl -s https://x'",
		"./curl-wrapper.sh",
		"python3 fetch.py",
		"",
	} {
		if needsNetwork(no) {
			t.Errorf("needsNetwork(%q) = true", no)
		}
	}
}

func TestSplitCommandList(t *testing.T) {
	got := splitCommandList("a | b && c ; d || e\nf")
	want := []string{"a", "b", "c", "d", "e", "f"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("split = %v, want %v", got, want)
	}
}

// A network client that fails silently in the read lane (`curl -s`
// prints nothing and exits non-zero) still gets the lane's hint: the
// program is the evidence when the text is not. The hint is the read
// lane's alone, and an ordinary failure of a non-network program stays
// unblamed.
func TestSilentNetworkFailureInTheReadLaneIsExplained(t *testing.T) {
	if _, err := exec.LookPath("curl"); err != nil {
		t.Skip("no curl on this machine")
	}
	r, err := New(t.TempDir(), nil, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	r.SetLaneExec(func(ctx context.Context, c string, lane sandbox.Lane) *exec.Cmd {
		return exec.CommandContext(ctx, "/bin/bash", "-c", c)
	}, sandbox.Enforcement{Confined: true, ReadLane: true})
	tool, _ := r.Get(ShellExecName)
	// Port 9 on loopback: refused at once, and -s keeps curl silent.
	silent := "curl -s -m 2 http://127.0.0.1:9/"
	out, err := tool.Run(context.Background(), map[string]any{"command": silent})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "[exit status") || !strings.Contains(out, `access: "write"`) {
		t.Errorf("silent read-lane failure not explained: %q", out)
	}
	out, _ = tool.Run(context.Background(), map[string]any{"command": silent, "access": "write"})
	if strings.Contains(out, "read lane denied") {
		t.Errorf("a write-lane failure of a network client is not the read lane's: %q", out)
	}
	out, _ = tool.Run(context.Background(), map[string]any{"command": "grep -q zzz-not-there /dev/null"})
	if strings.Contains(out, "lane denied") {
		t.Errorf("a non-network failure was blamed on the lane: %q", out)
	}
}
