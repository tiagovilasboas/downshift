// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package playbook

import (
	"fmt"
	"strings"
)

// Block composes the rules a worker receives: General, then the named set when
// one is given. An unknown name is refused with the valid names, so the
// caller can correct it in one round trip. The block is delimited, so the
// worker can see exactly what it was told.
func Block(sets []Set, named string) (string, error) {
	chosen := []Set{}
	if general, ok := byID(sets, General); ok {
		chosen = append(chosen, general)
	}
	if strings.TrimSpace(named) != "" {
		s, found := Find(sets, named)
		if !found {
			return "", fmt.Errorf("unknown playbook %q. Available: %s", named, strings.Join(Names(sets), ", "))
		}
		if s.ID != General {
			chosen = append(chosen, s)
		}
	}
	var b strings.Builder
	b.WriteString("<downshift-playbooks>\n")
	b.WriteString("These are the working rules for this task. If the repository states something different (CLAUDE.md, AGENTS.md, CONTRIBUTING.md), the repository wins.\n")
	for _, s := range chosen {
		b.WriteString("\n# " + s.Name + "\n\n" + s.Text + "\n")
	}
	b.WriteString("</downshift-playbooks>")
	return b.String(), nil
}

func byID(sets []Set, id string) (Set, bool) {
	for _, s := range sets {
		if s.ID == id {
			return s, true
		}
	}
	return Set{}, false
}
