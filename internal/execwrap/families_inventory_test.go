// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

// Doc-vs-code check: the family inventory in docs/reference/harness-matrix.md
// must list exactly the Family constants the policy files declare, and each
// row's behaviour must match expectedFormat (compresses) or its absence
// (passes through). Adding a family without a doc row, or a row for a family
// that does not exist, fails here.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const familyInventoryDoc = "../../docs/reference/harness-matrix.md"

const familyInventoryHeader = "| family | command forms | behaviour |"

var familyInventoryRow = regexp.MustCompile("^\\| `([^`]+)` \\| (.+) \\| (compresses|passes through) \\|$")

type inventoryRow struct {
	family   string
	behavior string
}

// parseFamilyInventory returns the rows of the inventory table, in document order.
func parseFamilyInventory(t *testing.T) []inventoryRow {
	t.Helper()
	raw, err := os.ReadFile(filepath.FromSlash(familyInventoryDoc))
	if err != nil {
		t.Fatalf("read %s: %v", familyInventoryDoc, err)
	}
	lines := strings.Split(string(raw), "\n")
	start := -1
	for i, line := range lines {
		if line == familyInventoryHeader {
			start = i + 2 // skip the header and the separator row
			break
		}
	}
	if start < 0 {
		t.Fatalf("%s has no family inventory table with header %q", familyInventoryDoc, familyInventoryHeader)
	}
	var rows []inventoryRow
	for _, line := range lines[start:] {
		if !strings.HasPrefix(line, "|") {
			break
		}
		m := familyInventoryRow.FindStringSubmatch(line)
		if m == nil {
			t.Fatalf("malformed inventory row: %q", line)
		}
		rows = append(rows, inventoryRow{family: m[1], behavior: m[3]})
	}
	if len(rows) == 0 {
		t.Fatal("family inventory table has no rows")
	}
	return rows
}

func TestFamilyInventoryMatchesFamilyOf(t *testing.T) {
	declared := familyConstants(t)
	rows := parseFamilyInventory(t)

	listed := map[string]bool{}
	for _, row := range rows {
		if listed[row.family] {
			t.Errorf("family %q is listed twice in %s", row.family, familyInventoryDoc)
		}
		listed[row.family] = true
	}

	for name, fam := range declared {
		if !listed[string(fam)] {
			t.Errorf("%s (%q) is a Family constant but has no row in %s", name, fam, familyInventoryDoc)
		}
	}
	declaredValues := map[string]bool{}
	for _, fam := range declared {
		declaredValues[string(fam)] = true
	}
	for _, row := range rows {
		if !declaredValues[row.family] {
			t.Errorf("%s lists %q, which no Family constant declares", familyInventoryDoc, row.family)
		}
	}
}

func TestFamilyInventoryBehaviourMatchesExpectedFormat(t *testing.T) {
	for _, row := range parseFamilyInventory(t) {
		_, compresses := expectedFormat[Family(row.family)]
		switch {
		case compresses && row.behavior != "compresses":
			t.Errorf("%q has an expectedFormat entry but the inventory says %q", row.family, row.behavior)
		case !compresses && row.behavior != "passes through":
			t.Errorf("%q has no expectedFormat entry but the inventory says %q", row.family, row.behavior)
		}
	}
}
