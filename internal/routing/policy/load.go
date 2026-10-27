// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/tiagovilasboas/downshift/internal/paths"
)

// FileName is the policy file name, in a project's .downshift directory and in
// the state directory.
const FileName = "policy.json"

// Locations names the two files a load reads. An empty path is an absent layer.
type Locations struct {
	Project string // <project>/.downshift/policy.json
	User    string // <state dir>/policy.json
}

// UserPath is the user's policy file in the effective state directory
// (DOWNSHIFT_STATE_DIR, else the default one).
func UserPath() (string, error) {
	dir, err := paths.StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, FileName), nil
}

// ProjectPath is the project policy file under the project directory dir.
func ProjectPath(dir string) string {
	return filepath.Join(dir, paths.DirName, FileName)
}

// Load reads, validates and merges the two files. It returns an error and no
// policy when any file cannot be used. A missing file is an absent layer, so
// Load of no files returns the defaults. The result is deterministic: the same
// files always give the same policy or the same error.
func Load(loc Locations) (*Policy, error) {
	project, err := readLayer(loc.Project)
	if err != nil {
		return nil, err
	}
	user := layer{}
	if !sameFile(loc.User, loc.Project) {
		if user, err = readLayer(loc.User); err != nil {
			return nil, err
		}
	}
	p := combine(project, user)
	if err := p.checkRefs(); err != nil {
		return nil, err
	}
	return p, nil
}

// sameFile reports whether two non-empty paths name the same file. A project
// search can climb to the user's own .downshift directory; reading it twice
// would add nothing, so the user layer is skipped.
func sameFile(user, project string) bool {
	return user == "" || (project != "" && filepath.Clean(user) == filepath.Clean(project))
}

func readLayer(path string) (layer, error) {
	if path == "" {
		return layer{}, nil
	}
	fi, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return layer{}, nil
	}
	if err != nil {
		return layer{}, refuse(path, "file", "%v", err)
	}
	if !fi.Mode().IsRegular() {
		return layer{}, refuse(path, "file", "not a regular file")
	}
	if fi.Size() > maxBytes {
		return layer{}, refuse(path, "file", "larger than %d bytes", maxBytes)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return layer{}, refuse(path, "file", "%v", err)
	}
	return parseLayer(path, data)
}

// parseLayer reads the routing section of one file. A file without a routing
// key holds no routing policy, and its other keys are not this package's: the
// tier floor in internal/policyfile reads them from the same file.
func parseLayer(source string, data []byte) (layer, error) {
	var doc struct {
		Version *int            `json:"version"`
		Routing json.RawMessage `json:"routing"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return layer{}, refuse(source, "file", "not valid JSON: %v", err)
	}
	if doc.Routing == nil {
		return layer{source: source}, nil
	}
	if doc.Version == nil || *doc.Version != 1 {
		return layer{}, refuse(source, "version", "must be 1 when routing is set")
	}
	if bytes.Equal(bytes.TrimSpace(doc.Routing), []byte("null")) {
		return layer{}, refuse(source, "routing", "must be an object")
	}
	dec := json.NewDecoder(bytes.NewReader(doc.Routing))
	dec.DisallowUnknownFields()
	var s section
	if err := dec.Decode(&s); err != nil {
		return layer{}, refuse(source, "routing", "%v", err)
	}
	return validateSection(source, s)
}
