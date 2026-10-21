// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
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
