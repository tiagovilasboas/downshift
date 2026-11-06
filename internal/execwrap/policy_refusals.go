// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"regexp"
	"strings"
)

// ════════════════════════════════════════════════════════════════════════════
// DOMAIN: argv shapes the compacted families refuse. A family is routed by its
// program and subcommand (policy_*.go); this table narrows it to the forms that
// only read, build or test, so a flag that writes a file, publishes, watches or
// opens a server is never wrapped. A refused argv is left as written.
// ════════════════════════════════════════════════════════════════════════════

// refusedFlags lists, per family, the flags that write outside the project's
// own build output, publish, or keep the command running.
var refusedFlags = map[Family][]string{
	// rspec --init writes .rspec; --out/-o write a report file.
	FamilyRspec: {"init", "out", "o"},
	// dotnet test --diag writes a log file; --results-directory a result tree.
	FamilyDotnetTest: {"diag", "d", "results-directory"},
	// phpstan writes a baseline or runs a watcher/server.
	FamilyPHPStan: {"generate-baseline", "watch", "fix", "pro"},
	// docker build --push publishes; --output exports to the host; the
	// secret, ssh and cache-to flags hand credentials or write a cache; --iidfile
	// and --metadata-file write a file.
	FamilyDockerBuild: {"push", "output", "o", "secret", "ssh", "cache-to", "iidfile", "metadata-file"},
	// cargo clippy --fix rewrites source files.
	FamilyCargoClippy: {"fix", "allow-dirty", "allow-staged", "allow-no-vcs"},
	// kubectl get --watch never exits, so the held output never reaches the model.
	FamilyKubectlGet: {"watch", "w", "watch-only"},
}

// refusedShape reports whether the argv of an allowlisted family has a shape
// the wrapper must not run. A family with no rule is never refused here.
func refusedShape(f Family, argv []string) bool {
	if names := refusedFlags[f]; len(names) > 0 && namesFlag(argv[1:], names...) {
		return true
	}
	switch f {
	case FamilyDockerBuild:
		return hasRemoteContext(argv[2:])
	case FamilyPHPStan:
		return !phpstanAnalyse(argv)
	case FamilyRubocop:
		return refusedRubocop(argv)
	}
	return false
}

// reScpAddress is an scp-style git address ("git@gitlab.com:org/repo").
var reScpAddress = regexp.MustCompile(`^[\w.-]+@[\w.-]+:`)

// remoteHostPrefixes are bare hosts the daemon treats as a git context.
var remoteHostPrefixes = []string{"github.com/", "gitlab.com/", "bitbucket.org/"}

// hasRemoteContext reports a build context fetched over the network (a URL, an
// scp-style address, a bare git host or a repository ending .git), which the
// Docker daemon would clone and run.
func hasRemoteContext(words []string) bool {
	for _, w := range words {
		if strings.Contains(w, "://") || reScpAddress.MatchString(w) ||
			strings.HasSuffix(w, ".git") || strings.Contains(w, ".git#") {
			return true
		}
		for _, h := range remoteHostPrefixes {
			if strings.HasPrefix(w, h) {
				return true
			}
		}
	}
	return false
}

// phpstanAnalyse accepts `phpstan` with options only, or `phpstan analyse`
// (analyze). Other subcommands (clear-result-cache, dump-deps, ...) write.
func phpstanAnalyse(argv []string) bool {
	for _, w := range argv[1:] {
		if strings.HasPrefix(w, "-") {
			continue
		}
		return w == "analyse" || w == "analyze"
	}
	return true
}
