// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestModeLayering checks the top-level mode key across the two files: an
// absent key inherits the other file, and the project file wins when both set it.
func TestModeLayering(t *testing.T) {
	cases := []struct {
		name    string
		project string // "" leaves the project file absent
		user    string // "" leaves the user file absent
		want    Mode
	}{
		{"no files is standard", "", "", ModeStandard},
		{"project economy", `{"version":1,"mode":"economy"}`, "", ModeEconomy},
		{"user economy inherited", "", `{"version":1,"mode":"economy"}`, ModeEconomy},
		{"project standard overrides user economy", `{"version":1,"mode":"standard"}`, `{"version":1,"mode":"economy"}`, ModeStandard},
		{"project file without mode inherits user economy", `{"version":1,"min_tier":"mid"}`, `{"version":1,"mode":"economy"}`, ModeEconomy},
		{"mode beside a routing section", `{"version":1,"mode":"economy","routing":{"critical_threshold":70}}`, "", ModeEconomy},
		{"mode beside a tier floor", `{"version":1,"min_tier":"mid","mode":"economy"}`, "", ModeEconomy},
		{"null mode is absent", `{"version":1,"mode":null}`, "", ModeStandard},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			loc := Locations{}
			if c.project != "" {
				loc.Project = projectFile(t, c.project)
			}
			if c.user != "" {
				loc.User = userFile(t, c.user)
			}
			p, err := Load(loc)
			if err != nil {
				t.Fatalf("Load refused: %v", err)
			}
			if p.Mode != c.want {
				t.Errorf("Mode = %q, want %q", p.Mode, c.want)
			}
			if p.EconomyMode() != (c.want == ModeEconomy) {
				t.Errorf("EconomyMode() = %v, want %v", p.EconomyMode(), c.want == ModeEconomy)
			}
		})
	}
}

// TestModeRefusals checks that an unknown mode is refused with its field, in
// either file, and that the strict routing decoder still refuses unknown keys
// beside a mode.
func TestModeRefusals(t *testing.T) {
	cases := []struct {
		name  string
		body  string
		field string
		want  string
	}{
		{"unknown mode name", `{"version":1,"mode":"cheap"}`, "mode", `"cheap" is not standard or economy`},
		{"mode is not a string", `{"version":1,"mode":true}`, "file", "not valid JSON"},
		{"unknown key inside routing still refused", `{"version":1,"mode":"economy","routing":{"bogus":1}}`, "routing", "bogus"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Load(Locations{Project: projectFile(t, c.body)})
			if err == nil {
				t.Fatal("Load accepted the file, want a refusal")
			}
			if !strings.Contains(err.Error(), c.field) || !strings.Contains(err.Error(), c.want) {
				t.Errorf("error %q, want field %q and %q", err, c.field, c.want)
			}
		})
	}
}

// TestModeIsRefusedInUserFileToo checks the same refusal from the user layer.
func TestModeIsRefusedInUserFileToo(t *testing.T) {
	_, err := Load(Locations{User: userFile(t, `{"version":1,"mode":"big"}`)})
	if err == nil || !strings.Contains(err.Error(), "mode") {
		t.Fatalf("user file with unknown mode: err = %v, want a mode refusal", err)
	}
}

// TestFloorAcceptsMode checks the strict tier-floor decoder: it accepts a valid
// mode key next to the floor and refuses an unknown one.
func TestFloorAcceptsMode(t *testing.T) {
	dir := t.TempDir()
	writeFloor(t, dir, `{"version":1,"mode":"economy","min_tier":"mid"}`)
	if _, err := LoadFloor(filepath.Join(dir, filepath.FromSlash(FloorRelPath))); err != nil {
		t.Fatalf("floor refused a valid mode: %v", err)
	}
	dir = t.TempDir()
	writeFloor(t, dir, `{"version":1,"mode":"big"}`)
	if _, err := LoadFloor(filepath.Join(dir, filepath.FromSlash(FloorRelPath))); err == nil {
		t.Fatal("floor accepted an unknown mode")
	}
}

// TestEconomyModeNilPolicy checks the nil-safe accessor used by decide.
func TestEconomyModeNilPolicy(t *testing.T) {
	var p *Policy
	if p.EconomyMode() {
		t.Error("nil policy reported economy mode")
	}
	if (&Policy{}).EconomyMode() {
		t.Error("zero policy reported economy mode")
	}
}
