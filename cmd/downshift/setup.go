// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/contextopt"
	"github.com/tiagovilasboas/downshift/internal/discovery"
	"github.com/tiagovilasboas/downshift/internal/paths"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// `downshift setup [harness...] [--dry-run] [--undo] [--yes]` wires every
// Downshift hook into the harness hook files listed in setup_targets.go. It
// merges (never replaces), is idempotent, shows a diff and keeps a backup.
// The per-harness facts are data in setupTargets; nothing here names a harness.

type setupOptions struct {
	Names  []string
	DryRun bool
	Undo   bool
	Yes    bool
	// TUI triggers interactive terminal UI setup.
	TUI bool
	// SkipDiscovery disables automatic model discovery during setup (useful in tests).
	SkipDiscovery bool
	// NoSessionPolicy skips the optional main-session policy (model opusplan and
	// plan as the starting mode); hooks are wired as usual.
	NoSessionPolicy bool
	// YouShouldKnow turns on Claude Code's built-in You should know mod. It is
	// opt-in because each check is a billed background request.
	YouShouldKnow bool
	// NoPlugin keeps the Claude Code hooks in settings.json instead of
	// installing the Downshift plugin (which carries the hooks and the
	// cache-tax mod).
	NoPlugin bool
	// Mcp registers `downshift mcp serve` as an MCP server (setup_mcp.go). On by
	// default; --no-mcp turns it off.
	Mcp bool
	// Exec is the process boundary used to install the plugin. Its zero value
	// runs nothing.
	Exec setupExec
	// PluginActive records, per target name, that the plugin carries the hooks
	// (installed now, or assumed in a dry run).
	PluginActive map[string]bool
	Home         string
	Binary       string
	Now          func() time.Time
	In           io.Reader
	Out          io.Writer
	Err          io.Writer

	// StateDir is where setup keeps what it generates (wrapper scripts).
	// Empty means <Home>/.downshift.
	StateDir string
	// ProjectDir is the project whose .downshift/policy.json setup offers to
	// create. Empty skips that step.
	ProjectDir string
	// BinDirs are the directories searched for other downshift executables that
	// setup offers to remove. Nil skips the step; the CLI entry sets it, so
	// tests that call setupRun directly never touch the real PATH.
	BinDirs []string
}

// runSetup is the CLI entry: it resolves HOME and the running binary.
func runSetup(args []string, in io.Reader, out, errw io.Writer) int {
	opts := defaultSetupOptions(in, out, errw)
	flags, help, bad := parseSetupArgs(args, &opts)
	if help {
		fmt.Fprint(out, setupUsage)
		return 0
	}
	if bad != "" {
		fmt.Fprintf(errw, "setup: unknown flag %q\n%s", bad, setupUsage)
		return 2
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(errw, "setup: cannot find the home directory: %v\n", err)
		return 1
	}
	opts.Home = home
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(errw, "setup: cannot find this binary: %v\n", err)
		return 1
	}
	opts.Binary = exe
	if code := guardHookBinary(opts, flags.allowUnsafe, os.TempDir()); code != 0 {
		return code
	}
	if dir, err := paths.StateDir(); err == nil {
		opts.StateDir = dir
	}
	if cwd, err := os.Getwd(); err == nil && !flags.noPolicy {
		opts.ProjectDir = cwd
	}
	if !flags.keepBinaries {
		opts.BinDirs = defaultBinDirs(home)
	}
	if !opts.DryRun && !opts.Undo {
		// Older releases created the state directory and its files with the
		// default umask; tighten what the product owns. Best effort.
		_ = paths.Harden()
	}
	if opts.TUI {
		return setupTUI(opts)
	}
	return setupRun(opts)
}

// guardHookBinary refuses to write hooks that point at a binary which will
// vanish or which another user can replace. A dry run and --undo only warn
// (they write no hook command); --allow-unsafe-binary overrides.
func guardHookBinary(o setupOptions, allow bool, tmp string) int {
	why := unsafeBinaryReason(o.Binary, tmp)
	if why == "" {
		return 0
	}
	msg := "setup: this binary (" + o.Binary + ") is not a safe hook target: " + why
	if allow || o.DryRun || o.Undo {
		fmt.Fprintln(o.Err, "warning: "+msg)
		return 0
	}
	fmt.Fprintln(o.Err, msg+".\nInstall downshift to a stable directory (e.g. `go install`, Homebrew, /usr/local/bin) and run setup from there, or pass --allow-unsafe-binary.")
	return 1
}

const setupUsage = `usage: downshift setup [harness...] [--dry-run] [--undo] [--yes] [--tui] [--no-session-policy] [--you-should-know] [--no-plugin] [--no-mcp] [--no-policy] [--keep-binaries] [--allow-unsafe-binary]

Wire every Downshift hook into the harnesses found on this machine.
  --tui                run interactive terminal UI onboarding
  --dry-run            show the diff, write nothing
  --undo               restore the latest backup of each hook file
  --yes                do not ask before writing
  --no-session-policy  wire the hooks only: do not set the Claude Code
                       main-session policy (model opusplan, sessions start in
                       plan mode)
  --you-should-know    also turn on Claude Code's built-in You should know mod
                       (opt-in: each check is a billed background request)
  --no-plugin          Claude Code: keep the hooks in settings.json instead of
                       installing the Downshift plugin (the plugin carries the
                       same hooks plus the cache-tax mod) and skip the sibling
                       mods (savvy-progress)
  --no-mcp             do not register "downshift mcp serve" as the MCP server
                       "downshift" in Claude Code (through its CLI) and Codex
                       (a marked block in config.toml). Registered by default;
                       --undo removes it. --mcp is still accepted
  --no-policy          do not offer .downshift/policy.json in the current
                       directory (see downshift policy init)
  --keep-binaries      do not offer to remove other downshift copies on PATH
                       and in the Go and per-user bin directories
  --allow-unsafe-binary
                       write hooks that point at this binary even when it sits
                       in a temporary, Go build or world-writable directory

Claude Code gets the policy by default, only for keys it does not already set.
`

func setupRun(o setupOptions) int {
	targets, code := setupSelect(o)
	if code != 0 {
		return code
	}
	if o.Undo {
		code := setupUndo(o, targets)
		if mcpCode := mcpUndo(o, targets); mcpCode != 0 {
			code = mcpCode
		}
		return code
	}

	var plans []setupPlan
	extra := map[string][]string{}
	for _, t := range targets {
		if t.File == "" {
			fmt.Fprintf(o.Out, "%s: detected, not wired by setup\n  %s\n\n", t.Name, t.Manual)
			if p, ok := planSteering(o, t); ok {
				plans = append(plans, p)
			}
			continue
		}
		if active, notes := ensurePlugin(o, t); active || len(notes) > 0 {
			if o.PluginActive == nil {
				o.PluginActive = map[string]bool{}
			}
			o.PluginActive[t.Name] = active
			extra[t.Name] = notes
		}
		extra[t.Name] = append(extra[t.Name], ensureMods(o, t)...)
		p := planTarget(o, t)
		p.notes = append(extra[t.Name], p.notes...)
		plans = append(plans, p.companions...)
		plans = append(plans, p)
		if reg, ok := planCrewRegistry(o, t); ok {
			plans = append(plans, reg)
		}
		if sp, ok := planSteering(o, t); ok {
			plans = append(plans, sp)
		}
		plans = append(plans, planSkills(o, t)...)
	}

	if p, ok := planPolicyInit(o); ok {
		plans = append(plans, p)
	}
	mcpFiles, mcpSteps := planMcp(o, targets)
	failed, proceed := applyPlans(o, plans)
	if runSetupMcp(o, mcpFiles, mcpSteps, proceed, failed) {
		failed = true
	}
	if !failed && !o.Undo {
		failed = pruneBinaries(o, plans)
	}
	if !proceed {
		if failed {
			return 1
		}
		return 0
	}
	// Ensure native context optimization is enabled by default.
	if optCfg, err := contextopt.LoadConfig(); err == nil {
		if !optCfg.Enabled {
			optCfg = contextopt.DefaultConfig()
			_ = contextopt.SaveConfig(optCfg)
		}
	}
	// Run discovery for harnesses with a source so sessions don't stay on handwritten lists.
	setupDiscoverModels(o)
	// Say which harnesses still lack a detected model list (read-only: nothing is written).
	setupEnsureSessionModels(o, targets)

	fmt.Fprint(o.Out, setupSummary(plans))
	fmt.Fprintln(o.Out, "Undo with: downshift setup --undo")
	if failed {
		return 1
	}
	return 0
}

// setupSummary says, once the files are written, what now applies to the
// developer. It reads the written Claude Code document so it never claims a
// session policy that is not in the file.
func setupSummary(plans []setupPlan) string {
	wrote := false
	var policy []string
	for _, p := range plans {
		if p.err != nil || !p.changed() {
			continue
		}
		wrote = true
		if p.target.Name != "claude-code" {
			continue
		}
		doc, err := parseJSONDoc(p.after)
		if err != nil {
			continue
		}
		if m, _ := doc.get("model"); m == "opusplan" {
			policy = append(policy, "the main session uses opusplan (Opus plans, Sonnet executes)")
		}
		if perms, ok := doc.get("permissions"); ok {
			if po, isObj := asObj(perms); isObj {
				if mode, _ := po.get("defaultMode"); mode == "plan" {
					policy = append(policy, "sessions start in plan mode: Claude plans and you approve before it edits")
				}
			}
		}
		if ep, ok := doc.get("enabledPlugins"); ok {
			if po, isObj := asObj(ep); isObj {
				if on, _ := po.get("cc-plugin-you-should-know@builtin"); on == true {
					policy = append(policy, "You should know is on: each check is a billed background request to the session's model (/plugin disable cc-plugin-you-should-know@builtin to turn off)")
				}
			}
		}
	}
	if !wrote {
		return ""
	}
	var b strings.Builder
	b.WriteString("\nWhat changes for you\n")
	b.WriteString("  Subagents: Downshift picks the model for each spawn from the task. Your main session is not touched.\n")
	if share := telemetry.ControlShare(); share > 0 {
		fmt.Fprintf(&b, "  Control group: about 1 in %d spawns runs unrouted on purpose, so `downshift report` can measure the savings. DOWNSHIFT_CONTROL_SHARE=0 turns it off.\n", int(math.Round(1/share)))
	} else {
		b.WriteString("  Control group: off (DOWNSHIFT_CONTROL_SHARE=0), so `downshift report` has nothing to compare.\n")
	}
	if len(policy) > 0 {
		fmt.Fprintf(&b, "  Claude Code: %s. Skip this with `downshift setup --no-session-policy`.\n", strings.Join(policy, "; "))
	}
	b.WriteString("  Check anything with `downshift doctor`, see the effect with `downshift stats`.\n\n")
	return b.String()
}

func diffName(shown string, exists bool, side string) string {
	if !exists {
		return "/dev/null"
	}
	return side + "/" + shown
}

func setupPrintNotes(w io.Writer, notes []string) {
	for _, n := range notes {
		fmt.Fprintf(w, "  note: %s\n", n)
	}
}

// setupDisplay shortens a path under HOME to ~/....
func setupDisplay(home, p string) string {
	if rel, err := filepath.Rel(home, p); err == nil && !strings.HasPrefix(rel, "..") {
		return "~/" + filepath.ToSlash(rel)
	}
	return p
}

func setupConfirm(o setupOptions, prompt string) bool {
	if o.Yes {
		return true
	}
	fmt.Fprint(o.Out, prompt)
	line, _ := bufio.NewReader(o.In).ReadString('\n')
	fmt.Fprintln(o.Out)
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	}
	return false
}

// setupDiscoverModels runs models discovery for harnesses with a source
// so fresh installs don't stay on handwritten or outdated lists.
func setupDiscoverModels(o setupOptions) {
	if o.SkipDiscovery || o.DryRun || o.Undo {
		return
	}
	cachePath, err := discovery.Path()
	if err != nil {
		return
	}
	cat := catalog.Load()
	sources := discovery.Defaults()
	now := o.Now().UTC()
	outcomes := discovery.Run(context.Background(), sources, 5*time.Second)
	updates := map[string]discovery.Entry{}
	for _, res := range outcomes {
		if res.Err != nil || len(res.Models) == 0 {
			continue
		}
		h := res.Source.Harness()
		ranked := discovery.Order(h, res.Models, cat)
		updates[h] = discovery.Entry{
			Source:    res.Source.Name(),
			FetchedAt: now,
			Models:    res.Models,
			Ranked:    ranked,
		}
	}
	if len(updates) > 0 {
		_ = discovery.Merge(cachePath, updates)
	}
}
