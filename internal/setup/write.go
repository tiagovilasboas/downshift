// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/tiagovilasboas/downshift-ai/internal/setup/configfile"
)

// Write backs up the existing file, then writes the new content
// atomically. It returns the backup path ("" for a new file; a marker file
// records that the file did not exist so --undo can remove it).
//
// The marker, and the backup of a file inside a folder the harness loads
// (Plan.Loaded), go to the state dir (<state>/backups/<stamp>/...), never next
// to the file: an empty marker in a plugin folder is loaded as a plugin. Other
// backups stay next to the file. Without Plan.Bases everything stays adjacent.
func Write(p Plan, stamp string) (string, error) {
	if st, err := os.Lstat(p.Path); err == nil && st.Mode()&os.ModeSymlink != 0 {
		// A linked file usually lives in a versioned repository: setup leaves it alone.
		return "", fmt.Errorf("%s is a link; setup leaves linked files alone (edit the real file by hand, or remove the link)", p.Path)
	}
	if len(p.Bases) > 0 {
		if err := ErrLinkedPath(p.Path, p.Bases...); err != nil {
			return "", err
		}
	}
	if err := os.MkdirAll(filepath.Dir(p.Path), 0o700); err != nil {
		return "", err
	}
	if p.Mode != 0 {
		// A generated script runs on every tool call. MkdirAll leaves a directory
		// that already exists as it was, so close it to everyone else here; if we
		// cannot (not ours), fail rather than put a script where others can swap it.
		if err := os.Chmod(filepath.Dir(p.Path), 0o700); err != nil {
			return "", fmt.Errorf("cannot restrict %s to its owner: %w", filepath.Dir(p.Path), err)
		}
	}
	mode := os.FileMode(0o600)
	bak := p.Path + BackupInfix + stamp
	if p.Existed {
		if st, err := os.Stat(p.Path); err == nil {
			mode = st.Mode().Perm()
		}
		if dest, ok := storeOf(p.Bases).location(p.Path, stamp, false); ok && p.Loaded {
			bak = dest
		}
		if err := os.MkdirAll(filepath.Dir(bak), 0o700); err != nil {
			return "", fmt.Errorf("backup: %w", err)
		}
		if err := configfile.WriteAtomic(bak, p.Before, mode); err != nil {
			return "", fmt.Errorf("backup: %w", err)
		}
	} else {
		marker := bak + ".new"
		if dest, ok := storeOf(p.Bases).location(p.Path, stamp, true); ok {
			marker = dest
		}
		if err := os.MkdirAll(filepath.Dir(marker), 0o700); err != nil {
			return "", fmt.Errorf("backup marker: %w", err)
		}
		if err := os.WriteFile(marker, nil, 0o600); err != nil {
			return "", fmt.Errorf("backup marker: %w", err)
		}
		bak = ""
	}
	if p.Mode != 0 {
		mode = p.Mode // a generated script must stay executable
	}
	if err := configfile.WriteAtomic(p.Path, p.After, mode); err != nil {
		return "", err
	}
	if bak != "" {
		pruneOlderBackups(p.Bases, p.Path, bak, stamp)
	}
	return bak, nil
}
