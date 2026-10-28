// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package playbook

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mustLoad(t *testing.T, dir string) []Set {
	t.Helper()
	sets, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return sets
}

func TestLoadShipsDefaultsInOrder(t *testing.T) {
	sets := mustLoad(t, t.TempDir())
	var got []string
	for _, s := range sets {
		got = append(got, s.ID+"="+s.Source.String())
	}
	want := "general=default backend=default frontend=default"
	if strings.Join(got, " ") != want {
		t.Fatalf("defaults = %v, want %s", got, want)
	}
	if sets[0].Name != "General" || sets[0].Text == "" {
		t.Fatalf("general = %+v", sets[0])
	}
}

func TestEditOverridesShippedSet(t *testing.T) {
	dir := t.TempDir()
	if err := Save(dir, Set{ID: "backend", Name: "Backend", Text: "- Our own rule."}); err != nil {
		t.Fatal(err)
	}
	sets := mustLoad(t, dir)
	b, ok := byID(sets, "backend")
	if !ok || b.Source != SourceEdited || b.Text != "- Our own rule." {
		t.Fatalf("backend after edit = %+v", b)
	}
}

func TestRemoveShippedSetStaysRemovedUntilRestored(t *testing.T) {
	dir := t.TempDir()
	if err := Remove(dir, "frontend"); err != nil {
		t.Fatal(err)
	}
	if _, ok := byID(mustLoad(t, dir), "frontend"); ok {
		t.Fatal("removed default came back on load")
	}
	if err := Restore(dir, "frontend"); err != nil {
		t.Fatal(err)
	}
	if _, ok := byID(mustLoad(t, dir), "frontend"); !ok {
		t.Fatal("restore did not bring the shipped set back")
	}
}

func TestGeneralIsNeverRemoved(t *testing.T) {
	dir := t.TempDir()
	if err := Remove(dir, General); err == nil {
		t.Fatal("general was removed")
	}
	// An empty General file is read as "restore ours", never as removed.
	if err := os.WriteFile(filepath.Join(dir, "general.md"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := byID(mustLoad(t, dir), General); !ok {
		t.Fatal("an empty general file dropped the general set")
	}
}

func TestCustomSetAddedAndDeleted(t *testing.T) {
	dir := t.TempDir()
	if err := Save(dir, Set{ID: "banco-de-dados", Name: "Banco de Dados", Text: "- Use migrations."}); err != nil {
		t.Fatal(err)
	}
	s, ok := Find(mustLoad(t, dir), "banco de dados")
	if !ok || s.Source != SourceCustom || s.Name != "Banco de Dados" {
		t.Fatalf("custom set = %+v, found %v", s, ok)
	}
	if err := Remove(dir, "banco-de-dados"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "banco-de-dados.md")); !os.IsNotExist(err) {
		t.Fatalf("custom file still there: %v", err)
	}
}

func TestRestoreRefusesCustomSet(t *testing.T) {
	if err := Restore(t.TempDir(), "banco-de-dados"); err == nil {
		t.Fatal("restore accepted a custom set")
	}
}

func TestFindFoldsCaseAndAccents(t *testing.T) {
	sets := []Set{{ID: "x", Name: "Banco de Dados"}, {ID: "y", Name: "Configuração"}}
	for _, q := range []string{"banco de dados", "BANCO DE DADOS", "  banco   de dados ", "configuracao"} {
		if _, ok := Find(sets, q); !ok {
			t.Errorf("Find(%q) missed", q)
		}
	}
}

func TestSlug(t *testing.T) {
	cases := map[string]string{"Banco de Dados!": "banco-de-dados", "  Frontend ": "frontend", "!!!": ""}
	for in, want := range cases {
		if got := Slug(in); got != want {
			t.Errorf("Slug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidateRefusesDuplicateNames(t *testing.T) {
	dir := t.TempDir()
	if err := Save(dir, Set{ID: "a", Name: "Banco de Dados", Text: "x"}); err != nil {
		t.Fatal(err)
	}
	if err := Save(dir, Set{ID: "b", Name: "banco de dados", Text: "y"}); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil {
		t.Fatal("two sets with the same folded name loaded")
	}
}

func TestBlockDefaultIsGeneralOnly(t *testing.T) {
	block, err := Block(mustLoad(t, t.TempDir()), "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(block, "<downshift-playbooks>") || !strings.HasSuffix(block, "</downshift-playbooks>") {
		t.Fatalf("block is not delimited: %.80s", block)
	}
	if !strings.Contains(block, "# General") || strings.Contains(block, "# Backend") {
		t.Fatalf("default block should carry General only")
	}
}

func TestBlockPutsGeneralBeforeTheNamedArea(t *testing.T) {
	block, err := Block(mustLoad(t, t.TempDir()), "backend")
	if err != nil {
		t.Fatal(err)
	}
	g, b := strings.Index(block, "# General"), strings.Index(block, "# Backend")
	if g < 0 || b < 0 || g > b {
		t.Fatalf("want General then Backend, got general=%d backend=%d", g, b)
	}
}

func TestBlockNamedGeneralIsNotDuplicated(t *testing.T) {
	block, err := Block(mustLoad(t, t.TempDir()), "GENERAL")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(block, "# General") != 1 {
		t.Fatal("general appears twice")
	}
}

func TestBlockRefusesUnknownNameWithTheValidOnes(t *testing.T) {
	_, err := Block(mustLoad(t, t.TempDir()), "Mobile")
	if err == nil || !strings.Contains(err.Error(), "Available: General, Backend, Frontend") {
		t.Fatalf("err = %v", err)
	}
}

func TestSplitHeadingWithoutHeadingUsesTheFileName(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "notes.md"), []byte("- plain rule\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	s, ok := byID(mustLoad(t, dir), "notes")
	if !ok || s.Name != "notes" || s.Text != "- plain rule" {
		t.Fatalf("headingless file = %+v", s)
	}
}

func TestCharsCountsRunes(t *testing.T) {
	if n := (Set{Text: "ção"}).Chars(); n != 3 {
		t.Fatalf("Chars = %d, want 3", n)
	}
}
