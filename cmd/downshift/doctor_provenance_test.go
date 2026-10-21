// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"runtime/debug"
	"testing"
)

func TestBuildProvenanceCheck(t *testing.T) {
	const rev = "0123456789abcdef0123456789abcdef01234567"
	cases := []struct {
		name       string
		info       *debug.BuildInfo
		ok         bool
		wantStatus string
		wantDetail string
	}{
		{
			name: "dirty tree warns",
			info: &debug.BuildInfo{Settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: rev}, {Key: "vcs.modified", Value: "true"}}},
			ok:         true,
			wantStatus: doctorWarn,
			wantDetail: "built from a dirty tree",
		},
		{
			name: "clean tree with revision is ok with short commit",
			info: &debug.BuildInfo{Settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: rev}, {Key: "vcs.modified", Value: "false"}}},
			ok:         true,
			wantStatus: doctorOK,
			wantDetail: "0123456",
		},
		{
			name:       "no vcs data is unknown and never warns",
			info:       &debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "-ldflags", Value: "-X main.x=y"}}},
			ok:         true,
			wantStatus: doctorOK,
			wantDetail: "unknown",
		},
		{
			name:       "no build info at all is unknown",
			info:       nil,
			ok:         false,
			wantStatus: doctorOK,
			wantDetail: "unknown",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := doctorEnv{BuildInfo: func() (*debug.BuildInfo, bool) { return tc.info, tc.ok }}
			c := buildProvenanceCheck(env)
			if c.ID != "build provenance" {
				t.Errorf("ID = %q", c.ID)
			}
			wantStatus(t, c, tc.wantStatus)
			if c.Detail != tc.wantDetail {
				t.Errorf("Detail = %q, want %q", c.Detail, tc.wantDetail)
			}
			if tc.wantStatus == doctorWarn && c.Fix == "" {
				t.Error("a provenance warning must name a fix")
			}
		})
	}
}

// A dirty build is a warning only: the report gains no failure, so the exit
// code stays 0.
func TestBuildProvenanceDirtyIsNotAFailure(t *testing.T) {
	rep := &doctorReport{}
	rep.add(buildProvenanceCheck(doctorEnv{BuildInfo: func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "vcs.modified", Value: "true"}}}, true
	}}))
	if rep.Failed != 0 || rep.Warnings != 1 {
		t.Fatalf("Failed=%d Warnings=%d, want 0 and 1", rep.Failed, rep.Warnings)
	}
}
