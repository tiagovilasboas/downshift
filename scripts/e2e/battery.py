#!/usr/bin/env python3
# Copyright (c) 2026 Tiago Vilas Boas.
# SPDX-License-Identifier: Apache-2.0
# Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift
"""End-to-end battery for a downshift binary, in a throwaway state dir.

Each check drives the real hook commands with Claude Code shaped payloads and
reads back the event log or the CLI JSON. Nothing touches ~/.downshift. Needs go and git on PATH for the compactor checks.
Run: make e2e  (or DOWNSHIFT_BIN=/path/to/downshift python3 -I scripts/e2e/battery.py)
"""
import hashlib, json, os, shutil, subprocess, sys, tempfile, uuid

REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
# DOWNSHIFT_BIN picks the binary under test; default is ./downshift at the repo root (make build).
BIN = os.path.abspath(os.environ.get("DOWNSHIFT_BIN", os.path.join(REPO, "downshift")))
results = []


def check(name, ok, detail=""):
    results.append((name, bool(ok), detail))


def run(args, payload=None, state=None, cwd=None, env_extra=None):
    env = dict(os.environ)
    env["DOWNSHIFT_STATE_DIR"] = state
    env.pop("CLAUDE_CODE_SESSION_ID", None)
    # The built-in control group leaves about 1 spawn in 10 unrouted on purpose; the battery checks routing, so it is off here.
    env["DOWNSHIFT_CONTROL_SHARE"] = "0"
    if env_extra:
        env.update(env_extra)
    p = subprocess.run([BIN] + args, input=json.dumps(payload) if payload is not None else None,
                       capture_output=True, text=True, env=env, cwd=cwd, timeout=180)
    return p


def events(state):
    path = os.path.join(state, "events.jsonl")
    if not os.path.exists(path):
        return []
    return [json.loads(l) for l in open(path) if l.strip()]


def session_hash(sid):
    return hashlib.sha256(("harness-downshift-session-v1:" + sid).encode()).hexdigest()


def pre(state, sid, prompt, model, tool_use):
    payload = {"session_id": sid, "tool_name": "Task", "tool_use_id": tool_use,
               "tool_input": {"prompt": prompt, "model": model, "subagent_type": "general-purpose", "description": "battery"}}
    p = run(["claude-code"], payload, state)
    try:
        out = json.loads(p.stdout or "{}")
    except json.JSONDecodeError:
        out = {}
    hso = out.get("hookSpecificOutput") or {}
    return (hso.get("updatedInput") or {}).get("model"), p


def post(state, sid, tool_use, agent_id, resolved):
    payload = {"session_id": sid, "tool_name": "Task", "tool_use_id": tool_use,
               "tool_input": {"prompt": "x", "subagent_type": "general-purpose"},
               "tool_response": {"status": "completed", "agentId": agent_id, "resolvedModel": resolved}}
    return run(["claude-code-post-tool-use"], payload, state)


def stop(state, sid, agent_id, model, usage):
    payload = {"session_id": sid, "agent_id": agent_id, "hook_event_name": "SubagentStop",
               "model": model, "usage": usage}
    return run(["claude-code-subagent-stop"], payload, state)


def panel(state, sid):
    p = run(["session", "--panel", f"--session={sid}"], None, state)
    try:
        return json.loads(p.stdout)
    except json.JSONDecodeError:
        return {}


def main():
    base = tempfile.mkdtemp(prefix="ds-battery-")
    try:
        # ---------- 1. Routing decisions on Claude Code ----------
        st = os.path.join(base, "routing"); os.makedirs(st)
        sid = "battery-" + uuid.uuid4().hex[:8]
        cases = [
            ("trivial on opus moves down", "rename the userId variable to userIdentifier", "claude-opus-5-5", lambda m: m in ("haiku", "claude-haiku-4-5")),
            ("trivial on sonnet moves down", "fix a typo in the README title", "claude-sonnet-5-5", lambda m: m in ("haiku", "claude-haiku-4-5")),
            ("trivial on haiku stays (floor)", "rename the userId variable to userIdentifier", "haiku", lambda m: m in (None, "haiku")),
            ("risky on opus is held", "rotate the production API secrets and update the payment webhook", "claude-opus-5-5", lambda m: m is None),
            ("auth change on opus is held", "change the login session token validation in the auth middleware", "claude-opus-5-5", lambda m: m is None),
            ("complex design on opus stays", "design a new multi-region replication strategy for the event store with failover and consistency guarantees", "claude-opus-5-5", lambda m: m in (None, "opus", "claude-opus-5-5")),
        ]
        for i, (name, prompt, model, ok) in enumerate(cases):
            got, p = pre(st, sid, prompt, model, f"toolu_r{i}")
            check(f"routing: {name}", ok(got), f"requested {model} -> {got or 'unchanged'}")

        # ---------- 2. Full spawn lifecycle: rewrite -> resolved -> usage -> panel ----------
        st = os.path.join(base, "lifecycle"); os.makedirs(st)
        sid = "battery-" + uuid.uuid4().hex[:8]
        got, _ = pre(st, sid, "rename the userId variable to userIdentifier", "claude-opus-5-5", "toolu_l1")
        post(st, sid, "toolu_l1", "agent-l1", "claude-haiku-4-5-20251001")
        stop(st, sid, "agent-l1", "claude-haiku-4-5-20251001",
             {"input_tokens": 1200, "output_tokens": 300, "cache_read_input_tokens": 5000, "cache_creation_input_tokens": 0})
        ev = events(st)
        outs = [e.get("outcome") for e in ev]
        check("lifecycle: rewrite, resolved and usage are all recorded",
              all(o in outs for o in ("rewrite_emitted", "resolved", "usage")), f"outcomes {outs}")
        usage = [e for e in ev if e.get("outcome") == "usage"]
        check("lifecycle: usage links to the decision", bool(usage) and bool(usage[-1].get("linked_decision")),
              f"linked_decision={usage[-1].get('linked_decision') if usage else None}")
        pj = panel(st, sid)
        check("panel: the opus->haiku spawn counts as a model switch",
              pj.get("model_switched", 0) >= 1, f"switched={pj.get('switched')} model_switched={pj.get('model_switched')} effort_changed={pj.get('effort_changed')}")
        tok = pj.get("tokens") or {}
        check("panel: tokens moved down are counted",
              (tok.get("work_saved", 0) + tok.get("cache_saved", 0)) > 0, f"tokens={tok}")

        # ---------- 3. Effort-only change is not a model switch ----------
        st = os.path.join(base, "effort"); os.makedirs(st)
        sid = "battery-" + uuid.uuid4().hex[:8]
        got, _ = pre(st, sid, "add a unit test for the parser edge cases and refactor the helper", "haiku", "toolu_e1")
        pj = panel(st, sid)
        check("panel: a haiku request never counts as a model downshift",
              pj.get("model_switched", 0) == 0, f"requested haiku -> {got or 'unchanged'}; model_switched={pj.get('model_switched')}")

        # ---------- 4. Compactor on real commands ----------
        st = os.path.join(base, "compact"); os.makedirs(st)
        env = {"DOWNSHIFT_EXEC": "1"}
        p = run(["compact", "--", "go", "test", "-count=1", "./internal/paths/"], None, st, cwd=REPO, env_extra=env)
        check("compactor: clean go test keeps exit 0", p.returncode == 0, f"exit {p.returncode}")
        p = run(["compact", "--", "go", "test", "-count=1", "-v", "./internal/paths/"], None, st, cwd=REPO, env_extra=env)
        raw_v = p.stdout
        check("compactor: verbose clean go test output is cut", p.returncode == 0 and "--- PASS" not in raw_v and len(raw_v) < 2000,
              f"exit {p.returncode}, emitted {len(raw_v)} bytes")
        p = run(["compact", "--", "go", "test", "-count=1", "-run", "TestDoesNotExistAnywhere_zz", "./internal/paths/"], None, st, cwd=REPO, env_extra=env)
        check("compactor: a no-test-match run keeps exit 0", p.returncode == 0, f"exit {p.returncode}")
        p = run(["compact", "--", "go", "vet", "./internal/paths/"], None, st, cwd=REPO, env_extra=env)
        check("compactor: go vet passes through with its exit code", p.returncode == 0, f"exit {p.returncode}")
        p = run(["compact", "--", "ls", "-la"], None, st, cwd=REPO, env_extra=env)
        check("compactor: a non-allowlisted command is refused (126) and says so",
              p.returncode == 126 and "refused" in p.stderr, f"exit {p.returncode}")
        # A failing test must come back verbatim with a non-zero exit.
        tmpmod = os.path.join(base, "failmod"); os.makedirs(tmpmod)
        open(os.path.join(tmpmod, "go.mod"), "w").write("module failmod\n\ngo 1.21\n")
        open(os.path.join(tmpmod, "f_test.go"), "w").write('package failmod\nimport "testing"\nfunc TestFail(t *testing.T){ t.Fatal("boom-expected") }\n')
        p = run(["compact", "--", "go", "test", "./..."], None, st, cwd=tmpmod, env_extra=env)
        check("compactor: a failing test keeps its exit code and its message",
              p.returncode != 0 and "boom-expected" in (p.stdout + p.stderr), f"exit {p.returncode}")
        m = run(["context", "metrics"], None, st)
        try:
            em = json.loads(m.stdout).get("exec_metrics", {})
        except json.JSONDecodeError:
            em = {}
        check("compactor: metrics count the runs and at least one cut",
              em.get("commands", 0) >= 5 and em.get("compacted", 0) >= 1,
              f"commands={em.get('commands')} compacted={em.get('compacted')} saved_bytes={em.get('saved_bytes')}")

        # ---------- 5. Bash hook rewrite ----------
        st = os.path.join(base, "bash"); os.makedirs(st)
        sid = "battery-" + uuid.uuid4().hex[:8]
        for cmd, want in (("go test ./internal/paths/", True), ("rm -rf /tmp/x", False), ("go test ./... | tee out.txt", False)):
            payload = {"session_id": sid, "tool_name": "Bash", "tool_use_id": "toolu_b" + uuid.uuid4().hex[:4], "tool_input": {"command": cmd}}
            p = run(["claude-code"], payload, st)
            try:
                newcmd = ((json.loads(p.stdout or "{}").get("hookSpecificOutput") or {}).get("updatedInput") or {}).get("command", "")
            except json.JSONDecodeError:
                newcmd = ""
            wrapped = " compact " in newcmd
            check(f"bash hook: {'wraps' if want else 'leaves alone'} `{cmd}`", wrapped == want, f"wrapped={wrapped}")

        # ---------- 6. Gateway shadow ----------
        st = os.path.join(base, "gateway"); os.makedirs(st)
        allow = {"version": 1, "harness": "opencode", "models": [
            {"id": "deepseek/deepseek-v4.1-flash-1017", "tier": "small", "max_effort": "medium", "zdr": False}]}
        open(os.path.join(st, "openrouter.json"), "w").write(json.dumps(allow))
        sid = "battery-" + uuid.uuid4().hex[:8]
        got, _ = pre(st, sid, "rename the userId variable to userIdentifier", "claude-opus-5-5", "toolu_g1")
        ev = events(st)
        gw = [e.get("gateway_would") for e in ev if e.get("gateway_would")]
        check("gateway: a shadow row is recorded as would-route", "deepseek/deepseek-v4.1-flash-1017" in gw, f"gateway_would={gw}")
        check("gateway: a shadow row never changes the live model", got in ("haiku", "claude-haiku-4-5"), f"live -> {got}")
        bad = dict(allow); bad["models"] = [{"id": "no-slash", "tier": "small", "max_effort": "medium", "zdr": False}]
        open(os.path.join(st, "openrouter.json"), "w").write(json.dumps(bad))
        got2, p2 = pre(st, sid, "rename the userId variable to userIdentifier", "claude-opus-5-5", "toolu_g2")
        check("gateway: a bad allowlist never blocks a spawn", p2.returncode == 0 and got2 in ("haiku", "claude-haiku-4-5"), f"exit {p2.returncode}, live -> {got2}")

        # ---------- 7. Fail-open ----------
        st = os.path.join(base, "failopen"); os.makedirs(st)
        env = dict(os.environ); env["DOWNSHIFT_STATE_DIR"] = st
        p = subprocess.run([BIN, "claude-code"], input="not json at all", capture_output=True, text=True, env=env, timeout=30)
        check("fail-open: garbage stdin leaves the spawn unchanged", p.returncode == 0 and p.stdout.strip() in ("", "{}"), f"exit {p.returncode} out={p.stdout.strip()[:40]}")
        big = {"tool_name": "Task", "tool_input": {"prompt": "x" * (2 * 1024 * 1024), "model": "claude-opus-5-5"}}
        p = subprocess.run([BIN, "claude-code"], input=json.dumps(big), capture_output=True, text=True, env=env, timeout=30)
        check("fail-open: a payload over 1 MiB leaves the spawn unchanged", p.returncode == 0 and p.stdout.strip() in ("", "{}"), f"exit {p.returncode}")
    finally:
        shutil.rmtree(base, ignore_errors=True)

    w = max(len(n) for n, _, _ in results)
    passed = sum(1 for _, ok, _ in results if ok)
    for n, ok, d in results:
        print(f"{'PASS' if ok else 'FAIL'}  {n.ljust(w)}  {d}")
    print(f"\n{passed}/{len(results)} passed")
    return 0 if passed == len(results) else 1


if __name__ == "__main__":
    sys.exit(main())
