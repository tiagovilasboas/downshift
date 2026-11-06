// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/console/setupwizard"
	"github.com/tiagovilasboas/downshift/internal/setup"
)

const workersPolicyRel = ".downshift/policy.json"

func workersDoc(t *testing.T, e *setupEnv) map[string]any {
	t.Helper()
	return parseDoc(t, e.read(workersPolicyRel))
}

func TestSetupWorkersFlagCreatesPolicy(t *testing.T) {
	for _, mode := range []string{"off", "read", "write"} {
		e := newSetupEnv(t, ".claude")
		args := []string{"--workers", mode, "--yes"}
		code, out, errOut := e.runOpts(args)
		if code != 0 {
			t.Fatalf("%s: exit %d\n%s%s", mode, code, out, errOut)
		}
		doc := workersDoc(t, e)
		orch, _ := doc["orchestration"].(map[string]any)
		if orch["workers"] != mode || doc["version"] != float64(1) {
			t.Errorf("%s: policy = %v", mode, doc)
		}
	}
}

func TestSetupWorkersMergeKeepsOtherKeys(t *testing.T) {
	e := newSetupEnv(t, ".claude")
	e.write(workersPolicyRel, `{"version":1,"routing":{"x":1},"orchestration":{"max_parallel":3,"workers":"off"}}`)
	args := []string{"--workers=read", "--yes"}
	if code, out, errOut := e.runOpts(args); code != 0 {
		t.Fatalf("exit %d\n%s%s", code, out, errOut)
	}
	doc := workersDoc(t, e)
	orch := doc["orchestration"].(map[string]any)
	if orch["workers"] != "read" || orch["max_parallel"] != float64(3) || doc["routing"] == nil {
		t.Errorf("merge lost keys: %v", doc)
	}
	baks, _ := filepath.Glob(e.path(workersPolicyRel) + setup.BackupInfix + "*")
	if len(baks) != 1 {
		t.Errorf("want one backup, got %v", baks)
	}
}

func TestSetupWorkersDryRunShowsDiffAndWritesNothing(t *testing.T) {
	e := newSetupEnv(t, ".claude")
	args := []string{"--workers", "write", "--dry-run"}
	code, out, _ := e.runOpts(args)
	if code != 0 || !strings.Contains(out, "policy.json would change") || !strings.Contains(out, `"workers": "write"`) {
		t.Errorf("exit %d, output:\n%s", code, out)
	}
	if _, err := os.Stat(e.path(workersPolicyRel)); err == nil {
		t.Error("dry run wrote the policy file")
	}
}

func TestSetupYesWithoutWorkersFlagWritesNothing(t *testing.T) {
	e := newSetupEnv(t, ".claude")
	if code, out, _ := e.run("--yes"); code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if _, err := os.Stat(e.path(workersPolicyRel)); err == nil {
		t.Error("--yes alone wrote the policy file")
	}
}

func TestSetupWorkersInvalidValueIsUsageError(t *testing.T) {
	for _, args := range [][]string{{"--workers", "bypass"}, {"--workers="}, {"--workers"}} {
		var out, errw bytes.Buffer
		if code := runSetup(args, strings.NewReader(""), &out, &errw); code != 2 {
			t.Errorf("%v: exit %d, want 2 (%s)", args, code, errw.String())
		}
	}
}

func TestSetupWorkersUsageMentionsFlag(t *testing.T) {
	if !strings.Contains(setupUsage, "--workers off|read|write") {
		t.Error("usage does not mention --workers")
	}
}

func TestSetupWorkersRefusesSymlinkedPolicy(t *testing.T) {
	e := newSetupEnv(t, ".claude", ".downshift")
	real := filepath.Join(t.TempDir(), "real.json")
	if err := os.WriteFile(real, []byte(`{"version":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, e.path(workersPolicyRel)); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	args := []string{"--workers", "read", "--yes"}
	code, _, errOut := e.runOpts(args)
	if code == 0 || !strings.Contains(errOut, "is a link") {
		t.Errorf("exit %d, stderr %q", code, errOut)
	}
	if b, _ := os.ReadFile(real); string(b) != `{"version":1}` {
		t.Errorf("linked file changed: %s", b)
	}
}

func TestWorkersChoiceNeverPreselectsWrite(t *testing.T) {
	c := workersChoice()
	if c.Items[0].ID != "read" || c.Items[len(c.Items)-1].ID != "write" {
		t.Errorf("order = %+v, want read first and write last", c.Items)
	}
}

func TestSelectionCarriesWorkersAnswer(t *testing.T) {
	var o setup.Options
	flags, _, bad := parseSetupArgs([]string{"--workers", "off"}, &o)
	if bad != "" || o.Workers != "off" || flags.noPolicy {
		t.Errorf("parsed %+v bad=%q", o, bad)
	}
}

// runOpts runs setup with the real flag parser, so the flag under test is the
// one the CLI reads.
func (e *setupEnv) runOpts(args []string) (code int, out, errOut string) {
	e.t.Helper()
	var o, er bytes.Buffer
	opts := setup.Options{Home: e.home, Binary: testBin, SkipDiscovery: true, In: strings.NewReader(""), Out: &o, Err: &er}
	opts.Now = func() time.Time {
		e.tick++
		return time.Date(2026, 10, 13, 12, 0, 0, e.tick*1000, time.UTC)
	}
	if _, help, bad := parseSetupArgs(args, &opts); help || bad != "" {
		e.t.Fatalf("args %v: help=%v bad=%q", args, help, bad)
	}
	return setupRun(opts), o.String(), er.String()
}

func TestSetupTUIAsksWorkersAndWritesTheAnswer(t *testing.T) {
	var out, errw bytes.Buffer
	o := tuiOpts(t, strings.NewReader(""), &out, &errw)
	sel := setupwizard.Selection{Harnesses: []string{"claude-code"}, Components: []string{"hooks"}, Choice: "read"}
	asked := false
	withWizard(t, func(wo setupwizard.Options, in io.Reader, w io.Writer) (setupwizard.Selection, bool, error) {
		asked = wo.Choice != nil && wo.Choice.Items[0].ID == "read"
		return sel, true, nil
	})
	if rc := setupTUI(o); rc != 0 {
		t.Fatalf("rc %d\n%s%s", rc, out.String(), errw.String())
	}
	if !asked {
		t.Error("the wizard did not get the workers question")
	}
	b, err := os.ReadFile(filepath.Join(o.StateDir, "policy.json"))
	if err != nil || !strings.Contains(string(b), `"workers": "read"`) {
		t.Errorf("policy = %q, %v", b, err)
	}
	if !strings.Contains(out.String(), "--workers read") {
		t.Error("equivalent command lacks --workers read")
	}
}

func TestSetupTUISkipsQuestionWhenFlagAnswered(t *testing.T) {
	var out, errw bytes.Buffer
	o := tuiOpts(t, strings.NewReader(""), &out, &errw)
	o.Workers = "off"
	withWizard(t, func(wo setupwizard.Options, in io.Reader, w io.Writer) (setupwizard.Selection, bool, error) {
		if wo.Choice != nil {
			t.Error("asked although --workers was given")
		}
		return claudeSelection, false, nil
	})
	setupTUI(o)
}
