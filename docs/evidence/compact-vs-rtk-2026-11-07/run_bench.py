#!/usr/bin/env python3
"""Compare `rtk <cmd>` with `downshift compact -- <cmd>` on the same commands.

Downshift by Tiago Vilas Boas - https://github.com/tiagovilasboas/downshift
Output seen by the agent = stdout and stderr merged, as the Claude Code Bash tool shows it.
Writes every captured output to OUT and a JSON result to OUT/results.json.
Usage: run_bench.py <scratch> <rtk-bin> <downshift-bin>
"""
import json, os, re, shlex, subprocess, sys, shutil, time

S, RTK, DS = sys.argv[1], sys.argv[2], sys.argv[3]
F = os.path.join(S, "fix")
OUT = os.path.join(S, "out")
shutil.rmtree(OUT, ignore_errors=True)
os.makedirs(OUT)
for t in ("raw", "rtk", "ds"):  # start every tool from an empty home and state dir
    shutil.rmtree(os.path.join(S, "home-" + t), ignore_errors=True)

REAL_HOME = os.path.expanduser("~")


def env_for(tool, extra=None):
    home = os.path.join(S, "home-" + tool)
    os.makedirs(home, exist_ok=True)
    e = dict(os.environ)
    e.update(
        HOME=home, USERPROFILE=home,
        XDG_DATA_HOME=os.path.join(home, ".local/share"),
        XDG_CONFIG_HOME=os.path.join(home, ".config"),
        XDG_CACHE_HOME=os.path.join(home, ".cache"),
        DOWNSHIFT_STATE_DIR=os.path.join(home, ".downshift"),
        GOCACHE=subprocess.run(["go", "env", "GOCACHE"], capture_output=True, text=True).stdout.strip(),
        GOMODCACHE=subprocess.run(["go", "env", "GOMODCACHE"], capture_output=True, text=True).stdout.strip(),
        GOPATH=subprocess.run(["go", "env", "GOPATH"], capture_output=True, text=True).stdout.strip(),
        CARGO_HOME=os.path.join(REAL_HOME, ".cargo"),
        RUSTUP_HOME=os.path.join(REAL_HOME, ".rustup"),
        NO_COLOR="1", CI="", TERM="dumb",
        RTK_TELEMETRY_DISABLED="1",
        PATH=os.path.join(F, "ts/node_modules/.bin") + os.pathsep + os.environ["PATH"],
    )
    e.pop("DOWNSHIFT_CCR", None)
    e.pop("DOWNSHIFT_EXEC", None)
    if extra:
        e.update(extra)
    return e


def run(argv, cwd, tool, extra=None):
    t0 = time.time()
    p = subprocess.run(argv, cwd=cwd, env=env_for(tool, extra), stdout=subprocess.PIPE,
                       stderr=subprocess.STDOUT, timeout=300)
    return p.returncode, p.stdout, time.time() - t0


NORM = [
    (re.compile(r"\b\d+(\.\d+)?(ms|s)\b"), "T"),       # timings
    (re.compile(r"\(\d+\.\d+s\)"), "(T)"),
    (re.compile(r"0x[0-9a-f]+"), "0xH"),
    (re.compile(r"goroutine \d+"), "goroutine N"),
    (re.compile(r"\(\d{3,6}\)"), "(PID)"),
    (re.compile(r"\s+"), " "),
]


def norm(s):
    for rx, rep in NORM:
        s = rx.sub(rep, s)
    return s.strip()


def lines(raw, pat):
    rx = re.compile(pat)
    return sorted({norm(l) for l in raw.splitlines() if rx.search(l) and l.strip()})


def tokens_present(raw, toks):
    return list(toks)


# Each case: name, cwd (relative to F), argv, rtk argv (after the rtk binary), how to pick facts.
# facts(raw_text) returns the strings that must still be readable in the compacted output.
GO_FAIL = r"--- FAIL|^FAIL|^panic:|_test\.go:\d+|exit status|want \d+|zero case broken|index out of range"
PY_FAIL = r"^FAILED |^E  |^_{3,} test_|^\d+ failed|assert "
CARGO_FAIL = r"panicked at|FAILED|failures:|assertion|left:|right:|test result: FAILED|multiplication of"
TSC_FAIL = r"error TS"
VT_FAIL = r"FAIL|AssertionError|Error:|expected|Tests |adds correctly|fetches config|config missing"
GIT_FILES = r"README\.md|store\.go|CHANGELOG|untracked"


def facts_regex(pat):
    return lambda raw: lines(raw, pat)


def facts_tokens(toks):
    return lambda raw: [t for t in toks if t in raw]


def facts_needles(toks):
    # semantic needles taken from how the fixtures were built; a needle that is not in the raw output
    # is a harness bug, so it is reported instead of silently dropped
    def f(raw):
        missing = [t for t in toks if t not in raw]
        assert not missing, f"needle not in raw output: {missing}"
        return list(toks)
    return f


def facts_count(n):
    # a pass run must still state the count of what passed
    return lambda raw: [n]


def facts_git_log(raw):
    # every short hash and subject of the 30 commits (git log default format)
    out = []
    for m in re.finditer(r"^commit ([0-9a-f]{40})\n(?:.*\n)*?\n\s{4}(.+)$", raw, re.M):
        out.append(m.group(1)[:7])
        out.append(norm(m.group(2)))
    return out


def facts_changed_lines(raw):
    return sorted({norm(l) for l in raw.splitlines()
                   if (l.startswith("+") or l.startswith("-")) and not l.startswith(("+++", "---"))})


def facts_files_matching(pat):
    def f(raw):
        return sorted({m.group(0) for m in re.finditer(pat, raw)})
    return f


CASES = [
    # name, cwd, argv, rtk argv, expected exit, facts, kind
    ("go-test-pass-12pkg", "gopass", ["go", "test", "-count=1", "./..."], ["go", "test", "-count=1", "./..."], 0,
     facts_count("12"), "pass"),
    ("go-test-fail", "gofail", ["go", "test", "-count=1", "./..."], ["go", "test", "-count=1", "./..."], 1,
     facts_needles(["failpkg","panicpkg","TestMulTwoByThree","TestMulFourByFive","TestMulZero","TestIdxOutOfRange","Mul(2,3) = 5, want 6","Mul(4,5) = 9, want 20","zero case broken","index out of range [7] with length 3","f_test.go:7","f_test.go:13","f_test.go:19","p.go:3","p_test.go:6"]), "fail"),
    ("pytest-pass-40", "py", ["pytest", "-q", "test_pass.py"], ["pytest", "-q", "test_pass.py"], 0,
     facts_count("40"), "pass"),
    ("pytest-fail", "py", ["pytest", "-q", "test_fail.py"], ["pytest", "-q", "test_fail.py"], 1,
     facts_needles(["test_broken_1","test_broken_2","test_broken_3","test_raises","[2, 3, 4, 5, 11]","[2, 3, 4, 5, 12]","[2, 3, 4, 5, 13]","ValueError: config missing: db_url","test_fail.py:94","test_fail.py:98","test_fail.py:102","test_fail.py:105"]), "fail"),
    ("cargo-test-pass", "rust", ["cargo", "test"], ["cargo", "test"], 0,
     facts_count("12"), "pass"),
    ("cargo-test-fail", "rustfail", ["cargo", "test"], ["cargo", "test"], 101,
     facts_needles(["idx_panics","mul_works","mul_zero","multiplication of 3 by 4","left: 7","right: 12","left: 5","right: 0","index out of bounds: the len is 3 but the index is 7","tests/fail.rs:2","tests/fail.rs:3","tests/fail.rs:4"]), "fail"),
    ("tsc-errors", "ts/bad", ["tsc", "-p", "."], ["tsc", "-p", "."], 2,
     facts_needles(["TS2322","TS2304","TS2339","Cannot find name 'missing'","Property 'toUpperCase' does not exist on type 'number'","bad.ts(1,7)","bad.ts(2,40)","bad.ts(3,18)","bad.ts(4,41)"]), "fail"),
    ("vitest-fail", "ts", ["vitest", "run", "vt"], ["vitest", "run", "vt"], 1,
     facts_needles(["adds correctly","fetches config","expected 4 to be 5","config missing: api_key","fail.test.ts:2","fail.test.ts:3"]), "fail"),
    ("vitest-pass-12", "ts", ["vitest", "run", "vt/ok1.test.ts", "vt/ok2.test.ts", "vt/ok3.test.ts"],
     ["vitest", "run", "vt/ok1.test.ts", "vt/ok2.test.ts", "vt/ok3.test.ts"], 0, facts_count("3"), "pass"),
    ("git-status", "gitrepo", ["git", "status"], ["git", "status"], 0, facts_needles(["README.md","store.go","CHANGELOG.old.md","CHANGELOG.md","untracked1.txt","untracked2.txt"]), "info"),
    ("git-diff", "gitrepo", ["git", "diff"], ["git", "diff"], 0, facts_changed_lines, "info"),
    ("git-log-30", "gitrepo", ["git", "log", "-n", "30"], ["git", "log", "-n", "30"], 0, facts_git_log, "info"),
    ("ls-la-150", "many", ["ls", "-la"], ["ls", "-la"], 0,
     facts_files_matching(r"file\d+\.txt|sub"), "info"),
    ("find-md-40", "many", ["find", ".", "-name", "*.md"], ["find", ".", "-name", "*.md"], 0,
     facts_files_matching(r"n\d+\.md"), "info"),
    ("grep-todo", "many", ["grep", "-rn", "TODO", "."], ["grep", "-rn", "TODO", "."], 0,
     facts_files_matching(r"(?:file\d+\.txt|n\d+\.md)"), "info"),
]


def has(out_text, facts):
    o = norm(out_text)
    return [f for f in facts if (re.search(r"(?<![\w.])" + re.escape(f) + r"(?![\w.])", o) if (f.isdigit() and len(f) <= 3) else norm(f) in o)]


results = []
for name, cwd, argv, rtk_argv, want_rc, facts_fn, kind in CASES:
    d = os.path.join(F, cwd)
    row = dict(name=name, kind=kind, cwd=cwd, argv=argv)
    rc, raw, _ = run(argv, d, "raw")
    raw_t = raw.decode("utf-8", "replace")
    facts = facts_fn(raw_t)
    row.update(raw_bytes=len(raw), raw_lines=raw_t.count("\n"), raw_rc=rc, n_facts=len(facts))
    open(os.path.join(OUT, name + ".raw.txt"), "wb").write(raw)

    rw = subprocess.run([RTK, "rewrite", shlex.join(argv)], env=env_for("rtk"), capture_output=True, text=True).stdout.strip()
    row["rtk_rewrite"] = rw
    rtk_cmd = shlex.split(rw) if rw.startswith("rtk ") else argv
    rtk_cmd = [RTK] + rtk_cmd[1:] if rw.startswith("rtk ") else rtk_cmd
    variants = [
        ("rtk", rtk_cmd, None, "rtk"),
        ("ds_ccr_on", [DS, "compact", "--"] + argv, None, "ds"),
        ("ds_ccr_off", [DS, "compact", "--"] + argv, {"DOWNSHIFT_CCR": "0"}, "ds"),
    ]
    for label, a, extra, tool in variants:
        try:
            rc2, out, secs = run(a, d, tool, extra)
        except Exception as e:  # noqa
            row[label] = dict(error=str(e))
            continue
        open(os.path.join(OUT, f"{name}.{label}.txt"), "wb").write(out)
        t = out.decode("utf-8", "replace")
        kept = has(t, facts)
        retr = None
        mm = re.search(r"retrieve ([0-9a-f]{12})", t) if label == "ds_ccr_on" else re.search(r"rtk recall ([0-9a-f]+)", t) if label == "rtk" else None
        if mm:
            rcmd = [DS, "retrieve", mm.group(1)] if label == "ds_ccr_on" else [RTK, "recall", mm.group(1)]
            _, rtxt, _ = run(rcmd, d, tool, extra)
            rt = rtxt.decode("utf-8", "replace")
            retr = dict(bytes=len(rtxt), facts_kept=len(has(rt + t, facts)), exact=(rtxt == raw))
        row[label] = dict(retrievable=retr, bytes=len(out), lines=t.count("\n"), rc=rc2, rc_ok=(rc2 == rc),
                          facts_kept=len(kept), facts_total=len(facts),
                          missing=[f for f in facts if f not in kept][:12],
                          identical_to_raw=(out == raw),
                          secs=round(secs, 2))
    results.append(row)

json.dump(results, open(os.path.join(OUT, "results.json"), "w"), indent=1)
print(f"{'case':22} {'raw':>7} | {'rtk':>7} {'facts':>9} rc | {'ds+ccr':>7} {'facts':>9} rc | {'ds':>7} {'facts':>9} rc")
for r in results:
    def c(k):
        v = r.get(k, {})
        if "bytes" not in v:
            return f"{'ERR':>7} {'-':>9} -"
        rr = v.get("retrievable")
        extra = f" ->{rr['facts_kept']}" if rr else ""
        return f"{v['bytes']:>7} {str(v['facts_kept'])+'/'+str(v['facts_total']):>9} {'ok' if v['rc_ok'] else 'X'}{extra}"
    print(f"{r['name']:22} {r['raw_bytes']:>7} | {c('rtk')} | {c('ds_ccr_on')} | {c('ds_ccr_off')}")
