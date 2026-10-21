// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"strings"
	"testing"
)

func TestDetectDocker(t *testing.T) {
	ps := "CONTAINER ID   IMAGE          COMMAND       CREATED       STATUS         PORTS                    NAMES\n" +
		"abc123def456   nginx:latest   \"/docker\"     2 hours ago   Up 2 hours     0.0.0.0:80->80/tcp       web\n"
	if f, fail := detectFormat([]byte(ps)); f != FormatDockerPs || fail {
		t.Fatalf("docker ps: format=%q fail=%v", f, fail)
	}

	img := "REPOSITORY   TAG       IMAGE ID       CREATED       SIZE\n" +
		"nginx        latest    abc123def456   2 weeks ago   187MB\n"
	if f, fail := detectFormat([]byte(img)); f != FormatDockerImages || fail {
		t.Fatalf("docker images: format=%q fail=%v", f, fail)
	}
}

func TestCompressDockerPs(t *testing.T) {
	in := "CONTAINER ID   IMAGE          COMMAND       CREATED       STATUS         PORTS                    NAMES\n" +
		"abc123def456   nginx:latest   \"/docker\"     2 hours ago   Up 2 hours     0.0.0.0:80->80/tcp       web\n" +
		"def456abc789   redis:7        \"redis\"       3 hours ago   Up 3 hours     6379/tcp                 cache\n"
	out, applied, reason := compressDockerPs([]byte(in))
	if !applied || reason != "docker_ps_compacted" {
		t.Fatalf("applied=%v reason=%q", applied, reason)
	}
	s := string(out)
	for _, want := range []string{"NAME\tIMAGE\tSTATUS\tPORTS", "web\tnginx:latest\tUp 2 hours", "cache\tredis:7\tUp 3 hours"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
	// COMMAND, CREATED, CONTAINER ID and IMAGE ID columns are dropped.
	if strings.Contains(s, "COMMAND") || strings.Contains(s, "CREATED") || strings.Contains(s, "CONTAINER ID") {
		t.Errorf("non-essential column leaked:\n%s", s)
	}
}

func TestCompressDockerImages(t *testing.T) {
	in := "REPOSITORY   TAG       IMAGE ID       CREATED       SIZE\n" +
		"nginx        latest    abc123def456   2 weeks ago   187MB\n" +
		"redis        7         def456abc789   3 weeks ago   117MB\n"
	out, applied, reason := compressDockerImages([]byte(in))
	if !applied || reason != "docker_images_compacted" {
		t.Fatalf("applied=%v reason=%q", applied, reason)
	}
	s := string(out)
	for _, want := range []string{"REPOSITORY\tTAG\tSIZE", "nginx\tlatest\t187MB", "redis\t7\t117MB"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
	if strings.Contains(s, "IMAGE ID") || strings.Contains(s, "2 weeks ago") {
		t.Errorf("non-essential column leaked:\n%s", s)
	}
}

func TestCompressDockerLogsDedup(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 8; i++ {
		b.WriteString("2026-10-21 connection retry failed\n")
	}
	b.WriteString("2026-10-21 connected\n")
	out, applied, reason := compressDockerLogs([]byte(b.String()))
	if !applied || reason != "docker_logs_deduplicated" {
		t.Fatalf("applied=%v reason=%q", applied, reason)
	}
	s := string(out)
	if !strings.Contains(s, "2026-10-21 connection retry failed (×8)") {
		t.Errorf("missing deduplicated count:\n%s", s)
	}
	if !strings.Contains(s, "2026-10-21 connected") {
		t.Errorf("dropped unique trailing line:\n%s", s)
	}
}

func TestCompressDockerLogsNoDuplicates(t *testing.T) {
	in := "line a\nline b\nline c\nline d\nline e\nline f\n"
	out, applied, reason := compressDockerLogs([]byte(in))
	if applied || reason != "docker_logs_no_duplicates" {
		t.Fatalf("applied=%v reason=%q", applied, reason)
	}
	if string(out) != in {
		t.Errorf("mutated unique logs:\n%s", string(out))
	}
}
