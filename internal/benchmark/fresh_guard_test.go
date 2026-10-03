// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1
// Commercial use requires a licence — see LICENSE for terms.

package benchmark_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/tiagovilasboas/harness-downshift/internal/benchmark"
)

// benchmark/fresh.json, benchmark/heldout2.json and benchmark/blind-vitrine.json
// are evaluation-only splits: never used to write or tune signals. These tests
// keep them honest.

// evalOnlySplits are the evaluation-only files under benchmark/.
var evalOnlySplits = []string{"fresh.json", "heldout2.json", "blind-vitrine.json"}

// minSplitSize is the smallest acceptable size per split. blind-vitrine is an
// externally authored set of 60 (20 per tier label); the others are 100+.
var minSplitSize = map[string]int{"blind-vitrine.json": 60}

const repoRoot = "../.."

var wordRe = regexp.MustCompile(`[a-z0-9]+`)

func tokenSet(s string) map[string]bool {
	set := map[string]bool{}
	for _, w := range wordRe.FindAllString(strings.ToLower(s), -1) {
		set[w] = true
	}
	return set
}

func jaccard(a, b map[string]bool) float64 {
	inter := 0
	for w := range a {
		if b[w] {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

func loadSplit(t *testing.T, name string) []benchmark.Task {
	t.Helper()
	tasks, err := benchmark.LoadDataset(filepath.Join(repoRoot, "benchmark", name))
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return tasks
}

// Fresh must be big enough to report, balanced, and not templated.
func TestFresh_ShapeAndNotTemplated(t *testing.T) {
	for _, name := range evalOnlySplits {
		checkShape(t, name, loadSplit(t, name))
	}
}

func checkShape(t *testing.T, name string, tasks []benchmark.Task) {
	want := 100
	if n, ok := minSplitSize[name]; ok {
		want = n
	}
	if len(tasks) < want {
		t.Fatalf("%s has %d tasks, want >= %d", name, len(tasks), want)
	}
	byLabel := map[string][]string{}
	for _, tk := range tasks {
		byLabel[tk.Label] = append(byLabel[tk.Label], tk.Prompt)
	}
	for label, prompts := range byLabel {
		if len(prompts) < 20 {
			t.Errorf("%s %s: %d tasks, want >= 20", name, label, len(prompts))
		}
		prefixes := map[string]int{}
		for _, p := range prompts {
			w := strings.Fields(strings.ToLower(p))
			if len(w) > 4 {
				w = w[:4]
			}
			prefixes[strings.Join(w, " ")]++
		}
		for prefix, n := range prefixes {
			if float64(n) > 0.2*float64(len(prompts)) {
				t.Errorf("%s %s: %d/%d prompts share the prefix %q (templated)", name, label, n, len(prompts), prefix)
			}
		}
	}
}

// No fresh prompt may appear (exactly or as a near-duplicate) in the seed,
// the regression set, or any classifier/semantic test case.
func TestFresh_NoLeakage(t *testing.T) {
	var fresh []benchmark.Task
	for _, name := range evalOnlySplits {
		fresh = append(fresh, loadSplit(t, name)...)
	}

	var others []string
	for _, name := range []string{"tasks.json", "holdout.json"} {
		ds, err := benchmark.LoadDataset(filepath.Join(repoRoot, "benchmark", name))
		if err != nil {
			t.Fatal(err)
		}
		for _, tk := range ds {
			others = append(others, tk.Prompt)
		}
	}
	for _, dir := range []string{"internal/core", "internal/semantic", "cmd/downshift"} {
		others = append(others, testStringLiterals(t, filepath.Join(repoRoot, dir))...)
	}

	for _, f := range fresh {
		fs := tokenSet(f.Prompt)
		for _, o := range others {
			if j := jaccard(fs, tokenSet(o)); j >= 0.6 {
				t.Errorf("fresh prompt %q leaks into training/tuning data as %q (jaccard %.2f)", f.Prompt, o, j)
			}
		}
	}
}

// testStringLiterals returns string literals of 4+ words from _test.go files.
func testStringLiterals(t *testing.T, dir string) []string {
	t.Helper()
	files, _ := filepath.Glob(filepath.Join(dir, "*_test.go"))
	var out []string
	for _, path := range files {
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				if s, err := strconv.Unquote(lit.Value); err == nil && len(strings.Fields(s)) >= 4 {
					out = append(out, s)
				}
			}
			return true
		})
	}
	return out
}

// Only CI (report step), docs and this guard may name fresh.json. Tuning
// code (Go sources, tools/, scripts other than the guard) must never read it.
func TestFresh_NotReadByTuningCode(t *testing.T) {
	allowed := map[string]bool{
		".github/workflows/ci.yml":               true,
		"internal/benchmark/fresh_guard_test.go": true,
		"scripts/fresh-guard.sh":                 true,
	}
	err := filepath.WalkDir(repoRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(repoRoot, path)
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", ".venv", "venv", "__pycache__", "dist":
				return filepath.SkipDir
			}
			if rel == "web" {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() { // symlinks (e.g. .venv/lib64), sockets
			return nil
		}
		if allowed[rel] || strings.HasSuffix(rel, ".md") || isEvalOnlySplit(rel) {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, name := range evalOnlySplits {
			if strings.Contains(string(b), name) {
				t.Errorf("%s references %s; that split is evaluation-only", rel, name)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestFreshGuardScript(t *testing.T) {
	cases := []struct {
		changed string
		wantErr bool
	}{
		{"benchmark/fresh.json\nREADME.md\n", false},
		{"internal/core/signals.go\n", false},
		{"benchmark/fresh.json\ninternal/core/signals.go\n", true},
		{"benchmark/fresh.json\ninternal/semantic/data/minilm.json\n", true},
		{"tools/minilm/train_prototypes.py\nbenchmark/fresh.json\n", true},
		{"benchmark/heldout2.json\ndocs/design/router-generalization.md\n", false},
		{"benchmark/heldout2.json\ninternal/core/classifier.go\n", true},
		{"tools/baseline/nb_tier.py\nbenchmark/heldout2.json\n", true},
		{"benchmark/blind-vitrine.json\nbenchmark/blind-vitrine.README.md\n", false},
		{"benchmark/blind-vitrine.json\ninternal/core/signals.go\n", true},
		{"tools/baseline/nb_tier.py\nbenchmark/blind-vitrine.json\n", true},
		{"internal/nbtier/train.json\nbenchmark/fresh.json\n", true},
	}
	for _, tc := range cases {
		cmd := exec.Command("bash", filepath.Join(repoRoot, "scripts", "fresh-guard.sh"))
		cmd.Stdin = strings.NewReader(tc.changed)
		err := cmd.Run()
		if (err != nil) != tc.wantErr {
			t.Errorf("changed=%q: err=%v, wantErr=%t", tc.changed, err, tc.wantErr)
		}
	}
}

func isEvalOnlySplit(rel string) bool {
	for _, name := range evalOnlySplits {
		if rel == "benchmark/"+name {
			return true
		}
	}
	return false
}
