// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"bytes"
	"regexp"
	"strings"
)

// Docker output compressor.
// docker ps and docker images produce wide tables with many columns.
// RTK keeps only essential columns: name, image, status, ports for ps;
// repository, tag, size for images.

var (
	// docker ps header detection
	reDockerPsHeader = regexp.MustCompile(`^CONTAINER ID\s+IMAGE\s+COMMAND\s+CREATED\s+STATUS\s+PORTS\s+NAMES`)
	// docker images header detection
	reDockerImagesHeader = regexp.MustCompile(`^REPOSITORY\s+TAG\s+IMAGE ID\s+CREATED\s+SIZE`)
)

// isDockerPs detects docker ps output.
func isDockerPs(input []byte) bool {
	return reDockerPsHeader.Match(input)
}

// isDockerImages detects docker images output.
func isDockerImages(input []byte) bool {
	return reDockerImagesHeader.Match(input)
}

// compressDockerPs compacts docker ps to essential columns: NAME, IMAGE, STATUS, PORTS
func compressDockerPs(input []byte) ([]byte, bool, string) {
	lines := bytes.Split(input, []byte("\n"))
	if len(lines) < 1 {
		return input, false, "docker_ps_no_content"
	}

	// Parse header to find column positions
	header := string(lines[0])
	if !reDockerPsHeader.MatchString(header) {
		return input, false, "docker_ps_header_not_found"
	}

	// Find column positions
	colImage := strings.Index(header, "IMAGE")
	colStatus := strings.Index(header, "STATUS")
	colPorts := strings.Index(header, "PORTS")
	colNames := strings.Index(header, "NAMES")

	if colImage < 0 || colStatus < 0 || colNames < 0 {
		return input, false, "docker_ps_columns_not_found"
	}

	var out bytes.Buffer
	out.WriteString("NAME\tIMAGE\tSTATUS\tPORTS\n")

	for _, line := range lines[1:] {
		if len(line) == 0 {
			continue
		}
		lineStr := string(line)
		if len(lineStr) < colNames {
			continue
		}

		// Extract columns (handling variable width)
		var image, status, ports, name string

		if colImage >= 0 && len(lineStr) > colImage {
			end := colStatus
			if end < 0 || end > len(lineStr) {
				end = len(lineStr)
			}
			image = strings.TrimSpace(lineStr[colImage:min(end, len(lineStr))])
			// Take just the image name, not the full field
			if spaceIdx := strings.Index(image, " "); spaceIdx > 0 {
				image = image[:spaceIdx]
			}
		}

		if colStatus >= 0 && len(lineStr) > colStatus {
			end := colPorts
			if end < 0 || end > len(lineStr) {
				end = len(lineStr)
			}
			status = strings.TrimSpace(lineStr[colStatus:min(end, len(lineStr))])
		}

		if colPorts >= 0 && colPorts < len(lineStr) {
			end := colNames
			if end < 0 || end > len(lineStr) {
				end = len(lineStr)
			}
			ports = strings.TrimSpace(lineStr[colPorts:min(end, len(lineStr))])
		}

		if colNames >= 0 && colNames < len(lineStr) {
			name = strings.TrimSpace(lineStr[colNames:])
		}

		// Only include if we got a name
		if name != "" {
			out.WriteString(name)
			out.WriteString("\t")
			out.WriteString(image)
			out.WriteString("\t")
			out.WriteString(status)
			out.WriteString("\t")
			out.WriteString(ports)
			out.WriteString("\n")
		}
	}

	return out.Bytes(), true, "docker_ps_compacted"
}

// compressDockerImages compacts docker images to: REPOSITORY, TAG, SIZE
func compressDockerImages(input []byte) ([]byte, bool, string) {
	lines := bytes.Split(input, []byte("\n"))
	if len(lines) < 1 {
		return input, false, "docker_images_no_content"
	}

	header := string(lines[0])
	if !reDockerImagesHeader.MatchString(header) {
		return input, false, "docker_images_header_not_found"
	}

	// Find column positions
	colTag := strings.Index(header, "TAG")
	colImageID := strings.Index(header, "IMAGE ID")
	colSize := strings.Index(header, "SIZE")

	if colTag < 0 || colSize < 0 {
		return input, false, "docker_images_columns_not_found"
	}

	var out bytes.Buffer
	out.WriteString("REPOSITORY\tTAG\tSIZE\n")

	for _, line := range lines[1:] {
		if len(line) == 0 {
			continue
		}
		lineStr := string(line)
		if len(lineStr) < colTag {
			continue
		}

		// Extract columns
		repo := strings.TrimSpace(lineStr[:colTag])

		var tag, size string
		if colTag < len(lineStr) {
			end := colImageID
			if end < 0 || end > len(lineStr) {
				end = len(lineStr)
			}
			tag = strings.TrimSpace(lineStr[colTag:min(end, len(lineStr))])
		}

		if colSize >= 0 && colSize < len(lineStr) {
			size = strings.TrimSpace(lineStr[colSize:])
		}

		if repo != "" {
			out.WriteString(repo)
			out.WriteString("\t")
			out.WriteString(tag)
			out.WriteString("\t")
			out.WriteString(size)
			out.WriteString("\n")
		}
	}

	return out.Bytes(), true, "docker_images_compacted"
}

// compressDockerLogs deduplicates repeated consecutive log lines, appending a
// (×N) suffix. It mirrors compressKubectlLogs and only applies when it actually
// removes at least a few duplicate lines, so unique logs are left untouched.
func compressDockerLogs(input []byte) ([]byte, bool, string) {
	lines := bytes.Split(input, []byte("\n"))
	if len(lines) < 5 {
		return input, false, "docker_logs_too_short"
	}

	var out bytes.Buffer
	var lastLine []byte
	hasLast := false
	repeatCount := 0
	compactedLines := 0

	flush := func() {
		if !hasLast {
			return
		}
		out.Write(lastLine)
		if repeatCount > 1 {
			out.WriteString(" (×")
			out.WriteString(itoa(repeatCount))
			out.WriteString(")")
			compactedLines += repeatCount - 1
		}
		out.WriteString("\n")
	}

	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		if hasLast && bytes.Equal(line, lastLine) {
			repeatCount++
			continue
		}
		flush()
		lastLine = line
		hasLast = true
		repeatCount = 1
	}
	flush()

	if compactedLines < 3 {
		return input, false, "docker_logs_no_duplicates"
	}

	return out.Bytes(), true, "docker_logs_deduplicated"
}
