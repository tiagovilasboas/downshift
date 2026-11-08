// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/setup"
)

// doctor reads the KiroCrew wiring from the same table setup writes from.
func TestDoctorRecognisesTheWrapperSetupWrites(t *testing.T) {
	w, ok := doctorWiringFor("kirocrew")
	if !ok || !w.Setup || len(w.Hooks) < 1 || !w.Hooks[0].Required {
		t.Fatalf("kirocrew wiring = %+v, ok=%v: want at least one required hook that setup can write", w, ok)
	}
	for _, cmd := range []string{
		filepath.Join("/h", ".downshift", "bin", setup.WrapperFile("kirocrew-pre-tool-use", runtime.GOOS)),
		"/repo/scripts/kirocrew-pre-tool-use.sh", // the install guide's manual way still counts
	} {
		if _, ok := matchHook(cmd, w.Hooks[0]); !ok {
			t.Errorf("doctor does not recognise %s", cmd)
		}
	}
	if _, ok := matchHook("/x/guard.py", w.Hooks[0]); ok {
		t.Error("doctor took an unrelated hook for the KiroCrew one")
	}
}

func TestDoctorWarnsWhenCrewRegistryStillHasDownshift(t *testing.T) {
	home := t.TempDir()
	body := `{"hooks":[{"name":"Downshift KiroCrew","command":"/Users/x/.local/bin/downshift kirocrew","enabled":true}]}`
	if err := os.MkdirAll(filepath.Join(home, ".kiro", "crew"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".kiro", "crew", "hooks.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	var got doctorCheck
	add := func(id, status, detail, fix string) {
		got = doctorCheck{ID: id, Status: status, Detail: detail, Fix: fix}
	}
	checkCrewRegistry(add, doctorEnv{Home: home}, "kirocrew", doctorWiring{Registry: ".kiro/crew/hooks.json"})
	if got.Status != doctorWarn || got.Fix != "downshift setup kirocrew" {
		t.Fatalf("check = %+v", got)
	}
	if !strings.Contains(got.Detail, "1 Downshift") {
		t.Fatalf("detail = %s", got.Detail)
	}
}
