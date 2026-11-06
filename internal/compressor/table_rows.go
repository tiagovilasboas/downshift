// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"regexp"
	"strings"
)

// tableRowsRecognised reports whether every non-empty line after the header of
// a column table is a row of that table. A line that is not (an error message
// mixed into the listing, a truncated row) makes the whole output stay raw, so
// a compaction never hides it.
func tableRowsRecognised(lines [][]byte, isRow func(fields []string) bool) bool {
	for _, line := range lines[1:] {
		if len(line) == 0 {
			continue
		}
		if !isRow(strings.Fields(string(line))) {
			return false
		}
	}
	return true
}

var (
	reContainerID = regexp.MustCompile(`^(sha256:)?[0-9a-f]{12,64}$`)
	reReadyPair   = regexp.MustCompile(`^\d+/\d+$`)
)

// isDockerPsRow: the first field is a container id and the row has a status.
func isDockerPsRow(fields []string) bool {
	return len(fields) >= 6 && reContainerID.MatchString(fields[0])
}

// isDockerImagesRow: repository, tag, image id, then created and size.
func isDockerImagesRow(fields []string) bool {
	return len(fields) >= 5 && reContainerID.MatchString(fields[2])
}

// isKubectlPodRow: name, READY as n/m, status, restarts and age.
func isKubectlPodRow(fields []string) bool {
	return len(fields) >= 5 && reReadyPair.MatchString(fields[1])
}
