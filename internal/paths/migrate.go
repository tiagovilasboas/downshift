// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MovedNote is the one-line file left in the legacy directory after a
// migration. It is not state: it never counts as legacy content.
const MovedNote = "MOVED.txt"

// Migration reports what MigrateLegacy did, or would do under dry run.
type Migration struct {
	Legacy string   // legacy directory
	Target string   // new state directory
	Moved  []string // entry names moved (or to be moved)
	DryRun bool
	Both   bool // both directories hold state; nothing was moved

	// Collide names the entries present in both directories (Both only).
	Collide []string
	// Warnings are non-fatal: data is safe in Target.
	Warnings []string
}

// MigrateLegacy moves the legacy state directory's files into the new state
// directory when the new one is absent or empty. A non-empty new directory is
// never touched (Both is set instead). A set DOWNSHIFT_STATE_DIR skips the
// migration: the user chose the directory. Only `downshift setup` calls this;
// hooks stay fast and never move files. A zero Migration means nothing to do.
func MigrateLegacy(dryRun bool) (Migration, error) {
	if os.Getenv(EnvStateDir) != "" {
		return Migration{}, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return Migration{}, err
	}
	m := Migration{
		Legacy: filepath.Join(home, LegacyName),
		Target: filepath.Join(home, DirName),
		DryRun: dryRun,
	}
	names, err := legacyEntries(m.Legacy)
	if err != nil || len(names) == 0 {
		return Migration{}, err
	}
	if n, _ := dirEntryCount(m.Target); n > 0 {
		m.Both = true
		m.Collide = collisions(m.Target, names)
		return m, nil
	}
	m.Moved = names
	if dryRun {
		return m, nil
	}
	warn, err := moveEntries(m, names)
	m.Warnings = warn
	if err != nil {
		return m, err
	}
	note := fmt.Sprintf("Downshift state moved to %s\n", m.Target)
	_ = os.WriteFile(filepath.Join(m.Legacy, MovedNote), []byte(note), 0o600)
	return m, nil
}

// Describe renders the report for setup output; "" when idle.
func (m Migration) Describe() string {
	var b strings.Builder
	switch {
	case m.Both:
		fmt.Fprintf(&b, "State: both %s and %s hold files; nothing moved. Merge by hand or remove one.\n", m.Legacy, m.Target)
		if len(m.Collide) > 0 {
			fmt.Fprintf(&b, "State: present in both: %s\n", strings.Join(m.Collide, ", "))
		}
	case len(m.Moved) == 0:
		return ""
	case m.DryRun:
		fmt.Fprintf(&b, "State: would move %d entries from %s to %s\n", len(m.Moved), m.Legacy, m.Target)
	default:
		fmt.Fprintf(&b, "State: moved %d entries from %s to %s\n", len(m.Moved), m.Legacy, m.Target)
	}
	for _, w := range m.Warnings {
		fmt.Fprintf(&b, "State: %s\n", w)
	}
	return b.String()
}
