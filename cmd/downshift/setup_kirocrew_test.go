// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// `downshift setup` wires the KiroCrew preToolUse hook through a generated
// wrapper. The wrapper machinery is tested in setup_wrapper_test.go with a
// synthetic harness; these tests hold the real KiroCrew row of the table to it:
// where its config lives, what it registers, and that doctor agrees.

const (
	kirocrewConfig = ".kiro/crew/config.json"
	kirocrewStem   = "kirocrew-pre-tool-use"
	kirocrewBefore = `{
  "timezone": "America/Sao_Paulo",
  "agent": {"kiro_hooks": {"preToolUse": [{"matcher": "*", "command": "` + guardHook + `"}]}}
}
`
)

func kirocrewWrapper(e *setupEnv) string {
	return e.path(filepath.Join(".downshift", "bin", wrapperFile(kirocrewStem, runtime.GOOS)))
}

func TestSetupKiroCrewRegistersTheWrapperNextToTheExistingHook(t *testing.T) {
	e := newSetupEnv(t, ".kiro")
	e.write(kirocrewConfig, kirocrewBefore)

	code, out, errOut := e.run("--yes")
	if code != 0 {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	got := strings.Join(preToolUseCommands(t, e.read(kirocrewConfig)), "|")
	if want := guardHook + "|" + kirocrewWrapper(e); got != want {
		t.Fatalf("preToolUse = %s, want the operator's hook kept and the wrapper added: %s", got, want)
	}
	if body := e.read(filepath.Join(".downshift", "bin", wrapperFile(kirocrewStem, runtime.GOOS))); !strings.Contains(body, testBin) || !strings.Contains(body, " kirocrew") {
		t.Errorf("the wrapper does not run %s kirocrew:\n%s", testBin, body)
	}
	if !strings.Contains(out, "Restart KiroCrew") {
		t.Errorf("no restart note:\n%s", out)
	}
	// Undo puts the config back as it was.
	if code, out, _ := e.run("--undo", "--yes"); code != 0 || e.read(kirocrewConfig) != kirocrewBefore {
		t.Errorf("undo exit %d, config restored=%v\n%s", code, e.read(kirocrewConfig) == kirocrewBefore, out)
	}
}

// The install guide's manual way (the wrapper in a checkout) is the same hook:
// it is pointed at the generated one, not added a second time.
func TestSetupKiroCrewReplacesTheManualWrapperOfTheInstallGuide(t *testing.T) {
	manual := strings.Replace(kirocrewBefore, guardHook, "/home/dev/downshift/scripts/kirocrew-pre-tool-use.sh", 1)
	e := newSetupEnv(t, ".kiro")
	e.write(kirocrewConfig, manual)
	if code, out, _ := e.run("--yes"); code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if got := preToolUseCommands(t, e.read(kirocrewConfig)); len(got) != 1 || got[0] != kirocrewWrapper(e) {
		t.Errorf("preToolUse = %v, want only the generated wrapper", got)
	}
}

// doctor reads the KiroCrew wiring from the same table setup writes from.
func TestDoctorRecognisesTheWrapperSetupWrites(t *testing.T) {
	w, ok := doctorWiringFor("kirocrew")
	if !ok || !w.Setup || len(w.Hooks) < 1 || !w.Hooks[0].Required {
		t.Fatalf("kirocrew wiring = %+v, ok=%v: want at least one required hook that setup can write", w, ok)
	}
	for _, cmd := range []string{
		filepath.Join("/h", ".downshift", "bin", wrapperFile(kirocrewStem, runtime.GOOS)),
		"/repo/scripts/kirocrew-pre-tool-use.sh", // the install guide's manual way still counts
	} {
		if _, ok := matchHook(cmd, w.Hooks[0]); !ok {
			t.Errorf("doctor does not recognise %s", cmd)
		}
	}
	if _, ok := matchHook(guardHook, w.Hooks[0]); ok {
		t.Error("doctor took an unrelated hook for the KiroCrew one")
	}
}

// The Crew hook page is not the policy hook. A Downshift row there, even one
// already pointed at the installed binary and enabled, is removed. A neighbour
// row stays. Undo puts the page back.
func TestSetupKiroCrewRemovesInformationalRegistry(t *testing.T) {
	e := newSetupEnv(t, ".kiro")
	e.write(kirocrewConfig, kirocrewBefore)
	const registry = ".kiro/crew/hooks.json"
	before := `{
  "hooks": [
    {
      "name": "Downshift KiroCrew",
      "command": "/home/dev/Github/harness-downshift/downshift kirocrew",
      "enabled": false
    },
    {
      "name": "Other",
      "command": "/usr/bin/true",
      "enabled": true
    }
  ]
}
`
	e.write(registry, before)
	if code, out, errOut := e.run("--yes"); code != 0 {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	if strings.Contains(e.read(registry), "downshift") || strings.Contains(e.read(registry), "Downshift") {
		t.Fatalf("registry still has Downshift:\n%s", e.read(registry))
	}
	if !strings.Contains(e.read(registry), "/usr/bin/true") {
		t.Fatalf("neighbour hook was dropped:\n%s", e.read(registry))
	}
	if code, out, _ := e.run("--undo", "--yes"); code != 0 || e.read(registry) != before {
		t.Fatalf("undo exit %d, registry restored=%v\n%s", code, e.read(registry) == before, out)
	}
}

func TestSetupKiroCrewLeavesACleanRegistryAlone(t *testing.T) {
	e := newSetupEnv(t, ".kiro")
	e.write(kirocrewConfig, kirocrewBefore)
	const registry = ".kiro/crew/hooks.json"
	clean := "{\"hooks\":[{\"name\":\"Other\",\"command\":\"/usr/bin/true\"}]}\n"
	e.write(registry, clean)
	code, out, _ := e.run("--dry-run")
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if strings.Contains(out, "hooks.json") {
		t.Fatalf("dry-run mentioned the registry:\n%s", out)
	}
	if e.read(registry) != clean {
		t.Fatalf("registry changed:\n%s", e.read(registry))
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
