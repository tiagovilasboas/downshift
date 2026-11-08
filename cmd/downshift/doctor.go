// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
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

	"github.com/tiagovilasboas/downshift-ai/internal/catalog"
	"github.com/tiagovilasboas/downshift-ai/internal/contextopt"
	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/gateway"
	"github.com/tiagovilasboas/downshift-ai/internal/paths"
	"github.com/tiagovilasboas/downshift-ai/internal/setup"
)

// doctorEnv is everything doctor reads from the machine, so tests can supply a
// temp HOME and fake binaries.
type doctorEnv struct {
	Home       string
	Executable func() (string, error)
	LookPath   func(string) (string, error)
	// PathDirs lists the PATH directories in order; nil skips the shadow check.
	PathDirs func() []string
	// PathCopies lists the distinct downshift executables on PATH and RunVersion
	// runs `<path> version`; nil skips the newest-on-PATH check.
	PathCopies func() []string
	RunVersion func(path string) (string, error)
	// Latest looks up the newest release; set only by `doctor --check-latest`.
	Latest func() (string, bool)
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
	Harness  string
}

func defaultDoctorEnv(cat *catalog.Catalog) doctorEnv {
	home, _ := os.UserHomeDir()
	return doctorEnv{Home: home, Executable: os.Executable, LookPath: exec.LookPath,
		PathDirs:   func() []string { return filepath.SplitList(os.Getenv("PATH")) },
		PathCopies: func() []string { return pathCopies(filepath.SplitList(os.Getenv("PATH"))) },
		RunVersion: runVersionOf,
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
	checkLatest := fs.Bool("check-latest", false, "also ask GitHub for the latest release (one request, cached 24 h)")
	harness := fs.String("harness", "", "check only this harness alongside global checks")
	selfTest := fs.Bool("self-test", false, "run isolated component checks")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintln(errW, "usage: downshift doctor [--verbose] [--quiet] [--json] [--check-latest] [--harness NAME]")
		return 2
	}
	if *harness != "" {
		found := false
		for _, nc := range core.AllHarnessCaps() {
			found = found || nc.Harness == *harness
		}
		if !found {
			fmt.Fprintf(errW, "unknown harness: %s\n", *harness)
			return 2
		}
	}
	if *selfTest && *harness == "" {
		fmt.Fprintln(errW, "--self-test requires --harness NAME")
		return 2
	}
	env := defaultDoctorEnv(cat)
	env.Verbose = *verbose
	env.Harness = *harness
	if *checkLatest {
		if cache, err := paths.Join("latest_release.json"); err == nil {
			env.Latest = defaultLatestLookup(cache).Latest
		}
	}
	rep := buildDoctorReport(env)
	if *selfTest {
		runDoctorSelfTest(rep, env)
		rep.OK = rep.Failed == 0
	}
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

	// home and width shape the text output only (doctor_text.go). Remediation
	// commands remain intact even when they exceed the terminal width.
	home    string
	width   int
	harness string
}

func (r *doctorReport) add(c doctorCheck) {
	if r.harness != "" && c.Harness != "" && c.Harness != r.harness {
		return
	}
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
// declares one (data on setup.Targets; doctor names no harness itself).
func pluginInstallHint() string {
	var parts []string
	for _, t := range setup.Targets {
		if t.Plugin != "" {
			parts = append(parts, fmt.Sprintf("%s: /plugin install %s --marketplace tiagovilasboas/downshift-ai", t.Name, strings.SplitN(t.Plugin, "@", 2)[0]))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return " (or the plugin, " + strings.Join(parts, "; ") + ")"
}

func buildDoctorReport(env doctorEnv) *doctorReport {
	version, commit := currentBuild()
	rep := &doctorReport{Version: version, Commit: commit, harness: env.Harness}
	ver := "downshift " + version
	if commit != "" {
		ver += " (commit " + commit + ")"
	}
	rep.add(doctorCheck{ID: "version", Status: doctorOK, Detail: ver})
	rep.add(buildProvenanceCheck(env))

	pathBin := checkBinary(rep, env)
	checkBinaryShadow(rep, env)
	checkVersionSync(rep, env, version)
	events := checkState(rep, env)
	checkCatalog(rep, env)
	checkContext(rep, env)
	checkSecurityAuditSkill(rep, env)
	checkGateways(rep, env)
	checkWorkers(rep, env)
	checkCircuits(rep, events)
	if env.Harness != "" && env.Harness != "claude-code" {
		for _, target := range setup.Targets {
			if target.Name == env.Harness {
				checkMcpTargets(func(id, status, detail, fix string) {
					rep.add(doctorCheck{ID: id, Harness: env.Harness, Status: status, Detail: detail, Fix: fix})
				}, env.Home, []setup.Target{target})
			}
		}
	}
	for _, note := range modelOverrideNotes(core.AllHarnessCaps(), env.Getenv) {
		name := strings.TrimPrefix(note, "model_override_env: ")
		if i := strings.Index(name, "="); i > 0 {
			name = name[:i]
		}
		rep.add(doctorCheck{ID: "model override", Status: doctorWarn, Detail: note, Fix: "unset " + name})
	}

	for _, nc := range core.AllHarnessCaps() {
		if env.Harness != "" && nc.Harness != env.Harness {
			continue
		}
		wiring, ok := doctorWiringFor(nc.Harness)
		if !ok || !fileExists(filepath.Join(env.Home, wiring.File)) {
			harness := nc.Harness
			add := func(id, status, detail, fix string) {
				rep.add(doctorCheck{ID: id, Harness: harness, Status: status, Detail: detail, Fix: fix})
			}
			if !checkGeneratedFiles(add, env.Home, harness) {
				rep.NotDetected = append(rep.NotDetected, harness)
			}
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
