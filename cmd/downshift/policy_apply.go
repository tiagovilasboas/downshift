// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/policyfile"
	"github.com/tiagovilasboas/downshift/internal/routing/contract"
	"github.com/tiagovilasboas/downshift/internal/routing/policy"
)

const policyApplyUsage = `usage: downshift policy apply --harness claude-code [--dry-run] [--yes]

Compile routing policy profiles into harness-native agent configurations.
Currently supported harnesses: claude-code (writes .claude/agents/<name>.md).
  --harness   target harness (required: claude-code)
  --dry-run   show what would be written, write nothing
  --yes       do not ask before writing
`

var profileNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$`)

type policyApplyOptions struct {
	Harness, Dir, Home string
	DryRun, Yes        bool
	In                 io.Reader
	Out, Err           io.Writer
	Now                func() time.Time
}

type agentPlan struct {
	name, path, shown, after, modelID string
	before                            []byte
	existed                           bool
}

func runPolicyApply(args []string, in io.Reader, stdout, stderr io.Writer) int {
	opts := policyApplyOptions{In: in, Out: stdout, Err: stderr, Now: time.Now}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--harness" && i+1 < len(args):
			i++
			opts.Harness = args[i]
		case strings.HasPrefix(a, "--harness="):
			opts.Harness = strings.TrimPrefix(a, "--harness=")
		case a == "--dry-run":
			opts.DryRun = true
		case a == "--yes" || a == "-y":
			opts.Yes = true
		case a == "-h" || a == "--help":
			fmt.Fprint(stdout, policyApplyUsage)
			return 0
		default:
			fmt.Fprintf(stderr, "policy apply: unknown or missing argument %q\n%s", a, policyApplyUsage)
			return 2
		}
	}
	if opts.Harness == "" {
		fmt.Fprintf(stderr, "policy apply: --harness is required (supported: claude-code)\n%s", policyApplyUsage)
		return 2
	}
	if opts.Harness != "claude-code" {
		fmt.Fprintf(stderr, "policy apply: unsupported harness %q (supported: claude-code)\n", opts.Harness)
		return 2
	}
	var err error
	if opts.Dir, err = os.Getwd(); err != nil {
		fmt.Fprintf(stderr, "policy apply: cannot determine working directory: %v\n", err)
		return 1
	}
	if opts.Home, err = os.UserHomeDir(); err != nil {
		fmt.Fprintf(stderr, "policy apply: cannot determine home directory: %v\n", err)
		return 1
	}
	return executePolicyApply(opts)
}

func executePolicyApply(opts policyApplyOptions) int {
	errf := func(format string, a ...any) {
		if opts.Err != nil {
			fmt.Fprintf(opts.Err, format, a...)
		}
	}
	project, _, err := policyfile.Find(opts.Dir)
	if err != nil {
		errf("policy apply: %v\n", err)
		return 1
	}
	user := ""
	if opts.Home != "" {
		user = filepath.Join(opts.Home, ".downshift", policy.FileName)
	} else if u, err := policy.UserPath(); err == nil {
		user = u
	}
	profiles := loadApplyProfiles(project, user)

	cat := catalog.Load()
	var plans []agentPlan

	for _, profile := range profiles {
		pName := profile.Name
		if profile.Harness != "" && profile.Harness != opts.Harness {
			continue
		}
		if !profileNameRegex.MatchString(pName) || strings.Contains(pName, "..") || strings.ContainsAny(pName, "/\\") {
			errf("policy apply: skipping profile %q: invalid name\n", pName)
			continue
		}
		coreTier, ok := profile.Tier.Core()
		if !ok {
			errf("policy apply: profile %q: unknown tier %q\n", pName, profile.Tier)
			continue
		}
		m := cat.ModelFor(opts.Harness, coreTier)
		if m.ID == "" {
			errf("policy apply: profile %q: unknown tier %q\n", pName, profile.Tier)
			continue
		}

		targetPath := filepath.Join(opts.Dir, ".claude", "agents", pName+".md")
		if userPath := filepath.Join(opts.Home, ".claude", "agents", pName+".md"); !exists(targetPath) && exists(userPath) {
			targetPath = userPath
		}
		if real, err := filepath.EvalSymlinks(targetPath); err == nil {
			targetPath = real
		} else if parentReal, err := filepath.EvalSymlinks(filepath.Dir(targetPath)); err == nil {
			targetPath = filepath.Join(parentReal, filepath.Base(targetPath))
		}

		raw, err := os.ReadFile(targetPath)
		existed := !errors.Is(err, fs.ErrNotExist)
		if existed && err != nil {
			errf("policy apply: cannot read %s: %v\n", targetPath, err)
			return 1
		}

		updated := updateFrontmatter(string(raw), pName, m.ID)
		shown := setupDisplay(opts.Home, targetPath)
		if rel, err := filepath.Rel(opts.Dir, targetPath); err == nil && !strings.HasPrefix(rel, "..") {
			shown = filepath.ToSlash(rel)
		}

		if existed && string(raw) == updated {
			if opts.Out != nil {
				fmt.Fprintf(opts.Out, "%s: already up to date (%s)\n", pName, shown)
			}
			continue
		}

		plans = append(plans, agentPlan{
			name: pName, path: targetPath, shown: shown,
			before: raw, after: updated, existed: existed, modelID: m.ID,
		})
	}

	if len(plans) == 0 {
		return 0
	}

	verb := "will update"
	if opts.DryRun {
		verb = "would update"
	}
	for _, plan := range plans {
		if opts.Out != nil {
			fmt.Fprintf(opts.Out, "%s: %s %s (model: %s)\n", plan.name, verb, plan.shown, plan.modelID)
			fmt.Fprint(opts.Out, unifiedDiff("a/"+plan.shown, "b/"+plan.shown, string(plan.before), plan.after))
			fmt.Fprintln(opts.Out)
		}
	}
	if opts.DryRun {
		if opts.Out != nil {
			fmt.Fprintln(opts.Out, "Dry run: nothing was written.")
		}
		return 0
	}

	if !setupConfirm(setupOptions{In: opts.In, Out: opts.Out, Yes: opts.Yes}, "Apply changes? [y/N] ") {
		if opts.Out != nil {
			fmt.Fprintln(opts.Out, "Not applied. Re-run with --yes to skip this question.")
		}
		return 1
	}

	now := opts.Now
	if now == nil {
		now = time.Now
	}
	stamp := now().UTC().Format("20060102T150405.000000")

	for _, plan := range plans {
		if plan.existed {
			bak := plan.path + setupBackupInfix + stamp
			if err := os.WriteFile(bak, plan.before, 0o600); err != nil {
				errf("%s: backup failed: %v\n", plan.name, err)
				return 1
			}
		}
		if err := os.MkdirAll(filepath.Dir(plan.path), 0o755); err != nil {
			errf("%s: cannot create directory: %v\n", plan.name, err)
			return 1
		}
		if err := atomicWriteFile(plan.path, []byte(plan.after), 0o644); err != nil {
			errf("%s: write failed: %v\n", plan.name, err)
			return 1
		}
		if opts.Out != nil {
			fmt.Fprintf(opts.Out, "%s: applied to %s (model: %s)\n", plan.name, plan.shown, plan.modelID)
		}
	}
	return 0
}

func updateFrontmatter(content, name, modelID string) string {
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "---" {
				modelIdx := -1
				for j := 1; j < i; j++ {
					if strings.HasPrefix(strings.TrimSpace(lines[j]), "model:") {
						modelIdx = j
						break
					}
				}
				if modelIdx >= 0 {
					lines[modelIdx] = "model: " + modelID
				} else {
					lines = append(lines[:i], append([]string{"model: " + modelID}, lines[i:]...)...)
				}
				return strings.Join(lines, "\n")
			}
		}
	}
	var sb strings.Builder
	sb.WriteString("---\nname: " + name + "\nmodel: " + modelID + "\n---\n")
	if t := strings.TrimSpace(content); t != "" {
		sb.WriteString("\n" + t + "\n")
	}
	return sb.String()
}

type applyProfile struct {
	Name, Harness string
	Tier          contract.Tier
}

func loadApplyProfiles(project, user string) []applyProfile {
	var out []applyProfile
	if p, err := policy.Load(policy.Locations{Project: project, User: user}); err == nil && len(p.Profiles) > 0 {
		for _, pr := range p.Profiles {
			out = append(out, applyProfile{Name: string(pr.Name), Tier: pr.Tier, Harness: string(pr.Harness)})
		}
		return out
	}
	type rawDoc struct {
		Name, Tier, Harness string
	}
	for _, path := range []string{project, user} {
		if path == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var doc struct {
			Profiles []rawDoc `json:"profiles"`
			Routing  struct {
				Profiles []rawDoc `json:"profiles"`
			} `json:"routing"`
		}
		if err := json.Unmarshal(data, &doc); err == nil {
			for _, p := range append(doc.Profiles, doc.Routing.Profiles...) {
				out = append(out, applyProfile{Name: p.Name, Tier: contract.Tier(p.Tier), Harness: p.Harness})
			}
		}
	}
	return out
}
