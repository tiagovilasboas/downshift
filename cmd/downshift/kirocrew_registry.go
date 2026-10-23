// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// planCrewRegistry plans removal of a Downshift row from the harness's
// informational hook list. The policy hook stays in the target's config file.
// A missing list, or one with no Downshift row, is not a plan.
func planCrewRegistry(o setupOptions, t setupTarget) (setupPlan, bool) {
	if t.Registry == "" {
		return setupPlan{}, false
	}
	p := setupPlan{
		target: t,
		path:   filepath.Join(o.Home, filepath.FromSlash(t.Registry)),
		notes: []string{
			"This list does not enforce policy. A Downshift row here duplicates agent.kiro_hooks and is removed; other rows stay.",
			"Quit KiroCrew before this write, then start it again. A running Crew keeps the list in memory and saves that copy back over the file on the next hook run.",
		},
	}
	raw, err := os.ReadFile(p.path)
	if errors.Is(err, os.ErrNotExist) {
		return setupPlan{}, false
	}
	if err != nil {
		p.err = err
		return p, true
	}
	after, removed, err := stripDownshiftCrewHooks(raw)
	if err != nil {
		p.err = fmt.Errorf("not valid JSON, left untouched: %w", err)
		return p, true
	}
	if removed == 0 {
		return setupPlan{}, false
	}
	p.existed = true
	p.before = raw
	p.after = after
	return p, true
}

// stripDownshiftCrewHooks drops Downshift KiroCrew rows from a Crew hooks
// document. removed is 0 when raw is returned unchanged.
func stripDownshiftCrewHooks(raw []byte) (out []byte, removed int, err error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var doc map[string]any
	if err := dec.Decode(&doc); err != nil {
		return nil, 0, err
	}
	hooks, ok := doc["hooks"].([]any)
	if !ok {
		return raw, 0, nil
	}
	kept := make([]any, 0, len(hooks))
	for _, item := range hooks {
		h, ok := item.(map[string]any)
		if ok && crewHookIsDownshift(h) {
			removed++
			continue
		}
		kept = append(kept, item)
	}
	if removed == 0 {
		return raw, 0, nil
	}
	doc["hooks"] = kept
	buf, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, 0, err
	}
	return append(buf, '\n'), removed, nil
}

// crewHookIsDownshift reports the informational "Downshift KiroCrew" row,
// including one whose command still points at a checkout binary.
func crewHookIsDownshift(h map[string]any) bool {
	if name, _ := h["name"].(string); name == "Downshift KiroCrew" {
		return true
	}
	cmd, _ := h["command"].(string)
	tok := strings.Fields(cmd)
	if len(tok) != 2 || tok[1] != "kirocrew" {
		return false
	}
	base := strings.ToLower(filepath.Base(tok[0]))
	return base == "downshift" || base == "downshift.exe"
}

// checkCrewRegistry warns when the informational list still has a Downshift
// row. Harnesses with no Registry are skipped.
func checkCrewRegistry(add func(id, status, detail, fix string), env doctorEnv, harness string, w doctorWiring) {
	if w.Registry == "" {
		return
	}
	path := filepath.Join(env.Home, filepath.FromSlash(w.Registry))
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	if err != nil {
		add("crew registry", doctorWarn, fmt.Sprintf("cannot read %s: %v", w.Registry, err), "downshift setup "+harness)
		return
	}
	_, removed, err := stripDownshiftCrewHooks(raw)
	if err != nil {
		add("crew registry", doctorWarn, fmt.Sprintf("%s is not valid JSON: %v", w.Registry, err), "downshift setup "+harness)
		return
	}
	if removed == 0 {
		return
	}
	add("crew registry", doctorWarn,
		fmt.Sprintf("%s has %d Downshift row(s); that list does not enforce policy and duplicates the native hook", w.Registry, removed),
		"downshift setup "+harness)
}
