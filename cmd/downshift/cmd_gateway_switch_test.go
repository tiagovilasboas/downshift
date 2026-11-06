// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/gateway"
)

func TestGatewayOnOffStatus(t *testing.T) {
	t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
	var out, errb bytes.Buffer
	if rc := runGateway([]string{"on", "9router", "--endpoint", "http://127.0.0.1:1"}, &out, &errb); rc != 0 {
		t.Fatalf("on rc=%d %s", rc, errb.String())
	}
	if !strings.Contains(errb.String(), "not reachable") || !strings.Contains(out.String(), "routed through 9router") {
		t.Errorf("on output: %q / %q", out.String(), errb.String())
	}
	if got := gatewayWorkerEnv("claude"); !reflect.DeepEqual(got, []string{"ANTHROPIC_BASE_URL=http://127.0.0.1:1"}) {
		t.Errorf("claude env = %v", got)
	}
	if got := gatewayWorkerEnv("codex"); !reflect.DeepEqual(got, []string{"OPENAI_BASE_URL=http://127.0.0.1:1/v1"}) {
		t.Errorf("codex env = %v", got)
	}
	out.Reset()
	if rc := runGateway([]string{"status"}, &out, &errb); rc != 0 || !strings.Contains(out.String(), "routed through 9router") {
		t.Errorf("status rc=%d out=%q", rc, out.String())
	}
	out.Reset()
	if rc := runGateway([]string{"off"}, &out, &errb); rc != 0 || !strings.Contains(out.String(), "gateway off") {
		t.Errorf("off rc=%d out=%q", rc, out.String())
	}
	if got := gatewayWorkerEnv("claude"); got != nil {
		t.Errorf("env after off = %v", got)
	}
}

func TestGatewayOnRejectsUnknownName(t *testing.T) {
	t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
	var out, errb bytes.Buffer
	if rc := runGateway([]string{"on", "nope"}, &out, &errb); rc != 2 {
		t.Errorf("rc=%d, want 2", rc)
	}
}

func TestGatewayStateLineRetired(t *testing.T) {
	line := gatewayStateLine(gateway.State{Name: "omniroute", Retired: true})
	if !strings.Contains(line, "gateway off") || !strings.Contains(line, "downshift gateway on 9router") {
		t.Fatalf("line = %q", line)
	}
}
