// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Command downshift is the Downshift binary. It runs as a hook: a
// harness pipes a JSON event on stdin, downshift classifies the subagent task
// and prints the steering JSON on stdout that rewrites the subagent's model.
//
// Usage as a Claude Code PreToolUse hook:
//
//	{ "type": "command", "command": "downshift claude-code" }
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	_ "github.com/tiagovilasboas/downshift/internal/compressor/learning"
	"github.com/tiagovilasboas/downshift/internal/hookport"
	_ "github.com/tiagovilasboas/downshift/internal/policyfile"
	_ "github.com/tiagovilasboas/downshift/internal/quota"
	"github.com/tiagovilasboas/downshift/internal/telemetry/fromcore"
)

// buildVersion, buildCommit and buildDate are set by release builds with
// -ldflags (see .goreleaser.yml). buildVersion is included in prompt-free
// routing telemetry to make policy incidents reproducible.
var (
	buildVersion = "dev"
	buildCommit  = ""
	buildDate    = ""
)

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		cat := catalog.Load()
		fromcore.Install(cat)
		os.Exit(runDefaultEntry(os.Stdin, os.Stdout, os.Stderr, cat))
	}
	// Help is answered before anything loads: no catalog, no stdin, no telemetry.
	if code, handled := commandHelpRequest(args, os.Stdout, os.Stderr); handled {
		os.Exit(code)
	}
	// A hook must never block a spawn (a Go panic exits 2, which every harness
	// reads as "deny"), so its catalog load runs under a recover and a failure
	// allows the spawn unchanged.
	if h, ok := harnessRegistry[args[0]]; ok && h.Hook != nil {
		cat, loaded := loadCatalogFailOpen(os.Stderr)
		if !loaded {
			os.Exit(0)
		}
		fromcore.Install(cat)
		os.Exit(h.Hook(os.Stdin, cat))
	}
	// compact wraps a shell command; it needs no catalog, so it skips the load.
	// exec is the old name, kept so commands rewritten by an older version still run;
	// compactor is the product name and runs the same wrapper.
	if args[0] == "compact" || args[0] == "compactor" || args[0] == "exec" {
		os.Exit(runExec(args[1:], os.Stdin, os.Stdout, os.Stderr))
	}

	// Load the effective catalog once at startup — embedded JSON with optional
	// user override at ~/.harness-downshift/catalog.json. Fail-open: if the
	// user file is invalid, the embedded catalog is used automatically.
	catalog := catalog.Load()
	fromcore.Install(catalog)

	switch args[0] {
	case "honor":
		if len(args) != 2 {
			fmt.Fprint(os.Stderr, "usage: downshift honor <harness>\n")
			os.Exit(2)
		}
		os.Exit(runHonor(os.Stdin, portFor(args[1]), catalog))
	case "usage":
		if len(args) != 2 {
			fmt.Fprint(os.Stderr, "usage: downshift usage <harness>\n")
			os.Exit(2)
		}
		os.Exit(runUsage(os.Stdin, portFor(args[1]), catalog))
	case "end":
		if len(args) != 2 {
			fmt.Fprint(os.Stderr, "usage: downshift end <harness>\n")
			os.Exit(2)
		}
		os.Exit(runEnd(os.Stdin, portFor(args[1]), catalog))
	case "claude-code-post-tool-use":
		os.Exit(runHonor(os.Stdin, portFor("claude-code"), catalog))
	case "claude-code-subagent-stop":
		os.Exit(runUsage(os.Stdin, portFor("claude-code"), catalog))
	case "models":
		os.Exit(runModels(catalog, args[1:]))
	case "quota":
		os.Exit(runQuota(args[1:], os.Stdin, os.Stdout, os.Stderr))
	case "stats":
		os.Exit(runStats(args[1:]))
	case "report":
		os.Exit(runReport(args[1:], os.Stdout, os.Stderr))
	case "probe":
		os.Exit(runProbe(args[1:], os.Stdin, os.Stdout, os.Stderr))
	case "cursor-touch":
		os.Exit(runCursorTouch(os.Stdin))
	case "session":
		os.Exit(runSession(args[1:], os.Stdout, os.Stderr, catalog))
	case "watch":
		os.Exit(runWatch(args[1:], os.Stdout, os.Stderr, catalog))
	case "eval-outcome":
		os.Exit(runEvalOutcome(args[1:], os.Stdout, os.Stderr))
	case "context":
		os.Exit(runContext(args[1:], os.Stdout, os.Stderr))
	case "dci":
		os.Exit(runDCI(args[1:], os.Stdin, os.Stdout, os.Stderr))
	case "learning":
		os.Exit(runLearning(args[1:], os.Stdout, os.Stderr))
	case "setup":
		os.Exit(runSetup(args[1:], os.Stdin, os.Stdout, os.Stderr))
	case "uninstall":
		os.Exit(runUninstall(args[1:], os.Stdin, os.Stdout, os.Stderr))
	case "accept":
		os.Exit(runAccept(args[1:], os.Stdout, os.Stderr))
	case "loop":
		os.Exit(runLoop(args[1:], os.Stdout, os.Stderr))
	case "serve":
		os.Exit(runServe(args[1:], os.Stdout, os.Stderr))
	case "dashboard":
		os.Exit(runDashboard(args[1:], os.Stdout, os.Stderr))
	case "doctor":
		os.Exit(runDoctor(args[1:], os.Stdout, os.Stderr, catalog))
	case "enable", "shell-init":
		// enable is the canonical name; shell-init is kept as a legacy alias.
		os.Exit(runShellInit(args[1:], os.Stdout, os.Stderr))
	case "policy":
		os.Exit(runPolicy(args[1:], os.Stdout, os.Stderr))
	case "config":
		os.Exit(runConfig(args[1:], os.Stdout, os.Stderr, catalog))
	case "version", "--version":
		os.Exit(runVersion(os.Stdout))
	case "-h", "--help", "help":
		if wantsAllHelp(args[1:]) {
			usage(os.Stdout)
		} else {
			usageShort(os.Stdout)
		}
		os.Exit(0)
	default:
		fmt.Fprintln(os.Stderr, unknownCommandLine(args[0]))
		os.Exit(2)
	}
}

// runVersion prints the binary version, plus commit and build date when the
// release build stamped them.
func runVersion(w io.Writer) int {
	line := "downshift " + buildVersion
	if buildCommit != "" || buildDate != "" {
		line += fmt.Sprintf(" (commit %s, built %s)", buildCommit, buildDate)
	}
	fmt.Fprintln(w, line)
	return 0
}

// hookPorts registers honor and usage only when that harness's own hook
// sends the field. Do not copy another harness's payload into this map.
// portFor returns the registered observation port. An unknown harness gets
// nil funcs so the stage stays unobserved instead of borrowing another
// harness's payload fields.
func portFor(id string) hookport.Port {
	if e, ok := harnessRegistry[id]; ok && e.Port != nil {
		return *e.Port
	}
	return hookport.Port{ID: id}
}
