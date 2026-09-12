// Command bench is the lagent task bench (ADR-0006): it runs a fixed
// task set against one or more configurations of a runtime binary in
// isolated homes, reads each run's transcript, and reports what a
// Phase 2 ADR needs as evidence — completion, rounds, tool calls,
// tokens, wall time.
//
//	go run ./bench run --bin dist/lagent --configs baseline [--reps 3]
//	go run ./bench report bench/_results/<dir>
//
// Nothing here ships in the lagent binary. Runs cost real minutes on
// the local model server, so the bench is run by hand, never by
// `make check`.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errOut, usage)
		return 2
	}
	var err error
	switch args[0] {
	case "run":
		err = cmdRun(args[1:], out)
	case "report":
		err = cmdReport(args[1:], out)
	case "-h", "--help", "help":
		fmt.Fprintln(out, usage)
		return 0
	default:
		err = fmt.Errorf("unknown command %q\n%s", args[0], usage)
	}
	if err != nil {
		fmt.Fprintln(errOut, "bench:", err)
		return 1
	}
	return 0
}

const usage = `usage:
  bench run    --bin <path> --configs <name[=file],...> [--runtime lagent|gem-agent]
               [--tasks a,b] [--reps N] [--timeout 10m] [--out DIR] [--mcp-bin <path>]
  bench report <results dir>

A configuration name resolves to bench/configs/<runtime>/<name>.toml;
name=/path/to/config.toml uses a file of your own (the reference
runtime's credentials never live in this repository).`

// runtimeProfile is what differs between the runtimes the bench can
// drive. The table is finite and each entry is verified against the
// binary before it is added (ADR-0006 §6); the CLI flags themselves
// (-p, --auto, --config) are the same on both.
type runtimeProfile struct {
	Name      string
	StateEnv  string // the state-root override the runtime honours
	ConfigDir string // under HOME: config.toml and the global mcp.json
	// PassThrough names files under the operator's real HOME that the
	// runtime needs even in an isolated one, as environment variables
	// pointing at them: the reference runtime authenticates through
	// Application Default Credentials, which live under ~/.config/gcloud
	// and would otherwise vanish with the HOME swap. Only set when the
	// file exists.
	PassThrough map[string]string
}

var runtimes = map[string]runtimeProfile{
	"lagent": {Name: "lagent", StateEnv: "LAGENT_STATE_DIR", ConfigDir: ".config/lagent"},
	"gem-agent": {Name: "gem-agent", StateEnv: "GEMAGENT_STATE_DIR", ConfigDir: ".config/gem-agent",
		PassThrough: map[string]string{
			"GOOGLE_APPLICATION_CREDENTIALS": ".config/gcloud/application_default_credentials.json",
		}},
}

// passThroughEnv resolves a profile's PassThrough files against the
// operator's real HOME; a file that does not exist sets nothing.
func (p runtimeProfile) passThroughEnv(realHome string) map[string]string {
	env := map[string]string{}
	for name, rel := range p.PassThrough {
		path := filepath.Join(realHome, rel)
		if _, err := os.Stat(path); err == nil {
			env[name] = path
		}
	}
	return env
}

// configRef names one configuration to measure: a label and the file
// that becomes the run's config.toml.
type configRef struct {
	Name string
	Path string
}

// parseConfigs turns "baseline,mine=/tmp/x.toml" into refs; a bare
// name resolves under bench/configs/<runtime>/.
func parseConfigs(spec, benchDir, runtime string) ([]configRef, error) {
	if strings.TrimSpace(spec) == "" {
		return nil, errors.New("--configs is required")
	}
	var refs []configRef
	for _, item := range strings.Split(spec, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		name, path, hasPath := strings.Cut(item, "=")
		if !hasPath {
			path = filepath.Join(benchDir, "configs", runtime, name+".toml")
		}
		if _, err := os.Stat(path); err != nil {
			return nil, fmt.Errorf("configuration %q: %w", name, err)
		}
		refs = append(refs, configRef{Name: name, Path: path})
	}
	if len(refs) == 0 {
		return nil, errors.New("--configs names nothing")
	}
	return refs, nil
}

// cell is one scheduled run: task × configuration × repetition.
type cell struct {
	Task   *task
	Config configRef
	Rep    int
}

// plan orders the runs cases-outside, configurations-inside (ADR-0006
// §4): for each task and repetition every configuration runs back to
// back, so a swing in the server's state hits all of them alike.
func plan(tasks []*task, configs []configRef, reps int) []cell {
	var cells []cell
	for _, t := range tasks {
		for rep := 1; rep <= reps; rep++ {
			for _, c := range configs {
				cells = append(cells, cell{Task: t, Config: c, Rep: rep})
			}
		}
	}
	return cells
}

func cmdRun(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	bin := fs.String("bin", "", "runtime binary to measure")
	rtName := fs.String("runtime", "lagent", "runtime profile: lagent or gem-agent")
	configs := fs.String("configs", "", "configurations, comma-separated (name or name=file)")
	taskSel := fs.String("tasks", "", "task names, comma-separated (default: all under bench/tasks)")
	reps := fs.Int("reps", 3, "repetitions per task and configuration")
	timeout := fs.Duration("timeout", 10*time.Minute, "deadline per run")
	outDir := fs.String("out", "", "results directory (default bench/_results/<timestamp>)")
	mcpBin := fs.String("mcp-bin", "", "the bench MCP fixture server (default dist/bench-mcp)")
	benchDir := fs.String("bench-dir", "bench", "the bench directory (tasks/, configs/)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *bin == "" {
		return errors.New("--bin is required")
	}
	rt, ok := runtimes[*rtName]
	if !ok {
		return fmt.Errorf("unknown runtime %q", *rtName)
	}
	refs, err := parseConfigs(*configs, *benchDir, rt.Name)
	if err != nil {
		return err
	}
	tasks, err := loadTasks(filepath.Join(*benchDir, "tasks"), splitList(*taskSel))
	if err != nil {
		return err
	}
	if *mcpBin == "" {
		*mcpBin = filepath.Join("dist", "bench-mcp")
	}
	for _, t := range tasks {
		if t.MCP {
			if _, err := os.Stat(*mcpBin); err != nil {
				return fmt.Errorf("task %s needs the MCP fixture server: %w (make bench-build)", t.Name, err)
			}
			break
		}
	}
	if *outDir == "" {
		*outDir = filepath.Join(*benchDir, "_results", time.Now().Format("20060102-150405"))
	}
	absBin, err := filepath.Abs(*bin)
	if err != nil {
		return err
	}
	absMCP, err := filepath.Abs(*mcpBin)
	if err != nil {
		return err
	}
	r := &runner{
		Runtime: rt, Bin: absBin, MCPBin: absMCP,
		OutDir: *outDir, Timeout: *timeout, Progress: out,
	}
	cells := plan(tasks, refs, *reps)
	fmt.Fprintf(out, "bench: %d runs (%d tasks × %d configs × %d reps) → %s\n",
		len(cells), len(tasks), len(refs), *reps, *outDir)
	for i, c := range cells {
		res, err := r.runOne(c)
		if err != nil {
			return fmt.Errorf("run %d/%d %s/%s#%d: %w", i+1, len(cells), c.Task.Name, c.Config.Name, c.Rep, err)
		}
		fmt.Fprintf(out, "%s [%d/%d] %s\n", time.Now().Format("15:04:05"), i+1, len(cells), res.line())
	}
	fmt.Fprintf(out, "bench: done — `go run ./bench report %s`\n", *outDir)
	return nil
}

func cmdReport(args []string, out io.Writer) error {
	if len(args) != 1 {
		return errors.New("report takes one results directory")
	}
	results, err := loadResults(args[0])
	if err != nil {
		return err
	}
	if len(results) == 0 {
		return fmt.Errorf("%s: no runs recorded", args[0])
	}
	_, err = io.WriteString(out, report(results))
	return err
}

func splitList(s string) []string {
	var out []string
	for _, item := range strings.Split(s, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}
