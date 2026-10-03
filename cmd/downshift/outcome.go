package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/tiagovilasboas/harness-downshift/internal/outcome"
)

var reportDatasets = []outcome.Dataset{
	{Path: "benchmark/tasks.json", Name: "`benchmark/tasks.json` (seed)"},
	{Path: "benchmark/holdout.json", Name: "`benchmark/holdout.json` (templated regression set)"},
}

// buildReport renders the generated README block from paths relative to root.
func buildReport(root, tasksDir, runsDir string) (string, error) {
	tasks, err := outcome.Load(filepath.Join(root, tasksDir))
	if err != nil {
		return "", err
	}
	runs, err := outcome.LoadRuns(filepath.Join(root, runsDir))
	if err != nil {
		return "", err
	}
	ds := make([]outcome.Dataset, len(reportDatasets))
	for i, d := range reportDatasets {
		ds[i] = outcome.Dataset{Path: filepath.Join(root, d.Path), Name: d.Name}
	}
	return outcome.Report(ds, tasks, runs)
}

func runEvalOutcome(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("eval-outcome", flag.ContinueOnError)
	fs.SetOutput(stderr)
	tasksDir := fs.String("tasks", "benchmark/outcomes/testdata", "task directory")
	runsDir := fs.String("runs", "benchmark/outcomes/runs", "recorded runs directory")
	verify := fs.Bool("verify", false, "offline: every stub fails, every reference passes, recorded results reproduce")
	record := fs.Bool("record", false, "solve every task with --solver (makes model calls; refused when CI is set)")
	tier := fs.String("tier", "", "tier label for --record: small, mid or frontier")
	model := fs.String("model", "", "model id recorded with --record")
	solver := fs.String("solver", "", "shell command for --record: prompt on stdin, answer on stdout")
	report := fs.Bool("report", false, "print the generated README block")
	write := fs.String("write", "", "with --report: rewrite the marked block in this file")
	check := fs.String("check", "", "with --report: exit 1 if the marked block in this file is stale")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	fail := func(err error) int { fmt.Fprintln(stderr, err); return 1 }
	ctx := context.Background()
	switch {
	case *verify:
		tasks, err := outcome.Load(*tasksDir)
		if err != nil {
			return fail(err)
		}
		problems, err := outcome.Verify(ctx, tasks, *runsDir)
		if err != nil {
			return fail(err)
		}
		for _, p := range problems {
			fmt.Fprintln(stderr, p)
		}
		if len(problems) > 0 {
			return 1
		}
		fmt.Fprintf(stdout, "eval-outcome: %d tasks verified (stub fails, reference passes, recorded results reproduce)\n", len(tasks))
		return 0
	case *record:
		if os.Getenv("CI") != "" {
			fmt.Fprintln(stderr, "eval-outcome --record makes paid model calls and is disabled when CI is set")
			return 2
		}
		if *solver == "" || *model == "" || (*tier != "small" && *tier != "mid" && *tier != "frontier") {
			fmt.Fprintln(stderr, "usage: downshift eval-outcome --record --tier=small|mid|frontier --model=<id> --solver='<cmd>'")
			return 2
		}
		tasks, err := outcome.Load(*tasksDir)
		if err == nil {
			_, err = outcome.Record(ctx, tasks, *tier, *model, *solver, *runsDir, func(f string, a ...any) { fmt.Fprintf(stderr, f, a...) })
		}
		if err != nil {
			return fail(err)
		}
		return 0
	case *report:
		block, err := buildReport("", *tasksDir, *runsDir)
		if err != nil {
			return fail(err)
		}
		target := *write + *check
		if target == "" {
			fmt.Fprint(stdout, block)
			return 0
		}
		doc, err := os.ReadFile(target)
		if err != nil {
			return fail(err)
		}
		updated, err := outcome.Splice(string(doc), block)
		if err != nil {
			return fail(err)
		}
		if *check != "" && updated != string(doc) {
			return fail(fmt.Errorf("%s: generated report block is stale; run `downshift eval-outcome --report --write %s`", target, target))
		}
		if *write != "" {
			if err := os.WriteFile(target, []byte(updated), 0o644); err != nil {
				return fail(err)
			}
		}
		return 0
	}
	fmt.Fprintln(stderr, "usage: downshift eval-outcome --verify | --record ... | --report [--write FILE | --check FILE]")
	return 2
}
