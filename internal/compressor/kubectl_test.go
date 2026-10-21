// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"strings"
	"testing"
)

func TestDetectKubectlPods(t *testing.T) {
	pods := "NAME                     READY   STATUS    RESTARTS   AGE\n" +
		"web-7d9f8c-abc           1/1     Running   3          2d\n"
	if f, fail := detectFormat([]byte(pods)); f != FormatKubectlPods || fail {
		t.Fatalf("kubectl pods: format=%q fail=%v", f, fail)
	}
}

func TestCompressKubectlGetPods(t *testing.T) {
	in := "NAME                     READY   STATUS    RESTARTS   AGE\n" +
		"web-7d9f8c-abc           1/1     Running   3          2d\n" +
		"cache-5b6c7d-xyz         1/1     Running   0          5d\n"
	out, applied, reason := compressKubectlGetPods([]byte(in))
	if !applied || reason != "kubectl_pods_compacted" {
		t.Fatalf("applied=%v reason=%q", applied, reason)
	}
	s := string(out)
	for _, want := range []string{"NAME\tSTATUS\tRESTARTS", "web-7d9f8c-abc\tRunning\t3", "cache-5b6c7d-xyz\tRunning\t0"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
	// READY and AGE columns are dropped.
	if strings.Contains(s, "READY") || strings.Contains(s, "AGE") || strings.Contains(s, "1/1") {
		t.Errorf("non-essential column leaked:\n%s", s)
	}
}

func TestCompressKubectlLogsDedup(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 6; i++ {
		b.WriteString("level=info msg=\"reconciling\"\n")
	}
	b.WriteString("level=info msg=\"done\"\n")
	out, applied, reason := compressKubectlLogs([]byte(b.String()))
	if !applied || reason != "kubectl_logs_deduplicated" {
		t.Fatalf("applied=%v reason=%q", applied, reason)
	}
	s := string(out)
	if !strings.Contains(s, "level=info msg=\"reconciling\" (×6)") {
		t.Errorf("missing deduplicated count:\n%s", s)
	}
	if !strings.Contains(s, "level=info msg=\"done\"") {
		t.Errorf("dropped unique trailing line:\n%s", s)
	}
}

func TestCompressKubectlLogsNoDuplicates(t *testing.T) {
	in := "line a\nline b\nline c\nline d\nline e\nline f\n"
	out, applied, reason := compressKubectlLogs([]byte(in))
	if applied || reason != "kubectl_logs_no_duplicates" {
		t.Fatalf("applied=%v reason=%q", applied, reason)
	}
	if string(out) != in {
		t.Errorf("mutated unique logs:\n%s", string(out))
	}
}
