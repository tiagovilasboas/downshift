// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestUsageShortIsShortAndPointsToAll(t *testing.T) {
	var short, full bytes.Buffer
	usageShort(&short)
	usage(&full)
	if short.Len() >= full.Len()/2 {
		t.Errorf("short help (%d bytes) should be well under the full one (%d)", short.Len(), full.Len())
	}
	if !strings.Contains(short.String(), "help --all") {
		t.Error("short help must point to help --all")
	}
	for _, cmd := range []string{"doctor", "try", "stats"} {
		if !strings.Contains(short.String(), "downshift "+cmd) {
			t.Errorf("short help is missing %q", cmd)
		}
	}
	for _, maint := range []string{"eval-adapt", "train", "shadow-report", "context "} {
		if strings.Contains(short.String(), maint) {
			t.Errorf("short help must not list maintainer command %q", maint)
		}
	}
}

func TestWantsAllHelp(t *testing.T) {
	for _, c := range []struct {
		args []string
		want bool
	}{{nil, false}, {[]string{"--all"}, true}, {[]string{"-a"}, true}, {[]string{"foo"}, false}} {
		if got := wantsAllHelp(c.args); got != c.want {
			t.Errorf("%v: got %v", c.args, got)
		}
	}
}
