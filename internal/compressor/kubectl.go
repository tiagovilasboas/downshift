// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"bytes"
	"regexp"
	"strings"
)

// Kubectl output compressor.
// kubectl get pods produces wide tables; RTK keeps only NAME, STATUS, RESTARTS.
// kubectl logs can be deduplicated for repeated entries.

var (
	// kubectl get pods header detection
	reKubectlPodsHeader = regexp.MustCompile(`^NAME\s+READY\s+STATUS\s+RESTARTS\s+AGE`)
)

// isKubectlGetPods detects kubectl get pods output.
func isKubectlGetPods(input []byte) bool {
	return reKubectlPodsHeader.Match(input)
}

// compressKubectlGetPods compacts to: NAME, STATUS, RESTARTS
func compressKubectlGetPods(input []byte) ([]byte, bool, string) {
	lines := bytes.Split(input, []byte("\n"))
	if len(lines) < 1 {
		return input, false, "kubectl_pods_no_content"
	}

	header := string(lines[0])
	if !reKubectlPodsHeader.MatchString(header) {
		return input, false, "kubectl_pods_header_not_found"
	}

	// Find column positions
	colReady := strings.Index(header, "READY")
	colStatus := strings.Index(header, "STATUS")
	colRestarts := strings.Index(header, "RESTARTS")
	colAge := strings.Index(header, "AGE")

	if colStatus < 0 || colRestarts < 0 {
		return input, false, "kubectl_pods_columns_not_found"
	}

	var out bytes.Buffer
	out.WriteString("NAME\tSTATUS\tRESTARTS\n")

	for _, line := range lines[1:] {
		if len(line) == 0 {
			continue
		}
		lineStr := string(line)

		// Extract name (everything before READY column)
		name := strings.TrimSpace(lineStr[:min(colReady, len(lineStr))])

		var status, restarts string

		// Extract status
		if colStatus < len(lineStr) {
			end := colRestarts
			if end > len(lineStr) {
				end = len(lineStr)
			}
			status = strings.TrimSpace(lineStr[colStatus:end])
		}

		// Extract restarts
		if colRestarts < len(lineStr) {
			end := colAge
			if end < 0 || end > len(lineStr) {
				end = len(lineStr)
			}
			restarts = strings.TrimSpace(lineStr[colRestarts:end])
			// Just the count, not the full field
			if spaceIdx := strings.Index(restarts, " "); spaceIdx > 0 {
				restarts = restarts[:spaceIdx]
			}
		}

		if name != "" {
			out.WriteString(name)
			out.WriteString("\t")
			out.WriteString(status)
			out.WriteString("\t")
			out.WriteString(restarts)
			out.WriteString("\n")
		}
	}

	return out.Bytes(), true, "kubectl_pods_compacted"
}

// compressKubectlLogs deduplicates repeated log lines.
// If the same line appears multiple times consecutively, show it once with a count.
func compressKubectlLogs(input []byte) ([]byte, bool, string) {
	lines := bytes.Split(input, []byte("\n"))
	if len(lines) < 5 {
		// Not worth compacting small outputs
		return input, false, "kubectl_logs_too_short"
	}

	var out bytes.Buffer
	var lastLine []byte
	repeatCount := 0
	totalLines := 0
	compactedLines := 0

	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		totalLines++

		if bytes.Equal(line, lastLine) {
			repeatCount++
			continue
		}

		// Flush previous repeated line
		if repeatCount > 0 {
			out.Write(lastLine)
			if repeatCount > 1 {
				out.WriteString(" (×")
				out.WriteString(itoa(repeatCount))
				out.WriteString(")")
				compactedLines += repeatCount - 1
			}
			out.WriteString("\n")
		}

		lastLine = line
		repeatCount = 1
	}

	// Flush last line
	if repeatCount > 0 {
		out.Write(lastLine)
		if repeatCount > 1 {
			out.WriteString(" (×")
			out.WriteString(itoa(repeatCount))
			out.WriteString(")")
			compactedLines += repeatCount - 1
		}
		out.WriteString("\n")
	}

	// Only compact if we actually deduplicated something
	if compactedLines < 3 {
		return input, false, "kubectl_logs_no_duplicates"
	}

	return out.Bytes(), true, "kubectl_logs_deduplicated"
}

// itoa is a simple int to string conversion without importing strconv
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
