// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package playbook

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/paths"
)

// Dir is the playbooks folder under the state directory. One file per set:
// "<id>.md", with "# Name" on its first line.
func Dir() (string, error) {
	state, err := paths.StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(state, "playbooks"), nil
}

// Load returns the active sets: the shipped defaults with the person's edits
// applied, then the person's own sets. A file with no text (empty, or only a
// heading) is not a playbook: for a default it means the person removed it; General cannot be removed, so an empty General file
// means ours. A missing folder is the shipped set.
func Load(dir string) ([]Set, error) {
	files, err := readFiles(dir)
	if err != nil {
		return nil, err
	}
	var out []Set
	for _, d := range shipped() {
		f, edited := files[d.ID]
		delete(files, d.ID)
		switch {
		case !edited, f.empty && d.ID == General:
			out = append(out, d)
		case f.empty:
		default:
			out = append(out, Set{ID: d.ID, Name: f.name, Text: f.text, Source: SourceEdited})
		}
	}
	ids := make([]string, 0, len(files))
	for id, f := range files {
		if !f.empty {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		f := files[id]
		name := f.name
		if name == "" {
			name = id
		}
		out = append(out, Set{ID: id, Name: name, Text: f.text, Source: SourceCustom})
	}
	return out, Validate(out)
}

// Save writes a set's file with its heading and text, atomically.
func Save(dir string, s Set) error {
	if s.ID == "" {
		return errors.New("a playbook needs an id")
	}
	body := "# " + s.Name + "\n\n" + strings.TrimSpace(s.Text) + "\n"
	if strings.TrimSpace(s.Text) == "" {
		body = "# " + s.Name + "\n"
	}
	return writeAtomic(filepath.Join(dir, s.ID+".md"), []byte(body))
}

// Remove drops a set. A shipped set gets an empty file, so it does not come
// back as the default on the next load; a custom set's file is deleted.
// General cannot be removed.
func Remove(dir, id string) error {
	if id == General {
		return errors.New("general cannot be removed: edit it, or restore ours")
	}
	path := filepath.Join(dir, id+".md")
	if isShipped(id) {
		return writeAtomic(path, nil)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// Restore puts a shipped set back to the text that ships, by deleting the
// person's file. A custom set has nothing to restore.
func Restore(dir, id string) error {
	if !isShipped(id) {
		return fmt.Errorf("%s is not a shipped playbook: nothing to restore", id)
	}
	if err := os.Remove(filepath.Join(dir, id+".md")); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// ShippedIDs lists the default sets in display order.
func ShippedIDs() []string {
	var ids []string
	for _, d := range shipped() {
		ids = append(ids, d.ID)
	}
	return ids
}

// isShipped reports whether id is one of the default sets.
func isShipped(id string) bool {
	for _, d := range ShippedIDs() {
		if d == id {
			return true
		}
	}
	return false
}

type file struct {
	name, text string
	empty      bool
}

// readFiles reads every "<id>.md" in dir. A file with no text is marked empty;
// a missing folder reads as no files.
func readFiles(dir string) (map[string]file, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]file{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := map[string]file{}
	for _, e := range entries {
		id, ok := strings.CutSuffix(e.Name(), ".md")
		if e.IsDir() || !ok || id == "" {
			continue
		}
		// A playbook is prefixed to a worker's prompt, so a link that could point
		// at any file is refused rather than followed.
		if !e.Type().IsRegular() {
			return nil, fmt.Errorf("playbook %s is not a regular file (links are refused)", e.Name())
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		name, text := splitHeading(string(b))
		out[id] = file{name: name, text: text, empty: text == ""}
	}
	return out, nil
}

// writeAtomic writes through a temporary file, so a crash never leaves half a playbook.
func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".playbook-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// FilePath is where a set's text lives under dir.
func FilePath(dir, id string) string { return filepath.Join(dir, id+".md") }
