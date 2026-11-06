// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package playbook

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// byID finds a set by its exact id.
func byID(sets []Set, id string) (Set, bool) {
	for _, s := range sets {
		if s.ID == id {
			return s, true
		}
	}
	return Set{}, false
}

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
	want := "general=default backend=default frontend=default research=default implementation=default review=default tdd-verification=default docs=default incident-rescue=default scout-then-build=default cascade=default audit-at-scale=default best-of-n=default"
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

func TestHeadingOnlyFileIsNotAPlaybook(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "dados.md"), []byte("# Dados\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := byID(mustLoad(t, dir), "dados"); ok {
		t.Fatal("a heading with no text became an active playbook")
	}
}

func TestSymlinkedPlaybookIsRefused(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(t.TempDir(), "secret.md")
	if err := os.WriteFile(target, []byte("# X\n\n- text\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "x.md")); err != nil {
		t.Skip("symlinks are not available here")
	}
	if _, err := Load(dir); err == nil {
		t.Fatal("a symlinked playbook was followed")
	}
}
