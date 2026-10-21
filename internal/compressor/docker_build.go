package compressor

import (
	"bytes"
	"regexp"
	"unicode/utf8"
)

// Docker build output patterns
var (
	reDockerBuildStep    = regexp.MustCompile(`(?m)^(Step \d+/\d+|#\d+) `)
	reDockerBuildFrom    = regexp.MustCompile(`(?mi)^(Step \d+/\d+|#\d+) : FROM`)
	reDockerBuildRun     = regexp.MustCompile(`(?mi)^(Step \d+/\d+|#\d+) : RUN`)
	reDockerBuildCopy    = regexp.MustCompile(`(?mi)^(Step \d+/\d+|#\d+) : (COPY|ADD)`)
	reDockerBuildCached  = regexp.MustCompile(`(?i)(CACHED|\-\-\-> Using cache)`)
	reDockerBuildSuccess = regexp.MustCompile(`(?m)^Successfully (built|tagged)`)
	reDockerBuildError   = regexp.MustCompile(`(?mi)^(error|failed|ERROR|FAILED)`)
	reDockerBuildHash    = regexp.MustCompile(`\-\-\-> [a-f0-9]{12}`)
	reDockerBuildPush    = regexp.MustCompile(`(?m)^(Pushing|The push refers)`)
	reDockerBuildLayer   = regexp.MustCompile(`(?m)^[a-f0-9]{12}: (Pushed|Waiting|Preparing|Layer already exists)`)

	// Buildx / BuildKit patterns
	reBuildKit       = regexp.MustCompile(`(?m)^\[[\w/\s]+\]`)
	reBuildKitExport = regexp.MustCompile(`(?m)^exporting to`)
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

	// Preserve errors fully
	if reDockerBuildError.Match(input) {
		return compressDockerBuildWithErrors(input)
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

func compressDockerBuildWithErrors(input []byte) ([]byte, bool, string) {
	lines := bytes.Split(input, []byte("\n"))
	var out bytes.Buffer

	out.WriteString("docker build: FAILED\n\n")

	// Find the error section and a few lines before it
	var errorStart int = -1
	for i, line := range lines {
		if reDockerBuildError.Match(line) {
			errorStart = i
			break
		}
	}

	if errorStart == -1 {
		return input, false, "docker_build_error_not_found"
	}

	// Include context: the step that failed and error lines
	contextStart := errorStart - 5
	if contextStart < 0 {
		contextStart = 0
	}

	// Find the step that failed
	var failedStep []byte
	for i := errorStart; i >= 0; i-- {
		if reDockerBuildStep.Match(lines[i]) {
			failedStep = lines[i]
			break
		}
	}

	if failedStep != nil {
		out.WriteString("Failed at: ")
		out.Write(bytes.TrimSpace(failedStep))
		out.WriteString("\n\n")
	}

	// Include error context (up to 20 lines)
	out.WriteString("Error output:\n")
	contextEnd := errorStart + 15
	if contextEnd > len(lines) {
		contextEnd = len(lines)
	}

	for i := contextStart; i < contextEnd; i++ {
		line := lines[i]
		if len(line) > 0 {
			out.WriteString("  ")
			out.Write(truncateBytes(line, 100))
			out.WriteString("\n")
		}
	}

	result := out.Bytes()
	if len(result) >= len(input) {
		return input, false, "docker_build_error_no_gain"
	}

	return result, true, "docker_build_error_compacted"
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
