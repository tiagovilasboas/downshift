// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"errors"
	"path/filepath"
	"strings"
)

// readDirUnderBroad refuses a resolved read dir that is broad or has a path
// component below the (resolved) root starting with ".".
func (d Deps) readDirUnderBroad(resolved string) error {
	if d.BroadDir != nil && d.BroadDir(resolved) {
		return errors.New("read is refused: dir is a broad directory (the filesystem root, your home, or an ancestor of it); " + broadFix)
	}
	root, err := resolveLoose(d.Root)
	if err != nil {
		return errors.New("read is refused: the project directory cannot be resolved; " + broadFix)
	}
	rel, err := filepath.Rel(root, resolved)
	if err != nil {
		return errDirOutside
	}
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if strings.HasPrefix(part, ".") && part != "." {
			return errors.New("read is refused: dir is inside a hidden directory (" + part + ") below a broad server root; " + broadFix)
		}
	}
	return nil
}
