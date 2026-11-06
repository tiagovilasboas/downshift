// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"path"
	"regexp"
	"strings"
)

// Lockfile line grammars. A diff of a lockfile is read as a stream of package
// version events: (name, version, sign). Whether a package was added, removed
// or changed is decided from the events of a whole file section, not from a
// single line, because git often shows a package's key line only as context.

// lockfileNames are the basenames whose diffs the lockfile compressor reads.
var lockfileNames = map[string]bool{
	"package-lock.json": true, "yarn.lock": true, "pnpm-lock.yaml": true, "Cargo.lock": true,
}

var (
	reNpmLockKey     = regexp.MustCompile(`^\s*"([^"]*)": \{$`)
	reNpmLockVersion = regexp.MustCompile(`^\s*"version": "([^"]+)",?$`)
	reYarnLockKey    = regexp.MustCompile(`^[^\s#][^\n]*:$`)
	reYarnLockVer    = regexp.MustCompile(`^  version:? "?([^"\s]+)"?$`)
	rePnpmLockKey    = regexp.MustCompile(`^  '?/?((?:@[^/@\s']+/)?[^@/\s']+)@([^:'\s(]+)[^:']*'?:$`)
	rePnpmLockKeyOld = regexp.MustCompile(`^  /((?:@[^/\s]+/)?[^/\s]+)/([0-9][^:/(_\s]*)[^:]*:$`)
	reCargoLockName  = regexp.MustCompile(`^name = "([^"]+)"$`)
	reCargoLockVer   = regexp.MustCompile(`^version = "([^"]+)"$`)
)

// npmLockStructural are JSON keys of package-lock.json that open an object but
// are not packages.
var npmLockStructural = map[string]bool{
	"dependencies": true, "devDependencies": true, "peerDependencies": true,
	"optionalDependencies": true, "packages": true, "engines": true, "bin": true,
	"funding": true, "requires": true, "peerDependenciesMeta": true, "overrides": true,
	"scripts": true,
}

// lockEvent is one package version seen on an added or removed line.
type lockEvent struct {
	name, version string
	sign          byte // '+' or '-'
}

// lockScan is what one file section of a lockfile diff yields.
type lockScan struct {
	events       []lockEvent
	unattributed int  // version lines whose package the hunk does not show
	unsafe       bool // a line outside the known grammar carries a danger word or secret
}

// isLockfilePath reports whether a path names a lockfile the compressor reads.
func isLockfilePath(p string) bool { return lockfileNames[path.Base(p)] }

// scanLockfile reads the hunk lines of one file section (lines after the
// diff header, hunk headers included) for the lockfile named base.
func scanLockfile(base string, lines []string) lockScan {
	var s lockScan
	name, haveName := "", false
	for _, line := range lines {
		if strings.HasPrefix(line, "@@") {
			name, haveName = "", false
			continue
		}
		if line == "" || line[0] == '\\' {
			continue
		}
		sign, content := line[0], line[1:]
		known := false
		switch base {
		case "package-lock.json":
			name, haveName, known = npmLockLine(s.add, sign, content, name, haveName)
		case "yarn.lock":
			name, haveName, known = yarnLockLine(s.add, sign, content, name, haveName)
		case "pnpm-lock.yaml":
			known = pnpmLockLine(&s, sign, content)
		case "Cargo.lock":
			name, haveName, known = cargoLockLine(s.add, sign, content, name, haveName)
		}
		if !known && lockLineUnsafe(content) {
			s.unsafe = true
		}
	}
	return s
}

// add records a version event, or counts it unattributed when no package
// name is known for it. It is bound as a method value by the line readers.
func (s *lockScan) add(sign byte, name string, haveName bool, version string) {
	if sign != '+' && sign != '-' {
		return
	}
	if !haveName {
		s.unattributed++
		return
	}
	s.events = append(s.events, lockEvent{name, version, sign})
}

func npmLockLine(add func(byte, string, bool, string), sign byte, c, name string, have bool) (string, bool, bool) {
	if m := reNpmLockKey.FindStringSubmatch(c); m != nil {
		key := m[1]
		if i := strings.LastIndex(key, "node_modules/"); i >= 0 {
			return key[i+len("node_modules/"):], true, true
		}
		if key == "" {
			return "(root)", true, true
		}
		if !npmLockStructural[key] {
			return key, true, true
		}
		return name, have, false
	}
	if m := reNpmLockVersion.FindStringSubmatch(c); m != nil {
		add(sign, name, have, m[1])
		return name, have, true
	}
	return name, have, false
}

func yarnLockLine(add func(byte, string, bool, string), sign byte, c, name string, have bool) (string, bool, bool) {
	if reYarnLockKey.MatchString(c) && !strings.HasPrefix(c, "__metadata") {
		spec := strings.Trim(strings.SplitN(strings.TrimSuffix(c, ":"), ",", 2)[0], `" `)
		if i := strings.LastIndex(spec, "@"); i > 0 {
			return spec[:i], true, true
		}
		return name, have, false
	}
	if m := reYarnLockVer.FindStringSubmatch(c); m != nil {
		add(sign, name, have, m[1])
		return name, have, true
	}
	return name, have, false
}

func pnpmLockLine(s *lockScan, sign byte, c string) bool {
	m := rePnpmLockKey.FindStringSubmatch(c)
	if m == nil {
		m = rePnpmLockKeyOld.FindStringSubmatch(c)
	}
	if m == nil {
		return false
	}
	s.add(sign, m[1], true, m[2])
	return true
}

func cargoLockLine(add func(byte, string, bool, string), sign byte, c, name string, have bool) (string, bool, bool) {
	if c == "[[package]]" {
		return "", false, true
	}
	if m := reCargoLockName.FindStringSubmatch(c); m != nil {
		return m[1], true, true
	}
	if m := reCargoLockVer.FindStringSubmatch(c); m != nil {
		add(sign, name, have, m[1])
		return name, have, true
	}
	return name, have, false
}

// lockLineUnsafe reports whether a line outside the package grammar carries a
// word that makes cutting it unsafe: a danger word, a deprecation notice or a
// secret (a registry URL can embed a token). Package names are data and sit on
// the known key and version lines, which this is never asked about.
func lockLineUnsafe(content string) bool {
	return reDangerWord.MatchString(content) || reGitDiffSecret.MatchString(content) ||
		strings.Contains(strings.ToLower(content), "deprecat")
}
