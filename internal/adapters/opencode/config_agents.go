// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package opencode

import (
	"bufio"
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// agentSubdirs are the folders of a config dir that hold markdown files with
// a permission frontmatter: agents and modes.
var agentSubdirs = []string{"agent", "agents", "mode", "modes"}

// agentDirsRestrictive reports whether an agent or mode markdown file under a
// config dir declares a permission in its frontmatter. Anything it cannot
// vouch for counts as restrictive: an unreadable or oversized file, and a
// link (the folder itself, a folder or a file inside it).
func agentDirsRestrictive(configDir string) bool {
	for _, sub := range agentSubdirs {
		root := filepath.Join(configDir, sub)
		if st, err := os.Lstat(root); err == nil && st.Mode()&os.ModeSymlink != 0 {
			return true
		}
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.Type()&fs.ModeSymlink != 0 {
				return errPermissionFound
			}
			if d.IsDir() || !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
				return nil
			}
			raw, err := readBounded(path)
			if err != nil {
				return err
			}
			if frontmatterHasPermission(raw) {
				return errPermissionFound
			}
			return nil
		})
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return true
		}
	}
	return false
}

var errPermissionFound = errors.New("agent declares a permission")

// frontmatterHasPermission scans the YAML frontmatter between the leading
// "---" lines for a top-level permission key.
func frontmatterHasPermission(raw []byte) bool {
	sc := bufio.NewScanner(bytes.NewReader(bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf"))))
	sc.Buffer(nil, 1<<20)
	if !sc.Scan() {
		return false
	}
	// A first line that opens with "---" but is not exactly that (gray-matter
	// also accepts a language tag such as "---yaml") is frontmatter this scan
	// does not parse: doubt.
	first := strings.TrimRight(sc.Text(), " \t\r")
	if strings.HasPrefix(first, "---") && first != "---" {
		return true
	}
	if first != "---" {
		return false
	}
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), " \t\r")
		if line == "---" || line == "..." {
			return false
		}
		name, _, found := strings.Cut(line, ":")
		if !found || name != strings.TrimLeft(name, " \t") {
			continue
		}
		if key := strings.Trim(strings.TrimRight(name, " \t"), "\"'"); key == "permission" || key == "permissions" {
			return true
		}
	}
	// An unterminated frontmatter or a scan error: doubt.
	return true
}
