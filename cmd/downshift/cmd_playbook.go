// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/playbook"
)

const playbookUsage = `usage: downshift playbook <command>

  list                 the playbooks a delegation can name, with their size
  show <name>          print one playbook's text
  edit <name>          open a playbook in $VISUAL or $EDITOR, then check it
  add <name>           create a playbook for an area, then open it
  remove <name>        drop a playbook (shipped ones come back with restore)
  restore <name>       put a shipped playbook back to the text that ships
  check                validate every playbook: names unique, text present

General always applies and cannot be removed. The text is handed to a worker
verbatim, so keep it short: it travels with every task of its area.
`

// runPlaybook is `downshift playbook`: the person's editor for the working
// rules a delegated worker receives. Files live under the state directory.
func runPlaybook(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprint(stdout, playbookUsage)
		if len(args) == 0 {
			return 2
		}
		return 0
	}
	dir, err := playbook.Dir()
	if err != nil {
		fmt.Fprintln(stderr, "playbook:", err)
		return 1
	}
	switch args[0] {
	case "list":
		return playbookList(dir, stdout, stderr)
	case "check":
		return playbookCheck(dir, stdout, stderr)
	case "show", "edit", "add", "remove", "restore":
		if len(args) < 2 {
			fmt.Fprintf(stderr, "playbook %s: needs a name\n%s", args[0], playbookUsage)
			return 2
		}
		return playbookAct(args[0], dir, strings.Join(args[1:], " "), stdout, stderr)
	default:
		fmt.Fprintf(stderr, "playbook: unknown command %q\n%s", args[0], playbookUsage)
		return 2
	}
}

func playbookList(dir string, stdout, stderr io.Writer) int {
	sets, err := playbook.Load(dir)
	if err != nil {
		fmt.Fprintln(stderr, "playbook list:", err)
		return 1
	}
	for _, s := range sets {
		fmt.Fprintf(stdout, "  %-14s %-8s %6d chars%s\n", s.Name, s.Source, s.Chars(), sizeNote(s))
	}
	for _, id := range playbook.ShippedIDs() {
		if _, ok := playbook.Find(sets, id); ok {
			continue
		}
		fmt.Fprintf(stdout, "  %-14s %-8s %6s\n", id, "removed", "")
	}
	return 0
}

func playbookCheck(dir string, stdout, stderr io.Writer) int {
	sets, err := playbook.Load(dir)
	if err != nil {
		fmt.Fprintln(stderr, "playbook check:", err)
		return 1
	}
	for _, s := range sets {
		if s.Text == "" {
			fmt.Fprintf(stderr, "playbook check: %s has no text\n", s.Name)
			return 1
		}
		if s.Chars() > playbook.WarnChars {
			fmt.Fprintf(stdout, "warning: %s is %d characters; it travels with every task of its area\n", s.Name, s.Chars())
		}
	}
	fmt.Fprintf(stdout, "ok: %d playbooks, names unique\n", len(sets))
	return 0
}

// playbookAct runs one command that takes a name. The name is matched after
// folding, so "banco de dados" finds "Banco de Dados"; a shipped set that was
// removed is found by its id, so `restore frontend` works.
func playbookAct(cmd, dir, name string, stdout, stderr io.Writer) int {
	sets, err := playbook.Load(dir)
	if err != nil {
		fmt.Fprintln(stderr, "playbook:", err)
		return 1
	}
	active, found := playbook.Find(sets, name)
	id := active.ID
	if !found {
		id = playbook.Slug(name)
	}
	switch cmd {
	case "show":
		if !found {
			fmt.Fprintf(stderr, "playbook: no playbook %q. Available: %s\n", name, strings.Join(playbook.Names(sets), ", "))
			return 1
		}
		fmt.Fprintln(stdout, active.Text)
		return 0
	case "edit":
		if !found {
			fmt.Fprintf(stderr, "playbook: %q is not active. Use `add` for a new one, or `restore` for a shipped one.\n", name)
			return 1
		}
		if err := playbook.Save(dir, active); err != nil {
			fmt.Fprintln(stderr, "playbook edit:", err)
			return 1
		}
		return editAndCheck(dir, id, stdout, stderr)
	case "add":
		if found {
			fmt.Fprintf(stderr, "playbook: %q already exists. Use `edit`.\n", active.Name)
			return 1
		}
		if id == "" {
			fmt.Fprintf(stderr, "playbook: %q has no usable characters for a name\n", name)
			return 1
		}
		if err := playbook.Save(dir, playbook.Set{ID: id, Name: strings.TrimSpace(name)}); err != nil {
			fmt.Fprintln(stderr, "playbook add:", err)
			return 1
		}
		return editAndCheck(dir, id, stdout, stderr)
	case "remove":
		if !found {
			fmt.Fprintf(stderr, "playbook: no playbook %q\n", name)
			return 1
		}
		if err := playbook.Remove(dir, id); err != nil {
			fmt.Fprintln(stderr, "playbook remove:", err)
			return 1
		}
		fmt.Fprintf(stdout, "removed %s\n", active.Name)
		return 0
	case "restore":
		if err := playbook.Restore(dir, id); err != nil {
			fmt.Fprintln(stderr, "playbook restore:", err)
			return 1
		}
		fmt.Fprintf(stdout, "restored %s to the shipped text\n", id)
		return 0
	}
	return 2
}

// editAndCheck opens the set's file in the person's editor, then checks the
// result. An empty file after editing removes the set, so a blank playbook
// never reaches a worker.
func editAndCheck(dir, id string, stdout, stderr io.Writer) int {
	path := playbook.FilePath(dir, id)
	if err := openEditor(path); err != nil {
		fmt.Fprintln(stderr, "playbook: editor:", err)
		return 1
	}
	sets, err := playbook.Load(dir)
	if err != nil {
		fmt.Fprintln(stderr, "playbook:", err)
		return 1
	}
	if s, ok := byPlaybookID(sets, id); ok {
		fmt.Fprintf(stdout, "%s: %d chars%s\n", s.Name, s.Chars(), sizeNote(s))
		return 0
	}
	fmt.Fprintf(stdout, "%s was empty, so it is not active\n", id)
	return 0
}

// openEditor runs $VISUAL, then $EDITOR, on the file, attached to the terminal.
func openEditor(path string) error {
	line := os.Getenv("VISUAL")
	if line == "" {
		line = os.Getenv("EDITOR")
	}
	if line == "" {
		line = "vi"
		if runtime.GOOS == "windows" {
			line = "notepad"
		}
	}
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return errors.New("VISUAL or EDITOR is empty")
	}
	cmd := exec.Command(fields[0], append(fields[1:], path)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

func sizeNote(s playbook.Set) string {
	if s.Chars() > playbook.WarnChars {
		return fmt.Sprintf("  warning: over %d characters", playbook.WarnChars)
	}
	return ""
}

func byPlaybookID(sets []playbook.Set, id string) (playbook.Set, bool) {
	for _, s := range sets {
		if s.ID == id {
			return s, true
		}
	}
	return playbook.Set{}, false
}
