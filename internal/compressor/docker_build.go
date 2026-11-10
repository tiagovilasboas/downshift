// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"bytes"
	"unicode/utf8"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
)

// Docker build output patterns
var (
	reDockerBuildStep    = lazyre.New(`(?m)^(Step \d+/\d+|#\d+) `)
	reDockerBuildCached  = lazyre.New(`(?i)(CACHED|\-\-\-> Using cache)`)
	reDockerBuildSuccess = lazyre.New(`(?m)^Successfully (built|tagged)`)
	reDockerBuildError   = lazyre.New(`(?mi)^(error|failed|ERROR|FAILED)`)
	reDockerBuildPush    = lazyre.New(`(?m)^(Pushing|The push refers)`)
	reDockerBuildLayer   = lazyre.New(`(?m)^[a-f0-9]{12}: (Pushed|Waiting|Preparing|Layer already exists)`)

	// Buildx / BuildKit patterns
	// Any line that reports a problem, wherever it sits in the transcript.
	reDockerBuildDanger = lazyre.New(`(?i)error|fail|warn|panic|denied|deprecated`)

	reBuildKit = lazyre.New(`(?m)^\[[\w/\s]+\]`)
)

func isDockerBuild(input []byte) bool {
	if len(input) > MaxProcessableBytes || !utf8.Valid(input) {
		return false
	}
	// Need both a step pattern AND either success or a dockerfile reference
	hasStep := reDockerBuildStep.Match(input)
	hasBuildKit := reBuildKit.Match(input) && bytes.Contains(input, []byte("FROM"))
	hasSuccess := reDockerBuildSuccess.Match(input)
	hasDockerfile := bytes.Contains(input, []byte("Dockerfile")) || bytes.Contains(input, []byte("FROM "))

	return (hasStep && (hasSuccess || hasDockerfile)) || (hasBuildKit && hasDockerfile)
}

func compressDockerBuild(input []byte) ([]byte, bool, string) {
	if len(input) > MaxProcessableBytes || !utf8.Valid(input) {
		return input, false, "docker_build_too_large"
	}

	// Fail open: build output (a RUN step's own lines) that names an error, a
	// failure or a warning is kept whole, and so is any line that starts like an
	// error (a clean exit can still print a pip "ERROR: ..." resolver notice).
	if reDockerBuildError.Match(input) || reDockerBuildDanger.Match(input) {
		return input, false, "docker_build_danger_preserved"
	}

	lines := bytes.Split(input, []byte("\n"))
	var out bytes.Buffer

	var totalSteps int
	var cachedSteps int
	var successTags []string
	var lastStep []byte

	for _, line := range lines {
		if len(line) == 0 {
			continue
		}

		// Count steps
		if reDockerBuildStep.Match(line) {
			totalSteps++
			lastStep = line
		}

		// Count cached
		if reDockerBuildCached.Match(line) {
			cachedSteps++
		}

		// Capture success tags
		if reDockerBuildSuccess.Match(line) {
			successTags = append(successTags, string(bytes.TrimSpace(line)))
		}
	}

	out.WriteString("docker build:\n")

	if totalSteps > 0 {
		out.WriteString("  steps: ")
		out.WriteString(itoa(totalSteps))
		if cachedSteps > 0 {
			out.WriteString(" (")
			out.WriteString(itoa(cachedSteps))
			out.WriteString(" cached)")
		}
		out.WriteString("\n")
	}

	if lastStep != nil && totalSteps > 1 {
		out.WriteString("  last: ")
		out.Write(truncateBytes(lastStep, 60))
		out.WriteString("\n")
	}

	for _, tag := range successTags {
		out.WriteString("  ✓ ")
		out.WriteString(tag)
		out.WriteString("\n")
	}

	if len(successTags) == 0 && totalSteps > 0 {
		out.WriteString("  (build in progress or no tag)\n")
	}

	result := out.Bytes()
	if len(result) >= len(input) {
		return input, false, "docker_build_no_gain"
	}

	return result, true, "docker_build_compacted"
}

func isDockerPush(input []byte) bool {
	if len(input) > MaxProcessableBytes || !utf8.Valid(input) {
		return false
	}
	return reDockerBuildPush.Match(input) && reDockerBuildLayer.Match(input)
}

func compressDockerPush(input []byte) ([]byte, bool, string) {
	if len(input) > MaxProcessableBytes || !utf8.Valid(input) {
		return input, false, "docker_push_too_large"
	}

	lines := bytes.Split(input, []byte("\n"))
	var out bytes.Buffer

	var pushedLayers int
	var cachedLayers int
	var waitingLayers int
	var targetImage string

	for _, line := range lines {
		if bytes.HasPrefix(line, []byte("The push refers to")) {
			parts := bytes.Fields(line)
			if len(parts) >= 5 {
				targetImage = string(parts[4])
			}
		}

		if reDockerBuildLayer.Match(line) {
			if bytes.Contains(line, []byte("Pushed")) {
				pushedLayers++
			} else if bytes.Contains(line, []byte("Layer already exists")) {
				cachedLayers++
			} else if bytes.Contains(line, []byte("Waiting")) || bytes.Contains(line, []byte("Preparing")) {
				waitingLayers++
			}
		}
	}

	out.WriteString("docker push:\n")
	if targetImage != "" {
		out.WriteString("  image: ")
		out.WriteString(targetImage)
		out.WriteString("\n")
	}

	out.WriteString("  layers: ")
	out.WriteString(itoa(pushedLayers))
	out.WriteString(" pushed")
	if cachedLayers > 0 {
		out.WriteString(", ")
		out.WriteString(itoa(cachedLayers))
		out.WriteString(" cached")
	}
	if waitingLayers > 0 {
		out.WriteString(", ")
		out.WriteString(itoa(waitingLayers))
		out.WriteString(" pending")
	}
	out.WriteString("\n")

	result := out.Bytes()
	if len(result) >= len(input) {
		return input, false, "docker_push_no_gain"
	}

	return result, true, "docker_push_compacted"
}

func truncateBytes(b []byte, maxLen int) []byte {
	if len(b) <= maxLen {
		return bytes.TrimSpace(b)
	}
	return append(bytes.TrimSpace(b[:maxLen]), []byte("...")...)
}
