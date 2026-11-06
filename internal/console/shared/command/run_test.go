// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
)

// helperEnvVar gates TestHelperEchoEnv so it only runs as a child process.
const helperEnvVar = "DOWNSHIFT_TEST_HELPER_ECHO_ENV"

// TestHelperEchoEnv is not a real test: the parent re-runs this test binary
// with helperEnvVar set, and the child prints the env it was started with.
func TestHelperEchoEnv(t *testing.T) {
	if os.Getenv(helperEnvVar) != "1" {
		return
	}
	fmt.Fprintf(os.Stdout, "PLAIN=%s NO_COLOR=%s\n", os.Getenv("DOWNSHIFT_PLAIN"), os.Getenv("NO_COLOR"))
}

func helperArgs() []string {
	return []string{"-test.run=^TestHelperEchoEnv$"}
}

func TestCapturePassesPlainModeToChild(t *testing.T) {
	t.Setenv(helperEnvVar, "1")
	t.Setenv("DOWNSHIFT_PLAIN", "0")
	res := Runner{Exe: os.Args[0]}.Capture(context.Background(), helperArgs())
	if !res.OK() {
		t.Fatalf("child failed: code=%d err=%v stderr=%s", res.Code, res.Err, res.Stderr)
	}
	if !strings.Contains(res.Stdout, "PLAIN=1 NO_COLOR=1") {
		t.Errorf("child env = %q, want PLAIN=1 NO_COLOR=1", res.Stdout)
	}
}

func TestInteractivePassesPlainModeToChild(t *testing.T) {
	t.Setenv(helperEnvVar, "1")
	t.Setenv("DOWNSHIFT_PLAIN", "0")
	var out bytes.Buffer
	cmd := Runner{Exe: os.Args[0]}.Interactive(helperArgs())
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("child failed: %v", err)
	}
	if !strings.Contains(out.String(), "PLAIN=1") {
		t.Errorf("child env = %q, want PLAIN=1", out.String())
	}
}

func TestCaptureCancelledContextReportsCanceled(t *testing.T) {
	t.Setenv(helperEnvVar, "1")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	res := Runner{Exe: os.Args[0]}.Capture(ctx, helperArgs())
	if res.OK() || !errors.Is(res.Err, context.Canceled) {
		t.Errorf("res = %+v, want Err context.Canceled", res)
	}
}
