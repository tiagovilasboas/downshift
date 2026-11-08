// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package limits

import (
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
)

// The "Capability grid" section of the harness matrix has one row per
// HarnessCapabilities field and one column per harness. These tests hold every
// cell to core.AllHarnessCaps(), so a capability cannot change in code without
// the matrix changing with it.

const gridSection = "## Capability grid (`HarnessCapabilities`)"

// gridSkipped lists the HarnessCapabilities fields that have no grid row, with
// the reason. Every other field must have a row; a new field that is neither a
// row nor listed here fails TestCapabilityGridCoversEveryField.
var gridSkipped = map[string]string{
	"NativeUsageSource":  "qualifies NativeSessionEnv (the parser name for that export); no independent capability",
	"MainToolCategories": "qualifies MainToolEvent (tool name to category); no independent capability",
}

// gridVocabulary is the fixed cell vocabulary.
var gridVocabulary = map[string]bool{"yes": true, "no": true, "partial": true}

// declared reports whether a capability field carries a value: a true switch,
// a non-empty string, slice or map, or a positive number. UsageSource also
// counts as declared only when it names a real source (UsageNone is "none").
func declared(name string, v reflect.Value) bool {
	if name == "UsageSource" {
		return v.String() != "" && v.String() != core.UsageNone
	}
	switch v.Kind() {
	case reflect.Bool:
		return v.Bool()
	case reflect.String, reflect.Slice, reflect.Map:
		return v.Len() > 0
	case reflect.Int:
		return v.Int() > 0
	}
	return false
}

var cellWord = regexp.MustCompile(`^[a-z]+`)

// readGrid returns capability field -> harness id -> cell word, from the
// matrix's capability grid table.
func readGrid(t *testing.T) map[string]map[string]string {
	t.Helper()
	data, err := os.ReadFile(matrixPath)
	if err != nil {
		t.Fatalf("read harness matrix: %v", err)
	}
	grid := map[string]map[string]string{}
	var header []string
	in := false
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "## ") {
			in = strings.TrimSpace(line) == gridSection
			continue
		}
		if !in || !strings.HasPrefix(line, "|") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "| "), "|")
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		switch {
		case header == nil:
			header = cells
		case strings.HasPrefix(cells[0], "---"):
		default:
			field := strings.Trim(cells[0], "`")
			if _, dup := grid[field]; dup {
				t.Fatalf("capability grid has two rows for %s", field)
			}
			if len(cells) != len(header) {
				t.Fatalf("capability grid row %s has %d cells, header has %d", field, len(cells), len(header))
			}
			row := map[string]string{}
			for i := 1; i < len(cells); i++ {
				row[header[i]] = cells[i]
			}
			grid[field] = row
		}
	}
	if len(grid) == 0 {
		t.Fatalf("harness matrix has no %q table", gridSection)
	}
	return grid
}

// TestCapabilityGridCoversEveryField fails when a capability field exists in
// code that the grid neither shows nor explicitly skips, and when the grid or
// the skip list names a field that no longer exists.
func TestCapabilityGridCoversEveryField(t *testing.T) {
	grid := readGrid(t)
	ty := reflect.TypeOf(core.HarnessCapabilities{})
	fields := map[string]bool{}
	for i := 0; i < ty.NumField(); i++ {
		name := ty.Field(i).Name
		fields[name] = true
		_, inGrid := grid[name]
		_, skipped := gridSkipped[name]
		switch {
		case inGrid && skipped:
			t.Errorf("%s is both a grid row and in gridSkipped", name)
		case !inGrid && !skipped:
			t.Errorf("capability %s has no grid row in the matrix and is not in gridSkipped", name)
		}
	}
	for name := range grid {
		if !fields[name] {
			t.Errorf("grid row %s is not a HarnessCapabilities field", name)
		}
	}
	for name := range gridSkipped {
		if !fields[name] {
			t.Errorf("gridSkipped entry %s is not a HarnessCapabilities field", name)
		}
	}
}

// TestCapabilityGridMatchesCaps compares every cell with the code.
func TestCapabilityGridMatchesCaps(t *testing.T) {
	grid := readGrid(t)
	all := core.AllHarnessCaps()
	for _, named := range all {
		rv := reflect.ValueOf(named.Caps)
		for field, row := range grid {
			cell, ok := row[named.Harness]
			if !ok {
				t.Errorf("capability grid has no %s column", named.Harness)
				break
			}
			word := cellWord.FindString(cell)
			if !gridVocabulary[word] {
				t.Errorf("%s/%s: cell %q must start with yes, no or partial", field, named.Harness, cell)
				continue
			}
			v := rv.FieldByName(field)
			if !v.IsValid() {
				continue // reported by the coverage test
			}
			want := "no"
			if declared(field, v) {
				want = "yes"
			}
			if word == "partial" {
				t.Errorf("%s/%s: partial matches no code state (code says %s); use yes or no with a note", field, named.Harness, want)
			} else if word != want {
				t.Errorf("%s/%s: matrix says %s, code says %s", field, named.Harness, word, want)
			}
		}
	}
	for _, name := range gridColumns(t) {
		found := false
		for _, n := range all {
			found = found || n.Harness == name
		}
		if !found {
			t.Errorf("capability grid column %q is not in core.AllHarnessCaps()", name)
		}
	}
}

// gridColumns returns the harness columns of the grid, from any one row.
func gridColumns(t *testing.T) []string {
	t.Helper()
	var cols []string
	for _, row := range readGrid(t) {
		for h := range row {
			if h != "capability" && h != "meaning" {
				cols = append(cols, h)
			}
		}
		break
	}
	return cols
}
