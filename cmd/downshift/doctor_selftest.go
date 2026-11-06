// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/tiagovilasboas/downshift/internal/execwrap"
)

// These probes exercise components in isolation, not the host application's hooks.
func runDoctorSelfTest(rep *doctorReport, env doctorEnv) {
	add := func(id string, ok bool, detail string) {
		status := doctorOK
		if !ok {
			status = doctorFail
		}
		rep.add(doctorCheck{ID: id, Harness: env.Harness, Status: status, Detail: detail})
	}
	if err := selfTestCompactor(); err != nil {
		add("self-test compactor", false, err.Error())
	} else {
		add("self-test compactor", true, "isolated success and error exit status verified")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	bin, err := env.Executable()
	if err == nil {
		var result mcpProbeResult
		result, err = probeMcpTools(ctx, bin)
		if err == nil {
			err = result.CallErr
		}
	}
	if err != nil {
		add("self-test mcp", false, err.Error())
	} else {
		add("self-test mcp", true, "this binary answered a local MCP tool call; host connection not tested")
	}
	smoke := runSmokeCheck(env.Harness, env.Cat)
	if smoke.Skipped {
		rep.add(doctorCheck{ID: "self-test hook", Harness: env.Harness, Status: doctorWarn, Detail: "synthetic hook probe unavailable; host execution not tested"})
	} else {
		detail := smoke.Detail
		if smoke.Decided {
			detail = "decision recorded"
		}
		add("self-test hook", smoke.Decided, "synthetic hook probe: "+detail+"; host execution not tested")
	}
}

func selfTestCompactor() error {
	for _, args := range [][]string{{"git", "config", "--list"}, {"git", "config", "--get"}} {
		cmd := exec.Command(args[0], args[1:]...)
		direct, err := cmd.CombinedOutput()
		want := 0
		if err != nil {
			if e, ok := err.(*exec.ExitError); ok {
				want = e.ExitCode()
			} else {
				return err
			}
		}
		var out bytes.Buffer
		result := execwrap.Run(args, execwrap.Options{Stdout: &out, Stderr: &out, Compact: true, SkipObservation: true})
		if (args[2] == "--list" && want != 0) || (args[2] == "--get" && (want == 0 || len(direct) == 0)) || result.ExitCode != want || (want != 0 && !bytes.Equal(out.Bytes(), direct)) {
			return fmt.Errorf("exit or failure output differs for git config (expected %d, got %d)", want, result.ExitCode)
		}
	}
	return nil
}
