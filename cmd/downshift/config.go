// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/paths"
	"github.com/tiagovilasboas/downshift/internal/scoreindex"
	"github.com/tiagovilasboas/downshift/internal/semantic"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// Where a configuration value comes from.
const (
	cfgEnv     = "env"     // an environment variable set for this process
	cfgFile    = "file"    // a file under the state directory (or one it points to)
	cfgDefault = "default" // nothing configured: the built-in behaviour
	cfgLegacy  = "legacy"  // state dir only: the pre-rename ~/.harness-downshift
)

// configEntry is one effective setting. Env names the variable that controls it
// (also when it is unset), so the output doubles as the list of knobs.
type configEntry struct {
	Key     string `json:"key"`
	Harness string `json:"harness,omitempty"`
	Value   string `json:"value"`
	Source  string `json:"source"` // env | file | default | legacy
	Env     string `json:"env,omitempty"`
	Note    string `json:"note,omitempty"`
}

type configReport struct {
	Settings []configEntry `json:"settings"`
}

// runConfig is `downshift config [--json]`: every effective setting with its
// value and source. Read-only: it reads the same inputs the hooks read and
// writes nothing.
func runConfig(args []string, w, errW io.Writer, cat *catalog.Catalog) int {
	fs := flag.NewFlagSet("config", flag.ContinueOnError)
	fs.SetOutput(errW)
	asJSON := fs.Bool("json", false, "machine-readable settings")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintln(errW, "usage: downshift config [--json]")
		return 2
	}
	rep := buildConfigReport(os.Getenv, time.Now(), cat)
	if *asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(rep)
		return 0
	}
	rep.write(w)
	return 0
}

func (r configReport) write(w io.Writer) {
	fmt.Fprintln(w, "downshift config (value, then where it comes from)")
	width := 0
	label := func(e configEntry) string {
		if e.Harness != "" {
			return e.Key + "." + e.Harness
		}
		return e.Key
	}
	for _, e := range r.Settings {
		if n := len(label(e)); n > width {
			width = n
		}
	}
	for _, e := range r.Settings {
		line := fmt.Sprintf("  %-*s  %s  [%s", width, label(e), e.Value, e.Source)
		if e.Env != "" && e.Source == cfgEnv {
			line += " " + e.Env
		}
		line += "]"
		if e.Note != "" {
			line += "  " + e.Note
		}
		fmt.Fprintln(w, line)
	}
}

// envVal returns the variable's value and whether it is set and non-empty.
func envVal(getenv func(string) string, name string) (string, bool) {
	v := strings.TrimSpace(getenv(name))
	return v, v != ""
}

func buildConfigReport(getenv func(string) string, now time.Time, cat *catalog.Catalog) configReport {
	var out []configEntry
	add := func(e configEntry) { out = append(out, e) }

	// State directory.
	if dir, src, err := paths.Resolve(); err != nil {
		add(configEntry{Key: "state_dir", Value: "unresolved: " + err.Error(), Source: cfgDefault, Env: paths.EnvStateDir})
	} else {
		s := cfgDefault
		switch src {
		case paths.SourceEnv:
			s = cfgEnv
		case paths.SourceLegacy:
			s = cfgLegacy
		}
		add(configEntry{Key: "state_dir", Value: dir, Source: s, Env: paths.EnvStateDir})
	}

	// Session model list, per harness, in the order the hooks resolve it.
	for _, nc := range core.AllHarnessCaps() {
		add(sessionConfigEntry(nc, getenv, now))
	}

	// Quota gate.
	if v, ok := envVal(getenv, "DOWNSHIFT_QUOTA_MODE"); ok && v == "required" {
		add(configEntry{Key: "quota_mode", Value: "required", Source: cfgEnv, Env: "DOWNSHIFT_QUOTA_MODE",
			Note: "rewrites need available quota evidence, even from a stale or absent snapshot"})
	} else {
		add(configEntry{Key: "quota_mode", Value: "default", Source: cfgDefault, Env: "DOWNSHIFT_QUOTA_MODE",
			Note: "no snapshot: rewrites allowed; an expired snapshot holds for 1 h, then is ignored"})
	}

	// Public capability scores.
	if scoreindex.Disabled() {
		add(configEntry{Key: "scores", Value: "off", Source: cfgEnv, Env: scoreindex.EnvSwitch, Note: "no network, router ignores scores.json"})
	} else {
		add(configEntry{Key: "scores", Value: "on", Source: cfgDefault, Env: scoreindex.EnvSwitch})
	}
	if core.ScoreRoutingEnabled() {
		add(configEntry{Key: "score_routing", Value: "live", Source: cfgEnv, Env: core.ScoreRoutingEnv, Note: "the score pick changes the routed model"})
	} else {
		add(configEntry{Key: "score_routing", Value: "shadow", Source: cfgDefault, Env: core.ScoreRoutingEnv, Note: "recorded as score_would, never applied"})
	}

	// Control group.
	share := strconv.FormatFloat(telemetry.ControlShare(), 'f', -1, 64)
	shareSrc := cfgDefault
	if raw, ok := envVal(getenv, telemetry.ControlShareEnv); ok {
		if _, err := strconv.ParseFloat(raw, 64); err == nil {
			shareSrc = cfgEnv // an unparseable value falls back to the default
		}
	}
	add(configEntry{Key: "control_share", Value: share, Source: shareSrc, Env: telemetry.ControlShareEnv,
		Note: "fraction of spawns left unrouted as a measured baseline"})

	// Quiet.
	if telemetry.Quiet() {
		add(configEntry{Key: "quiet", Value: "on", Source: cfgEnv, Env: "DOWNSHIFT_QUIET", Note: "no routing text inside the harness"})
	} else {
		add(configEntry{Key: "quiet", Value: "off", Source: cfgDefault, Env: "DOWNSHIFT_QUIET"})
	}

	// Catalog.
	if p, err := paths.CatalogPath(); err == nil && fileExists(p) {
		add(configEntry{Key: "catalog", Value: "user override " + p, Source: cfgFile})
	} else {
		e := configEntry{Key: "catalog", Value: "embedded", Source: cfgDefault}
		if cat != nil {
			e.Note = fmt.Sprintf("%d entries", len(cat.Entries()))
		}
		add(e)
	}

	// Semantic boost.
	if semantic.Enabled() {
		add(configEntry{Key: "semantic_boost", Value: "on (hash embedder)", Source: cfgDefault, Env: "DOWNSHIFT_SEMANTIC_BOOST"})
	} else {
		add(configEntry{Key: "semantic_boost", Value: "off", Source: cfgEnv, Env: "DOWNSHIFT_SEMANTIC_BOOST"})
	}

	return configReport{Settings: out}
}

// sessionConfigEntry mirrors the order the hooks use for the allowlist: native
// export, discovery cache, operator file. Nothing else supplies candidates.
func sessionConfigEntry(nc core.NamedCaps, getenv func(string) string, now time.Time) configEntry {
	e := configEntry{Key: "session_models", Harness: nc.Harness}
	if nc.Caps.NativeSessionEnv != "" {
		e.Env = nc.Caps.NativeSessionEnv
	}
	if s, present := core.LoadNativeSession(nc.Harness, now); present {
		e.Source, e.Env = cfgEnv, nc.Caps.NativeSessionEnv
		if s.Known {
			e.Value = fmt.Sprintf("native export (%d models)", len(s.IDs))
		} else {
			e.Value = "native export unreadable: rewrites held"
		}
		return e
	}
	if s := core.LoadDiscoveredSession(nc.Harness, now); s.Known {
		e.Value, e.Source, e.Env = fmt.Sprintf("discovered (%d models)", len(s.IDs)), cfgFile, core.EnvDiscovered
		if _, set := envVal(getenv, core.EnvDiscovered); set {
			e.Source = cfgEnv
		}
		return e
	}
	if s := core.LoadUserSession(nc.Harness); s.Known {
		e.Value, e.Source, e.Env = fmt.Sprintf("session-models.json (%d models)", len(s.IDs)), cfgFile, "DOWNSHIFT_SESSION_MODELS"
		if _, set := envVal(getenv, "DOWNSHIFT_SESSION_MODELS"); set {
			e.Source = cfgEnv
		}
		return e
	}
	e.Source = cfgDefault
	e.Value, e.Env = "none: rewrites held", "DOWNSHIFT_SESSION_MODELS"
	return e
}
