#!/usr/bin/env python3
# Copyright (c) 2026 Tiago Vilas Boas.
# SPDX-License-Identifier: Apache-2.0
"""Compare `downshift compact -- <cmd>` with `rtk <cmd>` on the same commands.

Downshift by Tiago Vilas Boas - https://github.com/tiagovilasboas/downshift

Builds small fixtures in a work directory, runs each command raw, under rtk and
under downshift compact (CCR default, on, off), and reports output bytes, exit
codes and which facts of the raw output survive. It measures bytes, not model
tokens, and nothing here calls an LLM. The report is
docs/evidence/compact-vs-rtk-2026-11-07.md.

    python3 scripts/compact-vs-rtk.py --downshift /path/to/downshift \
        --rtk /path/to/rtk --work /path/to/empty-dir

Needs go, python3 with pytest, node/npm (vitest is installed into the work
directory), tsc, cargo and git on PATH. The work directory only ever receives
fixtures, tool state and captured outputs; the repository is read, never written.
"""
import argparse, json, os, re, shlex, shutil, statistics, subprocess, sys

REPS = 3
ARMS = ["raw", "rtk", "rtk-norecall", "ds-default", "ds-ccr-on", "ds-ccr-off"]


# ------------------------------------------------------------------ fixtures
def w(path, text):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w") as fh:
        fh.write(text)


def sh(argv, cwd, check=True, env=None):
    return subprocess.run(argv, cwd=cwd, check=check, env=env, capture_output=True, text=True)


GO_TEST_HEAD = 'package {p}\n\nimport (\n\t"fmt"\n\t"testing"\n)\n\n'


def build_fixtures(F, repo):
    # Go: 30 packages that pass (plain run)
    w(f"{F}/go30/go.mod", "module fixture/go30\n\ngo 1.22\n")
    for i in range(1, 31):
        w(f"{F}/go30/pkg{i:02d}/pkg_test.go",
          f"package pkg{i:02d}\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) {{}}\nfunc TestB(t *testing.T) {{}}\n")
    # Go: 3 packages, 40 logging subtests each (verbose pass)
    w(f"{F}/gopass/go.mod", "module fixture/gopass\n\ngo 1.22\n")
    for p in ("alpha", "beta", "gamma"):
        w(f"{F}/gopass/{p}/{p}_test.go", GO_TEST_HEAD.format(p=p) +
          'func TestTable(t *testing.T) {\n\tfor i := 0; i < 40; i++ {\n'
          '\t\tt.Run(fmt.Sprintf("case_%d", i), func(t *testing.T) {\n'
          f'\t\t\tt.Logf("checking case %d of package {p}", i)\n\t\t}})\n\t}}\n}}\n')
    # Go: assertion failures (calc) and a panic (boom)
    w(f"{F}/gofail/go.mod", "module fixture/gofail\n\ngo 1.22\n")
    w(f"{F}/gofail/calc/calc_test.go", GO_TEST_HEAD.format(p="calc") + '''func TestOK(t *testing.T) {
	for i := 0; i < 30; i++ {
		t.Run(fmt.Sprintf("ok_%d", i), func(t *testing.T) { _ = i })
	}
}

func TestAddWrong(t *testing.T) {
	if got := 2 + 2; got != 5 {
		t.Errorf("add: got %d, want 5", got)
	}
}

func TestSliceWrong(t *testing.T) {
	got := []int{1, 2, 3}
	if len(got) != 4 {
		t.Fatalf("slice: len %d, want 4", len(got))
	}
}

func TestSubFail(t *testing.T) {
	t.Run("inner_bad", func(t *testing.T) { t.Error("inner failure marker XYZ-77") })
}
''')
    w(f"{F}/gofail/boom/boom_test.go", 'package boom\n\nimport "testing"\n\n'
      'func TestBefore(t *testing.T) { t.Log("before panic") }\n\n'
      'func TestBoom(t *testing.T) {\n\tvar m map[string]int\n\tm["k"] = 1\n}\n')
    # Python
    w(f"{F}/pypass/test_ok.py", "import pytest\n\n@pytest.mark.parametrize('i', range(120))\ndef test_ok(i):\n    assert i >= 0\n")
    w(f"{F}/pyfail/test_bad.py", '''import pytest

@pytest.mark.parametrize("i", range(60))
def test_ok(i):
    assert i >= 0

def test_eq_wrong():
    data = {"a": 1, "b": [1, 2, 3]}
    assert data == {"a": 1, "b": [1, 2, 4]}

def test_raises_wrong():
    with pytest.raises(ValueError):
        int("12")

def test_zero_div():
    x = 0
    return 1 / x
''')
    # TypeScript
    w(f"{F}/ts/tsconfig.json", '{ "compilerOptions": { "strict": true, "noEmit": true, "target": "es2020", "module": "commonjs" }, "include": ["src"] }\n')
    w(f"{F}/ts/src/a.ts", '''export function add(a: number, b: number): number { return a + b }
const x: string = add(1, 2)
const y: number = "nope"
export const z: Array<number> = [1, "two", 3]
function f(p: { name: string }) { return p.nme }
''')
    # vitest (installed into the fixture)
    w(f"{F}/vt/package.json", '{ "name": "vtfix", "version": "1.0.0", "private": true }\n')
    w(f"{F}/vt/ok/ok.test.js", "import {test,expect} from 'vitest'\nfor (let i=0;i<80;i++) test('case '+i,()=>{expect(i).toBeGreaterThanOrEqual(0)})\n")
    w(f"{F}/vt/bad/bad.test.js", """import {test,expect} from 'vitest'
for (let i=0;i<40;i++) test('case '+i,()=>{expect(i).toBeGreaterThanOrEqual(0)})
test('sum wrong',()=>{expect(1+1).toBe(3)})
test('obj wrong',()=>{expect({a:1,b:[1,2]}).toEqual({a:1,b:[1,3]})})
test('throws',()=>{ throw new Error('vitest marker VT-9') })
""")
    sh(["npm", "install", "--no-audit", "--no-fund", "--silent", "vitest@2.1.9"], f"{F}/vt")
    # Rust
    tests = "".join(f"    #[test] fn t{i}() {{ assert_eq!({i} + 1, {i + 1}); }}\n" for i in range(60))
    for name in ("rspass", "rsfail"):
        w(f"{F}/{name}/Cargo.toml", f'[package]\nname = "{name}"\nversion = "0.1.0"\nedition = "2021"\n')
    w(f"{F}/rspass/src/lib.rs", "#[cfg(test)]\nmod tests {\n" + tests + "}\n")
    w(f"{F}/rsfail/src/lib.rs", "#[cfg(test)]\nmod tests {\n" + tests +
      '    #[test] fn bad_eq() { assert_eq!(2 + 2, 5, "arith marker RS-42"); }\n'
      '    #[test] fn bad_panic() { let v: Vec<i32> = vec![]; let _ = v[3]; }\n}\n')
    # git: real Go sources from this repository, then edits, a staged delete and add, untracked files
    g = f"{F}/gitrepo"
    os.makedirs(g)
    sh(["git", "init", "-q", "-b", "main", "."], g)
    sh(["git", "config", "user.email", "fixture@example.invalid"], g)
    sh(["git", "config", "user.name", "fixture"], g)
    files = "advise.go compact.go gate.go keep.go policy.go metrics.go folded_test.go hold_test.go".split()
    for f in files:
        shutil.copy(os.path.join(repo, "internal", "execwrap", f), g)
    sh(["git", "add", "-A"], g)
    sh(["git", "commit", "-q", "-m", "import sources"], g)
    for f in files:
        out = []
        for i, line in enumerate(open(f"{g}/{f}").read().split("\n")):
            k = i % 17
            if k == 5 and line.strip() and not line.strip().startswith("//"):
                out.append(line + " // reviewed")
            elif k == 9 and line.strip().startswith("return"):
                out.append(line.replace("return", "return /* checked */", 1))
            elif k == 12 and "err" in line:
                out.append(line.replace("err", "e", 1))
            else:
                out.append(line)
        w(f"{g}/{f}", "\n".join(out))
    for n in "notes.md run.log coverage.out tmp_trace.txt scratch.go todo.txt out.json draft.md bench.csv sample.yaml".split():
        w(f"{g}/{n}", "x\n")
    sh(["git", "rm", "-qf", "folded_test.go"], g)
    shutil.copy(os.path.join(repo, "internal", "execwrap", "keep_test.go"), g)
    sh(["git", "add", "keep_test.go"], g)


def cases(F, repo):
    return [
        ("go-test-pass",    f"{F}/go30",    ["go", "test", "-count=1", "./..."], "pass"),
        ("go-test-v-pass",  f"{F}/gopass",  ["go", "test", "-count=1", "-v", "./..."], "pass"),
        ("go-test-fail",    f"{F}/gofail",  ["go", "test", "-count=1", "./calc"], "fail"),
        ("go-test-v-fail",  f"{F}/gofail",  ["go", "test", "-count=1", "-v", "./calc"], "fail"),
        ("go-test-panic",   f"{F}/gofail",  ["go", "test", "-count=1", "./boom"], "fail"),
        ("pytest-pass",     f"{F}/pypass",  ["pytest"], "pass"),
        ("pytest-v-pass",   f"{F}/pypass",  ["pytest", "-v"], "pass"),
        ("pytest-fail",     f"{F}/pyfail",  ["pytest"], "fail"),
        ("pytest-v-fail",   f"{F}/pyfail",  ["pytest", "-v"], "fail"),
        ("vitest-pass",     f"{F}/vt",      ["npx", "vitest", "run", "ok"], "pass"),
        ("vitest-fail",     f"{F}/vt",      ["npx", "vitest", "run", "bad"], "fail"),
        ("tsc-fail",        f"{F}/ts",      ["tsc", "-p", "."], "fail"),
        ("cargo-test-pass", f"{F}/rspass",  ["cargo", "test", "--offline"], "pass"),
        ("cargo-test-fail", f"{F}/rsfail",  ["cargo", "test", "--offline"], "fail"),
        ("git-status",      f"{F}/gitrepo", ["git", "status"], "list"),
        ("git-diff",        f"{F}/gitrepo", ["git", "diff"], "list"),
        ("git-log-50",      repo,           ["git", "log", "-n", "50"], "list"),
        ("ls-la",           repo,           ["ls", "-la"], "list"),
        ("find-go",         repo,           ["find", "internal/execwrap", "-name", "*.go"], "list"),
        ("grep-func",       repo,           ["grep", "-rn", "func Test", "internal/execwrap"], "list"),
    ]


# ------------------------------------------------------------------ arms
class Ctx:
    pass


def arm_env(c, arm):
    e = {k: v for k, v in os.environ.items() if not k.startswith(("DOWNSHIFT_", "RTK_"))}
    e["NO_COLOR"] = "1"
    e["CI"] = "1"
    e.pop("RUST_BACKTRACE", None)  # cargo's default; a set value inflates every arm alike
    e["RTK_TELEMETRY_DISABLED"] = "1"
    sd = f"{c.state}/{arm}"
    if arm.startswith("ds"):
        e["DOWNSHIFT_STATE_DIR"] = sd
        if arm == "ds-ccr-on":
            e["DOWNSHIFT_CCR"] = "1"
        elif arm == "ds-ccr-off":
            e["DOWNSHIFT_CCR"] = "0"
    if arm.startswith("rtk"):
        e["RTK_DB_PATH"] = f"{sd}/db.sqlite"
        e["RTK_CONFIG_DIR"] = f"{sd}/cfg"
        e["RTK_TEE_DIR"] = f"{sd}/tee"
        if arm == "rtk-norecall":
            e["RTK_RECALL"] = "0"
    return e


def argv_for(c, arm, argv, env):
    if arm == "raw":
        return argv
    if arm.startswith("rtk"):  # the command rtk's own hook rewriter would run
        r = subprocess.run([c.rtk, "rewrite", " ".join(shlex.quote(a) for a in argv)],
                           env=env, capture_output=True, text=True)
        if not r.stdout.strip():
            sys.exit(f"rtk has no rewrite for {argv}")
        return [c.rtk] + shlex.split(r.stdout.strip())[1:]
    return [c.ds, "compact", "--"] + argv


def run(c, arm, cwd, argv):
    env = arm_env(c, arm)
    a = argv_for(c, arm, argv, env)
    p = subprocess.run(a, cwd=cwd, env=env, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    return a, p.returncode, p.stdout


# ------------------------------------------------------------------ facts
def uniq(xs):
    seen, out = set(), []
    for x in xs:
        if x not in seen:
            seen.add(x)
            out.append(x)
    return out


def facts_for(cid, raw, kind):
    """[(label, needle)] read from the raw output; needle is a str or ('re', pattern)."""
    t = raw.decode("utf-8", "replace")
    L = t.splitlines()
    f = []
    if cid.startswith("go-test"):
        for l in L:
            m = re.match(r"\s*--- FAIL: (\S+)", l)
            if m: f.append((f"failing test {m.group(1)}", m.group(1)))
            m = re.match(r"\s*(\w+_test\.go:\d+): (.*)", l)
            if m and kind == "fail":
                f.append((f"assert site {m.group(1)}", m.group(1)))
                f.append((f"assert msg '{m.group(2)[:40]}'", m.group(2)))
            m = re.match(r"^(panic: .*?)( \[recovered\])?$", l)
            if m: f.append((f"panic '{m.group(1)}'", m.group(1)))
            m = re.match(r"^(FAIL|ok)\s+(\S+)", l)
            if m:
                seg = m.group(2).split("/")[-1]
                f.append((f"package {m.group(1)} {seg}", ("re", rf"(?<![\w.-]){seg}(?![\w.-])")))
            m = re.search(r"(\w+_test\.go:\d+) \+0x", l)  # frame in the code under test
            if m: f.append((f"panic frame {m.group(1)}", m.group(1)))
    elif cid.startswith("pytest") and kind == "pass":
        for m in re.finditer(r"(\d+) passed", t):
            f.append((f"count {m.group(0)}", m.group(0)))
    elif cid.startswith("pytest"):
        for l in L:
            m = re.match(r"FAILED (\S+) - ", l)
            if m: f.append((f"failing test {m.group(1)}", m.group(1).split("::")[-1]))
            m = re.match(r"E\s+(\S.*)", l)
            if m: f.append((f"error line '{m.group(1)[:40]}'", m.group(1).rstrip()))
            m = re.match(r"(\w+\.py:\d+): ", l)
            if m: f.append((f"fail site {m.group(1)}", m.group(1)))
        for m in re.finditer(r"(\d+) (failed|passed)", L[-1] if L else ""):
            f.append((f"count {m.group(0)}", m.group(0)))
    elif cid.startswith("vitest"):
        for l in L:
            m = re.match(r"\s*FAIL\s+\S+\s+>\s+(.*)", l)
            if m: f.append((f"failing test '{m.group(1).strip()}'", m.group(1).strip()))
            m = re.match(r"\s*((?:Assertion)?Error: .*)", l)
            if m: f.append((f"error '{m.group(1)[:45]}'", m.group(1).rstrip()))
            m = re.match(r"\s*❯ (\S+:\d+:\d+)", l)
            if m: f.append((f"fail site {m.group(1)}", m.group(1)))
            m = re.search(r"Tests\s+(.*)", l)
            if m:
                for n in re.findall(r"\d+ (?:failed|passed)", m.group(1)):
                    f.append((f"count {n}", ("re", rf"(?<!\d){n.split()[0]}(?!\d)")))
    elif cid.startswith("tsc"):
        for l in L:
            m = re.match(r"(\S+)\((\d+),(\d+)\): error (TS\d+): (.*)", l)
            if m:
                f.append((f"diag {m.group(1)}({m.group(2)},{m.group(3)}) {m.group(4)}",
                          ("re", rf"(?<!\d){m.group(2)}[,:]{m.group(3)}(?!\d)")))
                f.append((f"msg {m.group(4)} '{m.group(5)[:35]}'", m.group(5)))
    elif cid.startswith("cargo"):
        for l in L:
            m = re.match(r"test (\S+) \.\.\. FAILED", l)
            if m: f.append((f"failing test {m.group(1)}", m.group(1)))
            m = re.search(r"(src/\w+\.rs:\d+:\d+)", l)
            if m: f.append((f"panic site {m.group(1)}", m.group(1)))
        for needle, label in (("RS-42", "assert msg RS-42"), ("index out of bounds", "panic 'index out of bounds'"),
                              ("left: 4", "assert left: 4")):
            if needle in t: f.append((label, needle))
        m = re.search(r"test result: (\w+)\. (\d+) passed; (\d+) failed", t)
        if m:
            f.append((f"summary {m.group(2)} passed", f"{m.group(2)} passed"))
            if m.group(3) != "0": f.append((f"summary {m.group(3)} failed", f"{m.group(3)} failed"))
    elif cid == "git-status":
        for l in L:
            m = re.match(r"\t(?:(?:modified|deleted|new file):\s+)?(\S+)\s*$", l)
            if m: f.append((f"path {m.group(1)}", m.group(1)))
    elif cid == "git-diff":
        for l in L:
            m = re.match(r"diff --git a/(\S+) b/", l)
            if m: f.append((f"file {m.group(1)}", m.group(1)))
        for x in uniq([l[1:] for l in L if l.startswith("+") and not l.startswith("+++")]):
            f.append((f"+line {x[:30]}", x))
        for x in uniq([l[1:] for l in L if l.startswith("-") and not l.startswith("---")]):
            f.append((f"-line {x[:30]}", x))
    elif cid == "git-log-50":
        first = True
        for l in L:
            m = re.match(r"commit ([0-9a-f]{40})", l)
            if m:
                f.append((f"hash {m.group(1)[:7]}", m.group(1)[:7]))
                first = True
                continue
            m = re.match(r"Author:\s+(.*?) <", l)
            if m: f.append(("author name", m.group(1)))
            m = re.match(r"Date:\s+(\w{3} \w{3} \d+ [\d:]+ \d{4})", l)
            if m: f.append((f"commit date {m.group(1)}", m.group(1)))
            m = re.match(r"    (\S.*)", l)
            if m:
                f.append((f"{'subject' if first else 'body line'} '{m.group(1)[:30]}'", m.group(1)[:30]))
                first = False
    elif cid == "ls-la":
        first_file = None
        for l in L:
            p = l.split()
            if len(p) >= 9 and p[-1] not in (".", ".."):
                f.append((f"entry {p[-1]}", p[-1]))
                if first_file is None and l.startswith("-"): first_file = p
        f.append(("attr: rwx permission string", ("re", r"[d-][r-][w-][x-][r-][w-][x-][r-][w-][x-]")))
        f.append(("attr: owner name", "root"))
        f.append(("attr: modified time", ("re", r"\b\d\d:\d\d\b|\b(Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\b")))
        if first_file:
            f.append((f"attr: exact size of {first_file[-1]}", ("re", rf"(?<![\w.]){first_file[4]}(?![\w.])")))
    elif cid == "find-go":
        for l in L:
            f.append((f"path {os.path.basename(l)}", os.path.basename(l)))
    elif cid == "grep-func":
        for l in L:
            m = re.match(r"([^:]+):(\d+):(.*)", l)
            if m:
                f.append((f"match {os.path.basename(m.group(1))}:{m.group(2)}", ("re", rf"(?<!\d){m.group(2)}(?!\d)")))
                n = re.search(r"func (Test\w+)", m.group(3))
                if n: f.append((f"name {n.group(1)}", n.group(1)))
    seen, out = set(), []
    for lab, n in f:
        key = (lab, n if isinstance(n, str) else n[1])
        if key not in seen:
            seen.add(key)
            out.append((lab, n))
    return out


def has(needle, text):
    return needle in text if isinstance(needle, str) else re.search(needle[1], text) is not None


SIG = re.compile(r"fail|panic|error|assert|^E\s|✗|×|✖|FAIL", re.I)
NORM = [(re.compile(r"0x[0-9a-fA-F]+"), "0x"), (re.compile(r"\d+(\.\d+)?(ms|s)\b"), "T"),
        (re.compile(r"\d\d:\d\d:\d\d"), "HH"), (re.compile(r"\(\d{3,6}\)"), "(N)"),
        (re.compile(r"-[0-9a-f]{16}"), "-H")]


def norm(l):
    for rx, r in NORM:
        l = rx.sub(r, l)
    return l.strip()


def sig_lines(t):
    return uniq([norm(l) for l in t.splitlines() if l.strip() and SIG.search(l)])


def recall(c, text, env, cwd):
    ids, outs = [], []
    for h in re.findall(r"downshift retrieve ([0-9a-f]+)", text):
        p = subprocess.run([c.ds, "retrieve", h], env=env, cwd=cwd, capture_output=True)
        ids.append(h); outs.append(p.stdout.decode("utf-8", "replace"))
    for h in re.findall(r"rtk recall ([0-9a-f]+)", text):
        p = subprocess.run([c.rtk, "recall", h], env=env, cwd=cwd, capture_output=True)
        ids.append(h); outs.append((p.stdout + p.stderr).decode("utf-8", "replace"))
    return (ids, "\n".join(outs)) if ids else (None, None)


# ------------------------------------------------------------------ main
def version(argv):
    try:
        r = subprocess.run(argv, capture_output=True, text=True)
        return (r.stdout or r.stderr).strip().splitlines()[0]
    except Exception as exc:  # a missing tool is reported, not fatal
        return f"unavailable: {exc}"


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--downshift", required=True)
    ap.add_argument("--rtk", required=True)
    ap.add_argument("--work", required=True, help="empty directory for fixtures, state and captured output")
    ap.add_argument("--repo", default=os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
    a = ap.parse_args()
    c = Ctx()
    c.ds, c.rtk = os.path.abspath(a.downshift), os.path.abspath(a.rtk)
    work, repo = os.path.abspath(a.work), os.path.abspath(a.repo)
    if os.path.exists(work) and os.listdir(work):
        sys.exit("--work must be empty or absent")
    F, out_dir = f"{work}/fixtures", f"{work}/out"
    c.state = f"{work}/state"
    for a_ in ARMS:
        os.makedirs(f"{c.state}/{a_}")
    os.makedirs(out_dir)
    build_fixtures(F, repo)
    versions = {
        "downshift": version([c.ds, "version"]), "rtk": version([c.rtk, "--version"]),
        "go": version(["go", "version"]), "python": sys.version.split()[0],
        "pytest": version(["pytest", "--version"]), "node": version(["node", "--version"]),
        "vitest": "2.1.9 (pinned by the script)",
        "tsc": version(["tsc", "--version"]), "cargo": version(["cargo", "--version"]),
        "git": version(["git", "--version"]),
        "repo_head": sh(["git", "rev-parse", "HEAD"], repo).stdout.strip(),
    }
    results = []
    for cid, cwd, argv, kind in cases(F, repo):
        rec = {"id": cid, "cmd": " ".join(argv), "kind": kind, "arms": {}}
        os.makedirs(f"{out_dir}/{cid}")
        outs_first = {}
        for arm in ARMS:
            sizes, codes, first, used = [], [], None, None
            for _ in range(REPS):
                used, code, out = run(c, arm, cwd, argv)
                sizes.append(len(out)); codes.append(code)
                first = out if first is None else first
            outs_first[arm] = first
            open(f"{out_dir}/{cid}/{arm}.txt", "wb").write(first)
            rec["arms"][arm] = {"bytes": int(statistics.median(sizes)), "bytes_all": sizes, "exit": codes,
                                "argv": [os.path.basename(used[0])] + used[1:] if arm != "raw" else argv}
        raw = outs_first["raw"]
        facts = facts_for(cid, raw, kind)
        sig = sig_lines(raw.decode("utf-8", "replace"))
        rec["raw_facts"], rec["raw_sig_lines"] = len(facts), len(sig)
        rec["fact_matchers_hit_raw"] = sum(1 for _, n in facts if has(n, raw.decode("utf-8", "replace")))
        for arm in ARMS[1:]:
            txt = outs_first[arm].decode("utf-8", "replace")
            r = rec["arms"][arm]
            r["facts_kept"] = sum(1 for _, n in facts if has(n, txt))
            r["facts_lost"] = [lab for lab, n in facts if not has(n, txt)]
            ntxt = "\n".join(norm(l) for l in txt.splitlines())
            r["sig_kept"] = sum(1 for s in sig if s in ntxt)
            r["exit_matches_raw"] = all(x == rec["arms"]["raw"]["exit"][0] for x in r["exit"])
            ids, rtxt = recall(c, txt, arm_env(c, arm), cwd)
            if ids:
                r["recall_ids"] = ids
                r["recoverable_facts"] = sum(1 for _, n in facts if has(n, rtxt) or has(n, txt))
                r["recoverable_lost"] = [lab for lab, n in facts if not (has(n, rtxt) or has(n, txt))]
        results.append(rec)
        print(cid, {k: v["bytes"] for k, v in rec["arms"].items()}, flush=True)
    json.dump({"versions": versions, "results": results}, open(f"{work}/results.json", "w"), indent=1)
    print(json.dumps(versions, indent=1))


if __name__ == "__main__":
    main()
