// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/tiagovilasboas/downshift-ai/internal/setup"
)

// Feature flags and the activation step a harness needs before its hooks run
// (setup.Target.FeatureFlags and Activate). Harness facts are data on the
// target row; nothing here names a harness.

// Flag states read by readFeatureFlag.
const (
	featureUnset = "unset" // file or key absent: the flag's Default applies
	featureOn    = "on"
	featureOff   = "off"
)

// readFeatureFlag reports the flag's state in its file under home. A missing
// file is featureUnset; a file that does not parse, or a value of another type,
// is an error. The current key is read before its aliases.
func readFeatureFlag(home string, f setup.FeatureFlag) (string, error) {
	raw, err := os.ReadFile(filepath.Join(home, filepath.FromSlash(f.File)))
	if errors.Is(err, fs.ErrNotExist) {
		return featureUnset, nil
	}
	if err != nil {
		return "", err
	}
	var doc map[string]any
	if err := toml.Unmarshal(raw, &doc); err != nil {
		return "", err
	}
	table, _ := doc[f.Table].(map[string]any)
	for _, key := range append([]string{f.Key}, f.Aliases...) {
		v, ok := table[key]
		if !ok {
			continue
		}
		if t, isTable := v.(map[string]any); isTable {
			if v, ok = t["enabled"]; !ok {
				return featureUnset, nil
			}
		}
		on, isBool := v.(bool)
		if !isBool {
			return "", fmt.Errorf("%s.%s is not a boolean", f.Table, key)
		}
		if on {
			return featureOn, nil
		}
		return featureOff, nil
	}
	return featureUnset, nil
}

// checkFeatureFlags reports each flag the harness declares: on, off, or unset
// with the harness default.
func checkFeatureFlags(add func(id, status, detail, fix string), home string, w doctorWiring) {
	for _, f := range w.FeatureFlags {
		path := filepath.Join(home, filepath.FromSlash(f.File))
		id := "feature " + f.Key
		fix := fmt.Sprintf("set %s in %s", f.Line(), path)
		state, err := readFeatureFlag(home, f)
		switch {
		case err != nil:
			add(id, doctorWarn, fmt.Sprintf("cannot read %s: %v", path, err), "fix the file so it parses, then "+fix)
		case state == featureOn:
			add(id, doctorOK, f.Line(), "")
		case state == featureOff:
			add(id, doctorWarn, fmt.Sprintf("%s.%s is false in %s: %s", f.Table, f.Key, path, f.Purpose), fix)
		case f.Default:
			detail := fmt.Sprintf("not set in %s; on by default", path)
			if f.DefaultNote != "" {
				detail += " (" + f.DefaultNote + ")"
			}
			add(id, doctorOK, detail, "")
		default:
			add(id, doctorWarn, fmt.Sprintf("not set in %s and off by default: %s", path, f.Purpose), fix)
		}
	}
}

// activationFix is the fix for a harness whose hooks are wired but never ran:
// turn on each flag that gates every hook and is not explicitly on (an unset
// flag that defaults on is named too, because earlier releases default it
// off), then the harness's own activation step. Empty when the harness
// declares neither, so the caller keeps its generic fix.
func activationFix(home string, w doctorWiring) string {
	var steps []string
	for _, f := range w.FeatureFlags {
		if !f.AllHooks {
			continue
		}
		if state, err := readFeatureFlag(home, f); err == nil && state == featureOn {
			continue
		}
		steps = append(steps, fmt.Sprintf("set %s in %s", f.Line(), filepath.Join(home, filepath.FromSlash(f.File))))
	}
	if w.Activate != "" {
		steps = append(steps, w.Activate)
	}
	if len(steps) == 0 {
		return ""
	}
	return strings.Join(steps, "; ") + "; then run one command or spawn and check: downshift stats --days=1"
}
