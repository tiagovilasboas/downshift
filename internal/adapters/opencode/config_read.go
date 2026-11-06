// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package opencode

import (
	"errors"
	"fmt"
	"io"
	"os"
)

// maxConfigBytes caps every config file read. A larger file is not read at
// all: it counts as restrictive.
const maxConfigBytes = 1 << 20

var errConfigNotRegular = errors.New("config path is not a regular file")
var errConfigTooLarge = errors.New("config file is larger than the read cap")

// readBounded reads a regular file of at most maxConfigBytes. A missing file
// returns an error that wraps fs.ErrNotExist; a link, device, directory or
// oversized file returns an error that is not ErrNotExist.
func readBounded(path string) ([]byte, error) {
	st, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() {
		// A link in a config path is followed by OpenCode, so a file behind
		// one is read through it; anything else is not a config file.
		if st.Mode()&os.ModeSymlink == 0 {
			return nil, fmt.Errorf("%w: %s", errConfigNotRegular, path)
		}
		if st, err = os.Stat(path); err != nil {
			return nil, err
		}
		if !st.Mode().IsRegular() {
			return nil, fmt.Errorf("%w: %s", errConfigNotRegular, path)
		}
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, maxConfigBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > maxConfigBytes {
		return nil, errConfigTooLarge
	}
	return raw, nil
}
