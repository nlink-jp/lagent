package cmd

import (
	"bytes"
	"testing"
)

// resetRoot clears what a previous Execute left on the package-global
// rootCmd. cobra keeps parsed flag values between runs, so a `--version`
// from one test would short-circuit the next run into printing the
// version again — the way the "loop not implemented" test first passed
// by accident.
func resetRoot(t *testing.T, out *bytes.Buffer) {
	t.Helper()
	rootCmd.SetOut(out)
	rootCmd.SetErr(out)
	// cobra registers --version lazily on the first Execute, and only
	// when Version is set (main.go sets it; a test binary does not), so
	// give it a version and register the flag before resetting it.
	if rootCmd.Version == "" {
		rootCmd.Version = "v0.0.0-test"
	}
	rootCmd.InitDefaultVersionFlag()
	if err := rootCmd.Flags().Set("version", "false"); err != nil {
		t.Fatalf("reset --version: %v", err)
	}
}

// runRoot executes the root command with args and returns its combined
// output.
func runRoot(t *testing.T, args ...string) string {
	t.Helper()
	buf := &bytes.Buffer{}
	resetRoot(t, buf)
	rootCmd.SetArgs(args)
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("%v failed: %v", args, err)
	}
	return buf.String()
}

// `brew test` runs `lagent --version`, and the scaffold checklist
// requires the `version` subcommand to print the identical line.
func TestVersionFlagAndSubcommandAgree(t *testing.T) {
	rootCmd.Version = "v9.9.9-test"
	flag := runRoot(t, "--version")
	sub := runRoot(t, "version")
	want := "lagent version v9.9.9-test\n"
	if flag != want {
		t.Fatalf("--version printed %q, want %q", flag, want)
	}
	if sub != flag {
		t.Fatalf("`version` printed %q, --version printed %q; they must be identical", sub, flag)
	}
}

// Until Phase 1 lands the loop, running with no subcommand must fail
// loudly rather than pretend to start a session.
func TestRootWithoutLoopFails(t *testing.T) {
	resetRoot(t, &bytes.Buffer{})
	rootCmd.SetArgs([]string{})
	if err := rootCmd.Execute(); err == nil {
		t.Fatal("root command succeeded before the agent loop exists")
	}
}
