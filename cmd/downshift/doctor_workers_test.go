// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"runtime"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/gateway"
)

// workersReport runs checkWorkers against a temp state dir with the given
// DOWNSHIFT_WORKERS value and an optional user policy body.
func workersReport(t *testing.T, workersEnv, userPolicy string, gatewayOn bool) *doctorReport {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("DOWNSHIFT_STATE_DIR", dir)
	t.Chdir(t.TempDir())
	if userPolicy != "" {
		writeFile(t, dir+"/policy.json", userPolicy)
	}
	if gatewayOn {
		if err := gateway.SaveState(gateway.State{Enabled: true, Name: "9router", Endpoint: "http://127.0.0.1:1"}); err != nil {
			t.Fatal(err)
		}
	}
	rep := &doctorReport{}
	checkWorkers(rep, doctorEnv{Getenv: func(k string) string {
		if k == "DOWNSHIFT_WORKERS" {
			return workersEnv
		}
		return ""
	}})
	return rep
}

// defaultWorkersRow is the doctor row for the default: read for the verified
// agent, except on Windows, where the default is off.
func defaultWorkersRow() (want, fix string) {
	if runtime.GOOS == "windows" {
		return "off · source default", workersFixSetup
	}
	return "read (claude) · write off · source default", ""
}

func TestCheckWorkersRows(t *testing.T) {
	defWant, defFix := defaultWorkersRow()
	cases := []struct {
		name, env, user, want, fix string
	}{
		{"default", "", "", defWant, defFix},
		{"read env", "read", "", "read (claude, codex) · write off · source env", ""},
		{"write env", "bypass", "", "read+write (claude, codex) · source env", ""},
		{"read user", "", `{"orchestration":{"workers":"read"}}`, "read (claude, codex) · write off · source user", ""},
		{"off env", "off", "", "off · source env", workersFixSetup},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rep := workersReport(t, c.env, c.user, false)
			if len(rep.Checks) != 1 || rep.Checks[0].ID != "workers" || rep.Checks[0].Status != doctorOK {
				t.Fatalf("checks = %+v", rep.Checks)
			}
			got := rep.Checks[0]
			if !strings.Contains(got.Detail, c.want) {
				t.Errorf("detail %q lacks %q", got.Detail, c.want)
			}
			if c.fix == "" && got.Fix != "" || c.fix != "" && !strings.Contains(got.Fix, c.fix) {
				t.Errorf("fix = %q, want %q", got.Fix, c.fix)
			}
		})
	}
}

func TestCheckWorkersShowsNotes(t *testing.T) {
	rep := workersReport(t, "yes", "", false)
	if d := rep.Checks[0].Detail; !strings.Contains(d, "ignored") || !strings.Contains(d, "source default") {
		t.Errorf("detail = %q", d)
	}
}

func TestCheckWorkersIdleGatewayWarning(t *testing.T) {
	rep := workersReport(t, "off", "", true)
	if len(rep.Checks) != 2 || rep.Warnings != 1 {
		t.Fatalf("checks = %+v", rep.Checks)
	}
	g := rep.Checks[1]
	if g.ID != "gateway" || g.Status != doctorWarn || g.Detail != gatewayIdleMessage || g.Fix != workersFixSetup {
		t.Errorf("gateway row = %+v", g)
	}
	if on := workersReport(t, "read", "", true); len(on.Checks) != 1 || on.Warnings != 0 {
		t.Errorf("workers on must not warn: %+v", on.Checks)
	}
}

func TestDoctorJSONCarriesWorkers(t *testing.T) {
	rep := workersReport(t, "off", "", false)
	b, err := json.Marshal(rep)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Checks []doctorCheck `json:"checks"`
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Checks) != 1 || got.Checks[0].ID != "workers" || got.Checks[0].Fix == "" {
		t.Errorf("json checks = %+v", got.Checks)
	}
}

func TestGatewayOnWarnsWhenWorkersOff(t *testing.T) {
	t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
	t.Chdir(t.TempDir())
	t.Setenv("DOWNSHIFT_WORKERS", "off")
	var out, errb bytes.Buffer
	if rc := runGateway([]string{"on", "9router", "--endpoint", "http://127.0.0.1:1"}, &out, &errb); rc != 0 {
		t.Fatalf("rc=%d %s", rc, errb.String())
	}
	if !strings.Contains(errb.String(), gatewayIdleMessage) || !strings.Contains(errb.String(), workersFixSetup) {
		t.Errorf("stderr = %q", errb.String())
	}
	errb.Reset()
	t.Setenv("DOWNSHIFT_WORKERS", "read")
	if rc := runGateway([]string{"on", "9router", "--endpoint", "http://127.0.0.1:1"}, &out, &errb); rc != 0 || strings.Contains(errb.String(), gatewayIdleMessage) {
		t.Errorf("workers on: rc=%d stderr=%q", rc, errb.String())
	}
}
