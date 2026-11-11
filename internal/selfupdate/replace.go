// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package selfupdate

import (
	"fmt"
	"os"
	"path/filepath"
)

// Replace swaps the executable at target for data. target is resolved through
// symbolic links first, so a link on PATH keeps pointing at the new file. The
// new file is written next to the target and moved over it, so a failure
// leaves the old binary in place.
func Replace(target string, data []byte) (string, error) {
	real, err := filepath.EvalSymlinks(target)
	if err != nil {
		return "", fmt.Errorf("resolving %s: %w", target, err)
	}
	fi, err := os.Stat(real)
	if err != nil {
		return "", err
	}
	tmp, err := writeSibling(real, data, fi.Mode().Perm()|0o100)
	if err != nil {
		return "", err
	}
	if err := swap(real, tmp); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	return real, nil
}

// writeSibling writes data to a new file in target's directory.
func writeSibling(target string, data []byte, mode os.FileMode) (string, error) {
	f, err := os.CreateTemp(filepath.Dir(target), ".downshift-update-*")
	if err != nil {
		return "", fmt.Errorf("cannot write next to %s: %w", target, err)
	}
	name := f.Name()
	_, werr := f.Write(data)
	cerr := f.Close()
	if werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = os.Chmod(name, mode)
	}
	if werr != nil {
		_ = os.Remove(name)
		return "", fmt.Errorf("writing the new binary: %w", werr)
	}
	return name, nil
}

// OldCopy is where swap parks the previous binary on Windows.
func OldCopy(target string) string { return target + ".old" }
