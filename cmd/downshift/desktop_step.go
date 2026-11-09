// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"

	"github.com/tiagovilasboas/downshift-ai/internal/desktopview"
	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
)

// desktopDeltaEnv turns the agent-desktop delta off when set to 0; the
// compact view itself stays on (it is lossless).
const desktopDeltaEnv = "DOWNSHIFT_DESKTOP_DELTA"

// desktopStep is the execwrap Desktop step for one run: a repeat read of the
// same window, in the same session, is printed as the delta from the view of
// the previous read (internal/desktopview). It is nil, and every read is the
// whole view, without a session, for any other family, with the env switch
// off, or when the state directory cannot be resolved.
func desktopStep(session string, argv []string) func([]byte) []byte {
	if os.Getenv(desktopDeltaEnv) == "0" || execwrap.FamilyOf(argv) != execwrap.FamilyAgentDesktop {
		return nil
	}
	key := execwrap.DesktopScope(session, argv)
	if key == "" {
		return nil
	}
	store, err := desktopview.Default()
	if err != nil {
		return nil
	}
	return func(view []byte) []byte { return store.Apply(key, view) }
}
