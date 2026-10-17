// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/contextopt"
	"github.com/tiagovilasboas/downshift/internal/contextopt/providers/headroom"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/paths"
	"github.com/tiagovilasboas/downshift/internal/semantic/e5shadow"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// doctorEnv is everything doctor reads from the machine, so tests can supply a
// temp HOME and fake binaries.
type doctorEnv struct {
	Home       string
	Executable func() (string, error)
	LookPath   func(string) (string, error)
	// Run executes a detected companion binary (version probe). Nil skips the probe.
	Run     func(ctx context.Context, name string, args ...string) ([]byte, error)
	Now     func() time.Time
	Getenv  func(string) string
	Cat     *catalog.Catalog
	Verbose bool
}

func defaultDoctorEnv(cat *catalog.Catalog) doctorEnv {
	home, _ := os.UserHomeDir()
	return doctorEnv{Home: home, Executable: os.Executable, LookPath: exec.LookPath,
		Run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return exec.CommandContext(ctx, name, args...).Output()
		},
		Now: time.Now, Getenv: os.Getenv, Cat: cat}
}

// runDoctor is `downshift doctor`: a checklist, one line per check, with the
// exact fix under every failure or warning. Exit 0 when nothing failed.
func runDoctor(args []string, w, errW io.Writer, cat *catalog.Catalog) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(errW)
	asJSON := fs.Bool("json", false, "machine-readable checklist")
	verbose := fs.Bool("verbose", false, "list every model without a verified efforts list, and more detail")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintln(errW, "usage: downshift doctor [--verbose] [--json]")
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
		rep.write(w, env.Verbose)
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

func (r *doctorReport) write(w io.Writer, verbose bool) {
	fmt.Fprintf(w, "downshift doctor\n")
	section := "\x00"
	for _, c := range r.Checks {
		if c.Harness != section {
			section = c.Harness
			if section != "" {
				fmt.Fprintf(w, "\n%s\n", section)
			} else {
				fmt.Fprintln(w)
			}
		}
		mark := map[string]string{doctorOK: "✓", doctorWarn: "!", doctorFail: "✗"}[c.Status]
		fmt.Fprintf(w, "  %s %-15s %s\n", mark, c.ID, c.Detail)
		if c.Fix != "" && c.Status != doctorOK {
			for i, line := range strings.Split(c.Fix, "\n") {
				prefix := "fix: "
				if i > 0 {
					prefix = "     "
				}
				fmt.Fprintf(w, "      %s%s\n", prefix, line)
			}
		}
	}
	if len(r.NotDetected) > 0 {
		fmt.Fprintf(w, "\nnot detected (no hook file, skipped): %s\n", strings.Join(r.NotDetected, ", "))
	}
	if verbose && len(r.EffortUnknownModels) > 0 {
		fmt.Fprintf(w, "\nmodels without a verified efforts list:\n")
		for _, m := range r.EffortUnknownModels {
			fmt.Fprintf(w, "  %s\n", m)
		}
	}
	switch {
	case r.Failed > 0:
		fmt.Fprintf(w, "\n%d failed, %d warnings\n", r.Failed, r.Warnings)
	case r.Warnings > 0:
		fmt.Fprintf(w, "\nno failures, %d warnings\n", r.Warnings)
	default:
		fmt.Fprintf(w, "\nall good\n")
	}
}

func buildDoctorReport(env doctorEnv) *doctorReport {
	rep := &doctorReport{Version: buildVersion, Commit: buildCommit}
	ver := "downshift " + buildVersion
	if buildCommit != "" {
		ver += " (commit " + buildCommit + ")"
	}
	rep.add(doctorCheck{ID: "version", Status: doctorOK, Detail: ver})

	pathBin := checkBinary(rep, env)
	events := checkState(rep, env)
	checkCatalog(rep, env)
	checkScores(rep)
	checkContext(rep, env)
	checkE5Shadow(rep, env)

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

// checkScores reuses writeScoreStatus, so the refresh trigger and wording stay
// in one place.
func checkScores(rep *doctorReport) {
	var b bytes.Buffer
	writeScoreStatus(&b)
	line := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(b.String()), "scores: "))
	status := doctorOK
	fix := ""
	if strings.HasPrefix(line, "stale") || strings.HasPrefix(line, scoreStatusNone) {
		status, fix = doctorWarn, "downshift models scores --refresh"
	}
	rep.add(doctorCheck{ID: "scores cache", Status: status, Detail: line, Fix: fix})
}

func checkE5Shadow(rep *doctorReport, env doctorEnv) {
	shadow := strings.TrimSpace(env.Getenv(e5shadow.EnableEnv)) == "1"
	fuse := strings.TrimSpace(env.Getenv(e5shadow.IntegrateEnv)) == "1"
	if !shadow && !fuse {
		return
	}
	id := "e5 shadow"
	switch {
	case fuse && shadow:
		id = "e5 encoder"
	case fuse:
		id = "e5 fuse"
	}
	cmd := e5shadow.EmbedCmdFrom(env.Getenv)
	proto := strings.TrimSpace(env.Getenv(e5shadow.PrototypesEnv))
	if cmd == "" {
		rep.add(doctorCheck{
			ID:     id,
			Status: doctorWarn,
			Detail: "E5 enabled but embed cmd is empty (fail-open; fuse needs DOWNSHIFT_E5_CMD, shadow needs DOWNSHIFT_E5_SHADOW_CMD)",
			Fix:    "set DOWNSHIFT_E5_CMD (fuse) and/or DOWNSHIFT_E5_SHADOW_CMD (shadow) to a local ONNX helper",
		})
		return
	}
	if !e5shadow.CmdTrusted(cmd) {
		rep.add(doctorCheck{
			ID:     id,
			Status: doctorWarn,
			Detail: "E5 command is not a trusted local helper (absolute binary or python3 + absolute script)",
			Fix:    "use an absolute path to a reviewed embed helper; never curl/wget/sh -c",
		})
		return
	}
	if proto == "" {
		rep.add(doctorCheck{
			ID:     id,
			Status: doctorWarn,
			Detail: "DOWNSHIFT_E5_PROTOTYPES is empty",
			Fix:    "set DOWNSHIFT_E5_PROTOTYPES to a labs-built PrototypeStore JSON under ~/.downshift/",
		})
		return
	}
	detail := "shadow enabled with local helper and prototypes path configured"
	if fuse && !shadow {
		detail = "fuse enabled with local helper and prototypes path configured"
	} else if fuse && shadow {
		detail = "fuse+shadow enabled with local helper and prototypes path configured"
	}
	rep.add(doctorCheck{
		ID:     id,
		Status: doctorOK,
		Detail: detail,
	})
}

func checkContext(rep *doctorReport, env doctorEnv) {
	optCfg, _ := contextopt.LoadConfig()
	if optCfg.Enabled {
		rep.add(doctorCheck{ID: "context optim.", Status: doctorOK, Detail: "enabled (provider: " + optCfg.Provider + ")"})
	} else {
		rep.add(doctorCheck{ID: "context optim.", Status: doctorOK, Detail: "disabled (enable with `downshift context enable`)"})
	}
	checkCompanion(rep, env, optCfg)
	if env.Verbose {
		cmd := env.Getenv("DOWNSHIFT_MINILM_EMBED")
		if cmd != "" && cmd != "hash" {
			rep.add(doctorCheck{ID: "semantic boost", Status: doctorOK, Detail: "external command configured (" + cmd + ", hash fallback on failure)"})
		} else {
			rep.add(doctorCheck{ID: "semantic boost", Status: doctorOK, Detail: "in-process hash embedder (default, zero-dependency)"})
		}
	}
}

// checkCompanion reports the Headroom companion: recorded and present is ok;
// recorded but missing, or present but not recorded, is a warn with the exact
// fix. Absent and not recorded prints only in verbose: it is optional.
func checkCompanion(rep *doctorReport, env doctorEnv, optCfg contextopt.Config) {
	hp := &headroom.Provider{LookPath: env.LookPath, CommandRunner: env.Run}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	installed, ver, _, err := hp.Detect(ctx)
	recorded := optCfg.Companion == headroom.ProviderID
	label := headroom.ProviderID
	if ver != "" {
		label += " v" + ver
	}
	switch {
	case err != nil:
		rep.add(doctorCheck{ID: "companion", Status: doctorWarn, Detail: err.Error(), Fix: "reinstall: " + headroom.InstallHint})
	case recorded && installed:
		rep.add(doctorCheck{ID: "companion", Status: doctorOK, Detail: label + " recorded; wrap each harness with `headroom wrap <tool>` (Downshift cannot see whether a session is wrapped)"})
	case recorded:
		rep.add(doctorCheck{ID: "companion", Status: doctorWarn, Detail: "headroom recorded but not on PATH", Fix: "downshift context disable headroom"})
	case installed:
		rep.add(doctorCheck{ID: "companion", Status: doctorWarn, Detail: label + " detected, not recorded (savings would be attributed to routing alone)", Fix: "downshift context enable headroom"})
	case env.Verbose:
		rep.add(doctorCheck{ID: "companion", Status: doctorOK, Detail: "none (optional: Headroom, docs/guides/companions.md)"})
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
