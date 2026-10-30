// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/contextopt"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/gateway"
	"github.com/tiagovilasboas/downshift/internal/paths"
	"github.com/tiagovilasboas/downshift/internal/semantic"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// doctorEnv is everything doctor reads from the machine, so tests can supply a
// temp HOME and fake binaries.
type doctorEnv struct {
	Home       string
	Executable func() (string, error)
	LookPath   func(string) (string, error)
	// BuildInfo reads the running binary's build stamp; nil reports unknown.
	BuildInfo func() (*debug.BuildInfo, bool)
	// Companions lists the external companions to probe (the registry's by
	// default); tests inject fakes. Nil skips the companion row.
	Companions func() []contextopt.Provider
	// Gateways lists local router gateways to probe. Nil skips the gateway check.
	Gateways func() []gateway.Gateway
	Now      func() time.Time
	Getenv   func(string) string
	Cat      *catalog.Catalog
	Verbose  bool
}

func defaultDoctorEnv(cat *catalog.Catalog) doctorEnv {
	home, _ := os.UserHomeDir()
	return doctorEnv{Home: home, Executable: os.Executable, LookPath: exec.LookPath,
		BuildInfo:  debug.ReadBuildInfo,
		Companions: func() []contextopt.Provider { return companions(contextRegistry()) },
		Gateways:   defaultDoctorGateways,
		Now:        time.Now, Getenv: os.Getenv, Cat: cat}
}

// runDoctor is `downshift doctor`: a checklist, one line per check, with the
// exact fix under every failure or warning. Exit 0 when nothing failed.
func runDoctor(args []string, w, errW io.Writer, cat *catalog.Catalog) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(errW)
	asJSON := fs.Bool("json", false, "machine-readable checklist")
	verbose := fs.Bool("verbose", false, "list every model without a verified efforts list, and more detail")
	quiet := fs.Bool("quiet", false, "print only failed and warning checks")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintln(errW, "usage: downshift doctor [--verbose] [--quiet] [--json]")
		return 2
	}
	env := defaultDoctorEnv(cat)
	env.Verbose = *verbose
	rep := buildDoctorReport(env)
	if *asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(rep)
	} else {
		rep.home, rep.width = env.Home, terminalWidth(w)
		rep.writeText(w, env.Verbose, *quiet)
	}
	if rep.Failed > 0 {
		return 1
	}
	return 0
}

const (
	doctorOK   = "ok"
	doctorWarn = "warn"
	doctorFail = "fail"
)

type doctorCheck struct {
	ID      string `json:"id"`
	Harness string `json:"harness,omitempty"`
	Status  string `json:"status"` // ok | warn | fail
	Detail  string `json:"detail"`
	Fix     string `json:"fix,omitempty"` // exact command or edit, set for warn and fail
}

type doctorReport struct {
	Version             string        `json:"version"`
	Commit              string        `json:"commit,omitempty"`
	OK                  bool          `json:"ok"`
	Failed              int           `json:"failed"`
	Warnings            int           `json:"warnings"`
	Checks              []doctorCheck `json:"checks"`
	NotDetected         []string      `json:"not_detected,omitempty"`
	EffortUnknownCount  int           `json:"effort_unknown_count"`
	EffortUnknownModels []string      `json:"effort_unknown_models,omitempty"` // --verbose only

	// home and width shape the text output only (doctor_text.go): paths under
	// home print as ~, and no line is wider than width. JSON never sees them.
	home  string
	width int
}

func (r *doctorReport) add(c doctorCheck) {
	switch c.Status {
	case doctorFail:
		r.Failed++
	case doctorWarn:
		r.Warnings++
	}
	r.Checks = append(r.Checks, c)
}

// anyHarnessChecked reports whether at least one harness had a hook file.
func (r *doctorReport) anyHarnessChecked() bool {
	for _, c := range r.Checks {
		if c.Harness != "" {
			return true
		}
	}
	return false
}

// pluginInstallHint names the plugin alternative for every target that
// declares one (data on setupTargets; doctor names no harness itself).
func pluginInstallHint() string {
	var parts []string
	for _, t := range setupTargets {
		if t.Plugin != "" {
			parts = append(parts, fmt.Sprintf("%s: /plugin install %s --marketplace tiagovilasboas/downshift", t.Name, strings.SplitN(t.Plugin, "@", 2)[0]))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return " (or the plugin, " + strings.Join(parts, "; ") + ")"
}

func buildDoctorReport(env doctorEnv) *doctorReport {
	rep := &doctorReport{Version: buildVersion, Commit: buildCommit}
	ver := "downshift " + buildVersion
	if buildCommit != "" {
		ver += " (commit " + buildCommit + ")"
	}
	rep.add(doctorCheck{ID: "version", Status: doctorOK, Detail: ver})
	rep.add(buildProvenanceCheck(env))

	pathBin := checkBinary(rep, env)
	events := checkState(rep, env)
	checkCatalog(rep, env)
	checkPrices(rep, env)
	checkContext(rep, env)
	checkGateways(rep, env)
	for _, note := range modelOverrideNotes(core.AllHarnessCaps(), env.Getenv) {
		name := strings.TrimPrefix(note, "model_override_env: ")
		if i := strings.Index(name, "="); i > 0 {
			name = name[:i]
		}
		rep.add(doctorCheck{ID: "model override", Status: doctorWarn, Detail: note, Fix: "unset " + name})
	}

	for _, nc := range core.AllHarnessCaps() {
		wiring, ok := doctorWiringFor(nc.Harness)
		if !ok || !fileExists(filepath.Join(env.Home, wiring.File)) {
			rep.NotDetected = append(rep.NotDetected, nc.Harness)
			continue
		}
		checkHarness(rep, env, nc, wiring, pathBin, events)
	}

	if env.Cat != nil {
		unknown := env.Cat.EffortUnknownModels()
		rep.EffortUnknownCount = len(unknown)
		if len(unknown) > 0 {
			detail := fmt.Sprintf("%d models without a verified efforts list (effort not constrained)", len(unknown))
			if !env.Verbose {
				detail += "; --verbose lists them"
			}
			rep.add(doctorCheck{ID: "effort_unknown", Status: doctorOK, Detail: detail})
			if env.Verbose {
				rep.EffortUnknownModels = unknown
			}
		}
	}
	rep.OK = rep.Failed == 0
	return rep
}

// buildProvenanceCheck reports the VCS stamp of the running binary. A binary
// built from a dirty tree warns; a clean build shows its short commit; a
// binary without VCS data (release builds) is unknown and never warns. Doctor
// cannot tell whether a commit is on main, so it reports only what the stamp says.
func buildProvenanceCheck(env doctorEnv) doctorCheck {
	const id = "build provenance"
	var info *debug.BuildInfo
	if env.BuildInfo != nil {
		info, _ = env.BuildInfo()
	}
	var rev, modified string
	if info != nil {
		for _, s := range info.Settings {
			switch s.Key {
			case "vcs.revision":
				rev = s.Value
			case "vcs.modified":
				modified = s.Value
			}
		}
	}
	switch {
	case modified == "true":
		return doctorCheck{ID: id, Status: doctorWarn, Detail: "built from a dirty tree",
			Fix: "rebuild from a clean checkout (git status must be empty), then reinstall the binary"}
	case rev != "":
		short := rev
		if len(short) > 7 {
			short = short[:7]
		}
		return doctorCheck{ID: id, Status: doctorOK, Detail: short}
	default:
		return doctorCheck{ID: id, Status: doctorOK, Detail: "unknown"}
	}
}

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }

func dirExists(p string) bool { fi, err := os.Stat(p); return err == nil && fi.IsDir() }

func resolvedPath(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return filepath.Clean(p)
}

// checkBinary reports whether downshift is on PATH and whether it is the
// running binary. It returns the PATH location ("" when absent).
func checkBinary(rep *doctorReport, env doctorEnv) string {
	exe, _ := env.Executable()
	pathBin, err := env.LookPath("downshift")
	if err != nil {
		fix := "install downshift, or link this binary: ln -sf " + exe + " " + filepath.Join(env.Home, ".local", "bin", "downshift")
		rep.add(doctorCheck{ID: "binary on PATH", Status: doctorFail, Detail: "downshift is not on PATH", Fix: fix})
		return ""
	}
	if exe != "" && resolvedPath(exe) != resolvedPath(pathBin) {
		rep.add(doctorCheck{ID: "binary on PATH", Status: doctorWarn,
			Detail: fmt.Sprintf("PATH has %s; this run is %s", pathBin, exe),
			Fix:    fmt.Sprintf("install -m 0755 %s %s", exe, pathBin)})
		return pathBin
	}
	rep.add(doctorCheck{ID: "binary on PATH", Status: doctorOK, Detail: pathBin})
	return pathBin
}

// checkState reports the state directory and a legacy directory that still
// exists beside it. It returns the events of the effective state directory.
func checkState(rep *doctorReport, env doctorEnv) []telemetry.Event {
	dir, src, err := paths.Resolve()
	if err != nil {
		rep.add(doctorCheck{ID: "state dir", Status: doctorFail, Detail: err.Error(),
			Fix: "set " + paths.EnvStateDir + " to a writable absolute path"})
		return nil
	}
	rep.add(doctorCheck{ID: "state dir", Status: doctorOK, Detail: fmt.Sprintf("%s (%s)", dir, src)})
	events, _ := telemetry.ReadEventsTail(telemetry.DefaultTailBytes)

	if src == paths.SourceEnv {
		return events
	}
	newDir := filepath.Join(env.Home, paths.DirName)
	legacyDir := filepath.Join(env.Home, paths.LegacyName)
	switch {
	case src == paths.SourceLegacy:
		rep.add(doctorCheck{ID: "legacy dir", Status: doctorWarn,
			Detail: legacyDir + " is in use; the current location is " + newDir,
			Fix:    fmt.Sprintf("cp -R %s %s", legacyDir, newDir)})
	case dirExists(newDir) && dirExists(legacyDir):
		legacyEvents, _ := telemetry.ReadEventsTailFrom(filepath.Join(legacyDir, "events.jsonl"), telemetry.DefaultTailBytes)
		if eventsDiverge(events, legacyEvents) {
			rep.add(doctorCheck{ID: "legacy dir", Status: doctorWarn,
				Detail: fmt.Sprintf("%s also exists with different events; only %s is read", legacyDir, newDir),
				Fix:    fmt.Sprintf("mv %s %s.bak", legacyDir, legacyDir)})
		} else {
			rep.add(doctorCheck{ID: "legacy dir", Status: doctorOK, Detail: legacyDir + " exists, same events, ignored"})
		}
	}
	return events
}

func eventsDiverge(a, b []telemetry.Event) bool {
	if len(a) == 0 && len(b) == 0 {
		return false
	}
	if len(a) != len(b) {
		return true
	}
	return a[len(a)-1].CorrelationID != b[len(b)-1].CorrelationID
}

func checkCatalog(rep *doctorReport, env doctorEnv) {
	catalogPath, _ := paths.CatalogPath()
	if fileExists(catalogPath) {
		rep.add(doctorCheck{ID: "catalog", Status: doctorOK, Detail: "user override at " + catalogPath})
		return
	}
	rep.add(doctorCheck{ID: "catalog", Status: doctorOK, Detail: "embedded default (override path " + catalogPath + ")"})
}

// pricesMaxAgeDays is how old the embedded list prices may get before doctor
// warns that the price table is old. Prices only order models inside a tier
// for routing; savings are counted in tokens, never from prices.
const pricesMaxAgeDays = 30

// pricesRoleNote says what an old price table affects: only the price order
// used to pick between models of the same tier. Savings are tokens.
const pricesRoleNote = "it only affects the same-tier price order used in routing, not token savings"

// checkPrices warns when the embedded list prices were verified long ago (or
// carry no date). Prices only age between downshift releases; the weekly
// sync-prices workflow keeps the newest release current.
func checkPrices(rep *doctorReport, env doctorEnv) {
	now := time.Now
	if env.Now != nil {
		now = env.Now
	}
	days, ok := env.Cat.PricesAge(now())
	asOf := env.Cat.PricesAsOf()
	switch {
	case !ok:
		rep.add(doctorCheck{ID: "prices", Status: doctorWarn,
			Detail: "catalog has no valid prices_as_of date; " + pricesRoleNote,
			Fix:    "upgrade downshift to the latest release to refresh the price table (docs/reference/prices.md)"})
	case days > pricesMaxAgeDays:
		rep.add(doctorCheck{ID: "prices", Status: doctorWarn,
			Detail: fmt.Sprintf("price table is from %s (%d days old, limit %d); %s", asOf, days, pricesMaxAgeDays, pricesRoleNote),
			Fix:    "upgrade downshift to the latest release, or refresh the price table with 'downshift models pull' (docs/reference/prices.md)"})
	default:
		rep.add(doctorCheck{ID: "prices", Status: doctorOK, Detail: "list prices as of " + asOf})
	}
}

func checkContext(rep *doctorReport, env doctorEnv) {
	optCfg, _ := contextopt.LoadConfig()
	if optCfg.Enabled {
		detail := "enabled (provider: " + optCfg.Provider + ")"
		if optCfg.Provider == "native" {
			detail += "; on Claude Code it only measures bytes, a hook cannot rewrite tool output"
		}
		rep.add(doctorCheck{ID: "context optim.", Status: doctorOK, Detail: detail})
	} else {
		rep.add(doctorCheck{ID: "context optim.", Status: doctorOK, Detail: "disabled (enable with `downshift context enable`)"})
	}
	checkCompanion(rep, env, optCfg)
	if env.Verbose {
		if semantic.Enabled() {
			rep.add(doctorCheck{ID: "semantic boost", Status: doctorOK, Detail: "in-process FNV hash embedder (on by default)"})
		} else {
			rep.add(doctorCheck{ID: "semantic boost", Status: doctorOK, Detail: "disabled (DOWNSHIFT_SEMANTIC_BOOST=0)"})
		}
	}
}

// checkCompanion reports every registered companion: recorded and present is
// ok; recorded but missing, or present but not recorded, is a warn with the
// exact fix. Absent and not recorded prints only in verbose: it is optional.
// The providers come from env.Companions, so doctor names none of them.
func checkCompanion(rep *doctorReport, env doctorEnv, optCfg contextopt.Config) {
	if env.Companions == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for _, c := range env.Companions() {
		id := c.ID()
		installed, ver, _, err := c.Detect(ctx)
		recorded := optCfg.Companion == id
		label := id
		if ver != "" {
			label += " v" + ver
		}
		switch {
		case err != nil:
			rep.add(doctorCheck{ID: "companion", Status: doctorWarn, Detail: err.Error(), Fix: "run `" + id + " --version` by hand, then `downshift context doctor`"})
		case recorded && installed:
			rep.add(doctorCheck{ID: "companion", Status: doctorOK, Detail: label + " recorded; wrap each harness with its own command (Downshift cannot see whether a session is wrapped)"})
		case recorded:
			rep.add(doctorCheck{ID: "companion", Status: doctorWarn, Detail: id + " recorded but not on PATH", Fix: "downshift context disable " + id})
		case installed:
			rep.add(doctorCheck{ID: "companion", Status: doctorWarn, Detail: label + " detected, not recorded (savings would be attributed to routing alone)", Fix: "downshift context enable " + id})
		case env.Verbose:
			rep.add(doctorCheck{ID: "companion", Status: doctorOK, Detail: "none (optional: " + id + ", docs/guides/companions.md)"})
		}
	}
}

// modelOverrideNotes reports every declared model-override variable that is
// set. A harness-level model setting can take precedence over a hook's model
// rewrite, which would make a Downshift rewrite a silent no-op. The precedence
// comes from outside sources and is not verified here, so the wording stays
// conditional. Nothing about routing changes. Adding a harness's variable is a
// one-line capability change; doctor needs no edit.
func modelOverrideNotes(caps []core.NamedCaps, getenv func(string) string) []string {
	var notes []string
	for _, c := range caps {
		name := c.Caps.ModelOverrideEnv
		if name == "" {
			continue
		}
		v := strings.TrimSpace(getenv(name))
		if v == "" {
			continue
		}
		if len(v) > 64 {
			v = v[:64] + "..."
		}
		notes = append(notes, fmt.Sprintf("model_override_env: %s=%s; %s may apply it instead of a Downshift rewrite (not verified here). Check the resolved model with the post-tool-use hook.", name, v, c.Harness))
	}
	return notes
}
