// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"io"
	"path/filepath"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/paths"
)

type Options struct {
	Names  []string
	DryRun bool
	Undo   bool
	Yes    bool
	// TUI triggers interactive terminal UI setup.
	TUI bool
	// NoTUI disables automatic interactive terminal UI setup.
	NoTUI bool
	// SkipDiscovery disables automatic model discovery during setup (useful in tests).
	SkipDiscovery bool
	// NoSessionPolicy skips the optional main-session policy (model opusplan and
	// plan as the starting mode); hooks are wired as usual.
	NoSessionPolicy bool
	// YouShouldKnow turns on Claude Code's built-in You should know mod. It is
	// opt-in because each check is a billed background request.
	YouShouldKnow bool
	// NoPlugin keeps the Claude Code hooks in settings.json instead of
	// installing the Downshift plugin (which carries the hooks).
	NoPlugin bool
	// NoMods skips the target's mods and their shell-profile cleanup (internal/setup/mods.go).
	NoMods bool
	// NoSkills leaves the target's skills out of the plan (internal/setup/skills.go).
	NoSkills bool
	// NoSteering leaves the instructions file (steering and agent triggers) out of the plan.
	NoSteering bool
	// Workers is the answer for orchestration.workers in the user policy file
	// (off, read or write). Empty writes nothing about workers.
	Workers string
	// Mcp registers `downshift mcp serve` as an MCP server (internal/setup/mcp.go). On by
	// default; --no-mcp turns it off.
	Mcp bool
	// Exec is the process boundary used to install the plugin. Its zero value
	// runs nothing.
	Exec Exec
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
	// SmallTargets is, per harness that can rewrite a subagent's model, the
	// small-tier model the hook would write today, resolved by the CLI from the
	// detected model list (never from the catalog alone, except where the
	// harness declares that fallback). An entry with an empty Model has no
	// list, so no profile is written for it and the plan says what to run.
	// Nil or empty keeps the bare policy file (BarePolicy).
	SmallTargets []SmallTarget
	// BarePolicy writes the policy file with no routing section even when
	// SmallTargets resolve (`policy init --bare`).
	BarePolicy bool
	// BinDirs are the directories searched for other downshift executables that
	// setup offers to remove. Nil skips the step; the CLI entry sets it, so
	// tests that call Run directly never touch the real PATH.
	BinDirs []string
	// BuildDate is this build's stamp; pruneBinaries compares other copies
	// against it. Empty means unstamped, so nothing is removed.
	BuildDate string
	// DiscoverModels runs model discovery for the harnesses that have a
	// source. Nil skips it; the CLI wires internal/discovery here.
	DiscoverModels func(Options)
	// EnsureSessionModels reports the targets that still lack a detected model
	// list. Nil skips it.
	EnsureSessionModels func(Options, []Target)
}

// Exec is the process boundary of setup. The zero value runs nothing, so
// a caller that does not set it (the unit tests) keeps the settings-hooks path.
type Exec struct {
	LookPath func(name string) (string, error)
	Run      func(name string, args ...string) ([]byte, error)
}

// stateDir is where setup keeps the files it generates (the wrappers).
func (o Options) StatePath() string {
	if o.StateDir != "" {
		return o.StateDir
	}
	return filepath.Join(o.Home, paths.DirName)
}
