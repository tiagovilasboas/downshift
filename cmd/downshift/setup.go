// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// `downshift setup [harness...] [--dry-run] [--undo] [--yes]` wires every
// Downshift hook into the harness hook files listed in setup_targets.go. It
// merges (never replaces), is idempotent, shows a diff and keeps a backup.
// The per-harness facts are data in setupTargets; nothing here names a harness.

const setupBackupInfix = ".downshift-bak-"

type setupOptions struct {
	Names  []string
	DryRun bool
	Undo   bool
	Yes    bool
	// NoSessionPolicy skips the optional main-session policy (model opusplan and
	// plan as the starting mode); hooks are wired as usual.
	NoSessionPolicy bool
	Home            string
	Binary          string
	Now             func() time.Time
	In              io.Reader
	Out             io.Writer
	Err             io.Writer
}

// runSetup is the CLI entry: it resolves HOME and the running binary.
func runSetup(args []string, in io.Reader, out, errw io.Writer) int {
	opts := setupOptions{In: in, Out: out, Err: errw, Now: time.Now}
	for _, a := range args {
		switch {
		case a == "--dry-run":
			opts.DryRun = true
		case a == "--undo":
			opts.Undo = true
		case a == "--yes" || a == "-y":
			opts.Yes = true
		case a == "--no-session-policy":
			opts.NoSessionPolicy = true
		case a == "-h" || a == "--help":
			fmt.Fprint(out, setupUsage)
			return 0
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(errw, "setup: unknown flag %q\n%s", a, setupUsage)
			return 2
		default:
			opts.Names = append(opts.Names, a)
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(errw, "setup: cannot find the home directory: %v\n", err)
		return 1
	}
	opts.Home = home
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(errw, "setup: cannot find this binary: %v\n", err)
		return 1
	}
	opts.Binary = exe
	return setupRun(opts)
}

const setupUsage = `usage: downshift setup [harness...] [--dry-run] [--undo] [--yes] [--no-session-policy]

Wire every Downshift hook into the harnesses found on this machine.
  --dry-run            show the diff, write nothing
  --undo               restore the latest backup of each hook file
  --yes                do not ask before writing
  --no-session-policy  wire the hooks only: do not set the Claude Code
                       main-session policy (model opusplan, sessions start in
                       plan mode)

Claude Code gets the policy by default, only for keys it does not already set.
`

// setupPlan is the outcome of planning one target.
type setupPlan struct {
	target  setupTarget
	path    string
	before  []byte // nil when the file does not exist
	existed bool
	after   []byte
	notes   []string
	err     error
}

func (p setupPlan) changed() bool {
	return p.err == nil && (!p.existed || string(p.before) != string(p.after))
}

func setupRun(o setupOptions) int {
	targets, code := setupSelect(o)
	if code != 0 {
		return code
	}
	if o.Undo {
		return setupUndo(o, targets)
	}

	var plans []setupPlan
	for _, t := range targets {
		if t.File == "" {
			fmt.Fprintf(o.Out, "%s: detected, not wired by setup\n  %s\n\n", t.Name, t.Manual)
			continue
		}
		plans = append(plans, planTarget(o, t))
	}

	failed := false
	pending := 0
	for _, p := range plans {
		shown := setupDisplay(o.Home, p.path)
		switch {
		case p.err != nil:
			failed = true
			fmt.Fprintf(o.Err, "%s: %s: %v\n  nothing was written for %s\n\n", p.target.Name, shown, p.err, p.target.Name)
		case !p.changed():
			fmt.Fprintf(o.Out, "%s: %s already up to date\n", p.target.Name, shown)
			setupPrintNotes(o.Out, p.notes)
			fmt.Fprintln(o.Out)
		default:
			pending++
			verb := "will change"
			if o.DryRun {
				verb = "would change"
			}
			fmt.Fprintf(o.Out, "%s: %s %s\n", p.target.Name, shown, verb)
			fmt.Fprint(o.Out, unifiedDiff(diffName(shown, p.existed, "a"), diffName(shown, true, "b"), string(p.before), string(p.after)))
			setupPrintNotes(o.Out, p.notes)
			fmt.Fprintln(o.Out)
		}
	}

	if pending == 0 || o.DryRun {
		if o.DryRun && pending > 0 {
			fmt.Fprintln(o.Out, "Dry run: nothing was written.")
		}
		if failed {
			return 1
		}
		return 0
	}
	if !setupConfirm(o, "Write these changes? A backup of each existing file is kept. [y/N] ") {
		fmt.Fprintln(o.Out, "Not applied. Re-run with --yes to skip this question.")
		return 1
	}
	stamp := o.Now().UTC().Format("20060102T150405.000000")
	for _, p := range plans {
		if !p.changed() {
			continue
		}
		bak, err := setupWrite(p, stamp)
		if err != nil {
			failed = true
			fmt.Fprintf(o.Err, "%s: write failed: %v\n", p.target.Name, err)
			continue
		}
		if bak != "" {
			fmt.Fprintf(o.Out, "%s: written. Backup: %s\n", p.target.Name, setupDisplay(o.Home, bak))
		} else {
			fmt.Fprintf(o.Out, "%s: written (new file).\n", p.target.Name)
		}
	}
	fmt.Fprint(o.Out, setupSummary(plans))
	fmt.Fprintln(o.Out, "Undo with: downshift setup --undo")
	if failed {
		return 1
	}
	return 0
}

// setupSummary says, once the files are written, what now applies to the
// developer. It reads the written Claude Code document so it never claims a
// session policy that is not in the file.
func setupSummary(plans []setupPlan) string {
	wrote := false
	var policy []string
	for _, p := range plans {
		if p.err != nil || !p.changed() {
			continue
		}
		wrote = true
		if p.target.Name != "claude-code" {
			continue
		}
		doc, err := parseJSONDoc(p.after)
		if err != nil {
			continue
		}
		if m, _ := doc.get("model"); m == "opusplan" {
			policy = append(policy, "the main session uses opusplan (Opus plans, Sonnet executes)")
		}
		if perms, ok := doc.get("permissions"); ok {
			if po, isObj := asObj(perms); isObj {
				if mode, _ := po.get("defaultMode"); mode == "plan" {
					policy = append(policy, "sessions start in plan mode: Claude plans and you approve before it edits")
				}
			}
		}
	}
	if !wrote {
		return ""
	}
	var b strings.Builder
	b.WriteString("\nWhat changes for you\n")
	b.WriteString("  Subagents: Downshift picks the model for each spawn from the task. Your main session is not touched.\n")
	if share := telemetry.ControlShare(); share > 0 {
		fmt.Fprintf(&b, "  Control group: about 1 in %d spawns runs unrouted on purpose, so `downshift report` can measure the savings. DOWNSHIFT_CONTROL_SHARE=0 turns it off.\n", int(math.Round(1/share)))
	} else {
		b.WriteString("  Control group: off (DOWNSHIFT_CONTROL_SHARE=0), so `downshift report` has nothing to compare.\n")
	}
	if len(policy) > 0 {
		fmt.Fprintf(&b, "  Claude Code: %s. Skip this with `downshift setup --no-session-policy`.\n", strings.Join(policy, "; "))
	}
	b.WriteString("  Check anything with `downshift doctor`, see the effect with `downshift stats`.\n\n")
	return b.String()
}

// setupSelect resolves which targets to act on: the named ones, or every
// harness whose config directory exists.
func setupSelect(o setupOptions) ([]setupTarget, int) {
	byName := map[string]setupTarget{}
	var all []string
	for _, t := range setupTargets {
		byName[t.Name] = t
		all = append(all, t.Name)
	}
	var out []setupTarget
	if len(o.Names) > 0 {
		seen := map[string]bool{}
		for _, n := range o.Names {
			t, ok := byName[n]
			if !ok {
				fmt.Fprintf(o.Err, "setup: unknown harness %q (known: %s)\n", n, strings.Join(all, ", "))
				return nil, 2
			}
			if seen[n] {
				continue
			}
			seen[n] = true
			if !isDir(filepath.Join(o.Home, t.ConfigDir)) {
				fmt.Fprintf(o.Err, "setup: %s not found: ~/%s does not exist\n", n, t.ConfigDir)
				return nil, 1
			}
			out = append(out, t)
		}
		return out, 0
	}
	for _, t := range setupTargets {
		if isDir(filepath.Join(o.Home, t.ConfigDir)) {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		var dirs []string
		for _, t := range setupTargets {
			dirs = append(dirs, "~/"+t.ConfigDir)
		}
		fmt.Fprintf(o.Err, "setup: no supported harness found (looked for %s)\n", strings.Join(dirs, ", "))
		return nil, 1
	}
	return out, 0
}

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func diffName(shown string, exists bool, side string) string {
	if !exists {
		return "/dev/null"
	}
	return side + "/" + shown
}

func setupPrintNotes(w io.Writer, notes []string) {
	for _, n := range notes {
		fmt.Fprintf(w, "  note: %s\n", n)
	}
}

// setupDisplay shortens a path under HOME to ~/....
func setupDisplay(home, p string) string {
	if rel, err := filepath.Rel(home, p); err == nil && !strings.HasPrefix(rel, "..") {
		return "~/" + filepath.ToSlash(rel)
	}
	return p
}

func setupConfirm(o setupOptions, prompt string) bool {
	if o.Yes {
		return true
	}
	fmt.Fprint(o.Out, prompt)
	line, _ := bufio.NewReader(o.In).ReadString('\n')
	fmt.Fprintln(o.Out)
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	}
	return false
}

// planTarget reads the file and computes the merged document. It never writes.
func planTarget(o setupOptions, t setupTarget) setupPlan {
	p := setupPlan{target: t, path: filepath.Join(o.Home, filepath.FromSlash(t.File))}
	raw, err := os.ReadFile(p.path)
	switch {
	case err == nil:
		p.existed = true
		p.before = raw
	case errors.Is(err, os.ErrNotExist):
	default:
		p.err = err
		return p
	}
	doc, err := parseJSONDoc(raw)
	if err != nil {
		p.err = fmt.Errorf("not valid JSON, left untouched: %w", err)
		return p
	}
	notes, err := mergeTarget(doc, t, o.Binary, o.NoSessionPolicy)
	if err != nil {
		p.err = err
		return p
	}
	p.notes = append(notes, t.Notes...)
	out, err := renderJSONDoc(doc)
	if err != nil {
		p.err = err
		return p
	}
	p.after = out
	return p
}

// setupWrite backs up the existing file, then writes the new content
// atomically. It returns the backup path ("" for a new file; a marker file
// records that the file did not exist so --undo can remove it).
func setupWrite(p setupPlan, stamp string) (string, error) {
	if err := os.MkdirAll(filepath.Dir(p.path), 0o755); err != nil {
		return "", err
	}
	mode := os.FileMode(0o644)
	bak := p.path + setupBackupInfix + stamp
	if p.existed {
		if st, err := os.Stat(p.path); err == nil {
			mode = st.Mode().Perm()
		}
		if err := os.WriteFile(bak, p.before, mode); err != nil {
			return "", fmt.Errorf("backup: %w", err)
		}
	} else {
		if err := os.WriteFile(bak+".new", nil, 0o644); err != nil {
			return "", fmt.Errorf("backup marker: %w", err)
		}
		bak = ""
	}
	tmp := p.path + ".downshift-tmp"
	if err := os.WriteFile(tmp, p.after, mode); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, p.path); err != nil {
		os.Remove(tmp)
		return "", err
	}
	return bak, nil
}

// latestBackup returns the newest backup for path, and whether it is the
// "file did not exist" marker.
func latestBackup(path string) (backup string, marker bool) {
	matches, _ := filepath.Glob(path + setupBackupInfix + "*")
	if len(matches) == 0 {
		return "", false
	}
	sort.Strings(matches)
	b := matches[len(matches)-1]
	return b, strings.HasSuffix(b, ".new")
}

func setupUndo(o setupOptions, targets []setupTarget) int {
	type restore struct {
		t      setupTarget
		path   string
		backup string
		marker bool
		cur    []byte
		want   []byte
	}
	var todo []restore
	for _, t := range targets {
		if t.File == "" {
			continue
		}
		path := filepath.Join(o.Home, filepath.FromSlash(t.File))
		b, marker := latestBackup(path)
		if b == "" {
			fmt.Fprintf(o.Out, "%s: no backup of %s, nothing to undo\n", t.Name, setupDisplay(o.Home, path))
			continue
		}
		r := restore{t: t, path: path, backup: b, marker: marker}
		r.cur, _ = os.ReadFile(path)
		if !marker {
			data, err := os.ReadFile(b)
			if err != nil {
				fmt.Fprintf(o.Err, "%s: cannot read backup %s: %v\n", t.Name, b, err)
				return 1
			}
			r.want = data
		}
		todo = append(todo, r)
	}
	if len(todo) == 0 {
		return 0
	}
	for _, r := range todo {
		shown := setupDisplay(o.Home, r.path)
		verb := "will restore"
		if o.DryRun {
			verb = "would restore"
		}
		if r.marker {
			fmt.Fprintf(o.Out, "%s: %s %s to absent (setup created it)\n", r.t.Name, shown, verb)
		} else {
			fmt.Fprintf(o.Out, "%s: %s %s from %s\n", r.t.Name, shown, verb, setupDisplay(o.Home, r.backup))
		}
		fmt.Fprint(o.Out, unifiedDiff("a/"+shown, diffName(shown, !r.marker, "b"), string(r.cur), string(r.want)))
		fmt.Fprintln(o.Out)
	}
	if o.DryRun {
		fmt.Fprintln(o.Out, "Dry run: nothing was written.")
		return 0
	}
	if !setupConfirm(o, "Restore these files? [y/N] ") {
		fmt.Fprintln(o.Out, "Not applied. Re-run with --yes to skip this question.")
		return 1
	}
	code := 0
	for _, r := range todo {
		var err error
		if r.marker {
			err = os.Remove(r.path)
			if errors.Is(err, os.ErrNotExist) {
				err = nil
			}
		} else {
			err = os.WriteFile(r.path, r.want, 0o644)
			if st, serr := os.Stat(r.backup); serr == nil && err == nil {
				err = os.Chmod(r.path, st.Mode().Perm())
			}
		}
		if err != nil {
			fmt.Fprintf(o.Err, "%s: restore failed: %v\n", r.t.Name, err)
			code = 1
			continue
		}
		fmt.Fprintf(o.Out, "%s: restored %s\n", r.t.Name, setupDisplay(o.Home, r.path))
	}
	return code
}

// ---- merge ---------------------------------------------------------------

// quoteCommand renders `<binary> <args>`, quoting the binary when needed.
func quoteCommand(binary, args string) string {
	b := binary
	if strings.ContainsAny(b, " \t") {
		b = `"` + b + `"`
	}
	return b + " " + args
}

// isDownshiftCommand reports whether command runs `downshift <args>` through
// any binary path, so a stale path can be replaced.
func isDownshiftCommand(command, args string) bool {
	c := strings.TrimSpace(command)
	var bin, rest string
	if strings.HasPrefix(c, `"`) {
		end := strings.Index(c[1:], `"`)
		if end < 0 {
			return false
		}
		bin, rest = c[1:1+end], c[2+end:]
	} else if i := strings.IndexAny(c, " \t"); i >= 0 {
		bin, rest = c[:i], c[i:]
	} else {
		bin = c
	}
	base := strings.TrimSuffix(filepath.Base(strings.ReplaceAll(bin, `\`, "/")), ".exe")
	return base == "downshift" && strings.Join(strings.Fields(rest), " ") == strings.Join(strings.Fields(args), " ")
}

func asObj(v any) (*jobj, bool) { o, ok := v.(*jobj); return o, ok }

// mergeTarget applies every hook (and the status line) of t to doc in place.
// skipOptional leaves out the optional defaults (the main-session policy).
func mergeTarget(doc *jobj, t setupTarget, binary string, skipOptional bool) ([]string, error) {
	var notes []string
	for _, d := range t.Defaults {
		if skipOptional && d.Optional {
			continue
		}
		parent := doc
		if d.Parent != "" {
			pv, ok := doc.get(d.Parent)
			if !ok || pv == nil {
				next := newJobj()
				doc.set(d.Parent, next)
				parent = next
			} else if next, isObj := asObj(pv); isObj {
				parent = next
			} else {
				return nil, fmt.Errorf("%q is not a JSON object, left untouched", d.Parent)
			}
		}
		v, ok := parent.get(d.Key)
		if !ok {
			parent.set(d.Key, d.Value)
			continue
		}
		if d.Note != "" && fmt.Sprint(v) != fmt.Sprint(d.Value) {
			notes = append(notes, d.Note)
		}
	}
	holder := doc
	for _, key := range t.HooksPath {
		v, ok := holder.get(key)
		if !ok || v == nil {
			next := newJobj()
			holder.set(key, next)
			holder = next
			continue
		}
		next, isObj := asObj(v)
		if !isObj {
			return nil, fmt.Errorf("%q is not a JSON object, left untouched", key)
		}
		holder = next
	}
	for _, h := range t.Hooks {
		var list []any
		if v, ok := holder.get(h.Event); ok && v != nil {
			arr, isArr := v.([]any)
			if !isArr {
				return nil, fmt.Errorf("%q is not a JSON array, left untouched", h.Event)
			}
			list = arr
		}
		cmd := quoteCommand(binary, h.Args)
		if repointElsewhere(holder, h, cmd) {
			continue
		}
		if t.Nested {
			list = ensureNestedHook(list, h, cmd)
		} else {
			list = ensureFlatHook(list, h, cmd)
		}
		holder.set(h.Event, list)
	}
	if sl := t.StatusLine; sl != nil {
		note, err := ensureStatusLine(doc, *sl, binary)
		if err != nil {
			return nil, err
		}
		if note != "" {
			notes = append(notes, note)
		}
	}
	return notes, nil
}

// repointElsewhere finds a Downshift hook for h outside holder's own h.Event
// list, such as inside an Antigravity named group, and points it at cmd. It
// reports whether one was found, so setup does not add a duplicate.
func repointElsewhere(holder *jobj, h setupHook, cmd string) bool {
	found := false
	var walk func(v any, under bool)
	walk = func(v any, under bool) {
		switch x := v.(type) {
		case *jobj:
			if c, ok := x.vals["command"].(string); ok && under && isDownshiftCommand(c, h.Args) {
				if c != cmd {
					x.set("command", cmd)
				}
				found = true
			}
			for k, child := range x.vals {
				walk(child, under || k == h.Event)
			}
		case []any:
			for _, item := range x {
				walk(item, under)
			}
		}
	}
	for k, child := range holder.vals {
		if k != h.Event {
			walk(child, false)
		}
	}
	return found
}

func hookCommand(o *jobj) string {
	c, _ := o.vals["command"].(string)
	return c
}

func ensureNestedHook(list []any, h setupHook, cmd string) []any {
	for _, item := range list {
		group, ok := asObj(item)
		if !ok {
			continue
		}
		inner, _ := group.vals["hooks"].([]any)
		for j, hk := range inner {
			ho, ok := asObj(hk)
			if !ok || !isDownshiftCommand(hookCommand(ho), h.Args) {
				continue
			}
			if hookCommand(ho) != cmd {
				ho.set("command", cmd)
			}
			cur, _ := group.vals["matcher"].(string)
			if h.Matcher == "" || cur == h.Matcher {
				return list
			}
			if len(inner) == 1 {
				group.set("matcher", h.Matcher)
				return list
			}
			// The group also holds someone else's hook: leave it and move ours.
			rest := append(append([]any{}, inner[:j]...), inner[j+1:]...)
			group.set("hooks", rest)
			return append(list, newNestedGroup(h, cmd))
		}
	}
	return append(list, newNestedGroup(h, cmd))
}

func newNestedGroup(h setupHook, cmd string) *jobj {
	hk := newJobj()
	hk.set("type", "command")
	hk.set("command", cmd)
	g := newJobj()
	if h.Matcher != "" {
		g.set("matcher", h.Matcher)
	}
	g.set("hooks", []any{hk})
	return g
}

func ensureFlatHook(list []any, h setupHook, cmd string) []any {
	for _, item := range list {
		e, ok := asObj(item)
		if !ok || !isDownshiftCommand(hookCommand(e), h.Args) {
			continue
		}
		if hookCommand(e) != cmd {
			e.set("command", cmd)
		}
		if cur, _ := e.vals["matcher"].(string); h.Matcher != "" && cur != h.Matcher {
			e.set("matcher", h.Matcher)
		}
		return list
	}
	e := newJobj()
	if h.Matcher != "" {
		e.set("matcher", h.Matcher)
	}
	e.set("command", cmd)
	return append(list, e)
}

// ensureStatusLine sets the status line only when none exists. A Downshift
// status line pointing at another binary is repointed; anyone else's is left
// alone and the returned note says how to chain it.
func ensureStatusLine(doc *jobj, sl setupStatusLine, binary string) (string, error) {
	cmd := quoteCommand(binary, sl.Args)
	v, ok := doc.get(sl.Key)
	cur, isObj := asObj(v)
	empty := !ok || v == nil || (isObj && len(cur.keys) == 0) || (isObj && hookCommand(cur) == "" && cur.vals["type"] == nil)
	if empty {
		o := newJobj()
		o.set("type", "command")
		o.set("command", cmd)
		doc.set(sl.Key, o)
		return "", nil
	}
	if isObj && isDownshiftCommand(hookCommand(cur), sl.Args) {
		if hookCommand(cur) != cmd {
			cur.set("command", cmd)
		}
		return "", nil
	}
	existing := ""
	if isObj {
		existing = hookCommand(cur)
	}
	return fmt.Sprintf("%s already set (%s): left untouched. To keep Downshift's quota line, pipe its input to `%s` before rendering (docs/guides/quota.md).",
		sl.Key, existing, quoteCommand(binary, sl.ChainArgs)), nil
}
