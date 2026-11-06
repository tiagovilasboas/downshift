// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRunGateway_Help(t *testing.T) {
	for _, arg := range []string{"-h", "--help", "help"} {
		var stdout, stderr bytes.Buffer
		code := runGateway([]string{arg}, &stdout, &stderr)
		if code != 0 {
			t.Errorf("arg %q: want 0, got %d", arg, code)
		}
		if !strings.Contains(stdout.String(), "usage: downshift gateway") {
			t.Errorf("arg %q: missing usage in output", arg)
		}
	}
}

func TestRunGateway_InvalidArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runGateway([]string{"unknown-subcommand"}, &stdout, &stderr)
	if code != 2 {
		t.Errorf("want 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "usage: downshift gateway") {
		t.Errorf("missing usage in stderr")
	}
}

func TestRunGateway_Models(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runGateway([]string{"models"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("want 0, got %d", code)
	}
	out := stdout.String()
	if !strings.Contains(out, "9router") {
		t.Errorf("missing gateway mentions in models output: %s", out)
	}
	if !strings.Contains(out, "9router/<provider>/<model-id>") {
		t.Errorf("missing 9router prefix in models output")
	}
	if strings.Contains(strings.ToLower(out), "omni") {
		t.Errorf("models output must not mention other gateways: %s", out)
	}

	// Extra args should be rejected
	stdout.Reset()
	stderr.Reset()
	code = runGateway([]string{"models", "extra"}, &stdout, &stderr)
	if code != 2 {
		t.Errorf("want 2 for extra args, got %d", code)
	}
}

func TestRunGateway_StatusOutput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runGateway([]string{"status"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("want 0, got %d", code)
	}
	out := stdout.String()
	if !strings.Contains(out, "Gateway Companion Routers") {
		t.Errorf("missing header in status output: %s", out)
	}
	if !strings.Contains(out, "9router:") {
		t.Errorf("missing gateway names in status output: %s", out)
	}
	if !strings.Contains(out, "ANTHROPIC_BASE_URL=") {
		t.Errorf("missing env hint in status output: %s", out)
	}
}

func TestRunGateway_StatusJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runGateway([]string{"--json"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("want 0, got %d", code)
	}

	var reps []gatewayStatusReport
	if err := json.Unmarshal(stdout.Bytes(), &reps); err != nil {
		t.Fatalf("failed to decode json: %v, raw: %s", err, stdout.String())
	}
	if len(reps) != 1 || reps[0].Name != "9router" {
		t.Fatalf("expected only a 9router report, got %+v", reps)
	}
}
