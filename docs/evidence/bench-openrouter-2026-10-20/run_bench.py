#!/usr/bin/env python3
# Synthetic small-task bench through the OpenRouter chat completions API.
# Reads the key from the OPENROUTER_API_KEY environment variable; it is never printed or stored.
# Usage: OPENROUTER_API_KEY=... python3 run_bench.py
import json
import os
import re
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request
from datetime import datetime, timezone
from pathlib import Path

HERE = Path(__file__).resolve().parent
TASKS_DIR = HERE / "tasks"
OUT_DIR = HERE / "outputs"
RESULTS = HERE / "results.jsonl"

URL = "https://openrouter.ai/api/v1/chat/completions"
MODELS = [
    "deepseek/deepseek-v4.1-flash-1017",
    "minimax/minimax-m3.1-flash:free",
    "anthropic/claude-haiku-4.5",
]
REPS = 3
MAX_TOKENS = 1500
CHECK_TIMEOUT_S = 10
FENCE = re.compile(r"```(?:python|py)?[ \t]*\n(.*?)```", re.S)


def slug(model):
    return re.sub(r"[^a-z0-9]+", "-", model.lower()).strip("-")


def extract_code(text):
    match = FENCE.search(text)
    return match.group(1) if match else text


def call_model(key, model, prompt):
    body = json.dumps({
        "model": model,
        "messages": [{"role": "user", "content": prompt}],
        "temperature": 0,
        "max_tokens": MAX_TOKENS,
    }).encode()
    req = urllib.request.Request(
        URL,
        data=body,
        headers={"Authorization": "Bearer " + key, "Content-Type": "application/json"},
        method="POST",
    )
    started = time.perf_counter()
    try:
        with urllib.request.urlopen(req, timeout=180) as resp:
            payload = json.load(resp)
        latency_ms = (time.perf_counter() - started) * 1000
        choice = payload["choices"][0]
        usage = payload.get("usage") or {}
        return {
            "text": choice["message"].get("content") or "",
            "finish_reason": choice.get("finish_reason"),
            "input_tokens": usage.get("prompt_tokens"),
            "output_tokens": usage.get("completion_tokens"),
            "latency_ms": round(latency_ms, 1),
            "error": None,
        }
    except urllib.error.HTTPError as exc:
        latency_ms = (time.perf_counter() - started) * 1000
        message = ""
        try:
            message = json.load(exc).get("error", {}).get("message", "")
        except Exception:
            pass
        return {"text": "", "finish_reason": None, "input_tokens": None, "output_tokens": None,
                "latency_ms": round(latency_ms, 1), "error": "HTTP %d: %s" % (exc.code, message[:200])}
    except Exception as exc:  # network, timeout, malformed body
        latency_ms = (time.perf_counter() - started) * 1000
        return {"text": "", "finish_reason": None, "input_tokens": None, "output_tokens": None,
                "latency_ms": round(latency_ms, 1), "error": type(exc).__name__ + ": " + str(exc)[:200]}


def run_check(task_dir, code):
    """Run the task's tests against the candidate code in a fresh subprocess. Returns (passed, reason)."""
    tests = (task_dir / "tests.py").resolve()
    with tempfile.TemporaryDirectory() as work:
        Path(work, "candidate.py").write_text(code, encoding="utf-8")
        script = (
            "import sys; sys.path.insert(0, %r); "
            "exec(compile(open(%r, encoding='utf-8').read(), %r, 'exec'), {'__name__': '__main__'})"
        ) % (work, str(tests), str(tests))
        try:
            proc = subprocess.run(
                [sys.executable, "-I", "-c", script],
                cwd=work, capture_output=True, text=True, timeout=CHECK_TIMEOUT_S,
            )
        except subprocess.TimeoutExpired:
            return False, "timeout"
        if proc.returncode == 0 and "ALL PASS" in proc.stdout:
            return True, ""
        lines = (proc.stderr.strip().splitlines() or ["exit %d" % proc.returncode])
        return False, lines[-1][:200]


def main():
    key = os.environ.get("OPENROUTER_API_KEY", "")
    if not key:
        sys.exit("OPENROUTER_API_KEY is not set")
    OUT_DIR.mkdir(exist_ok=True)
    only = os.environ.get("BENCH_TASK", "")
    tasks = sorted(p for p in TASKS_DIR.iterdir() if p.is_dir() and (not only or p.name == only))
    results_path = Path(os.environ.get("BENCH_OUT", str(RESULTS)))
    with results_path.open("w", encoding="utf-8") as out:
        for task_dir in tasks:
            prompt = (task_dir / "prompt.txt").read_text(encoding="utf-8")
            for rep in range(1, REPS + 1):
                for model in MODELS:
                    res = call_model(key, model, prompt)
                    code = extract_code(res["text"]) if not res["error"] else ""
                    passed, check_err = (False, "api error") if res["error"] else run_check(task_dir, code)
                    (OUT_DIR / ("%s__%s__%d.txt" % (slug(model), task_dir.name, rep))).write_text(
                        res["text"], encoding="utf-8")
                    row = {
                        "model": model,
                        "task": task_dir.name,
                        "rep": rep,
                        "pass": passed,
                        "input_tokens": res["input_tokens"],
                        "output_tokens": res["output_tokens"],
                        "latency_ms": res["latency_ms"],
                        "error": res["error"],
                        "check_error": None if passed else check_err,
                        "finish_reason": res["finish_reason"],
                        "ran_at": datetime.now(timezone.utc).isoformat(timespec="seconds"),
                    }
                    out.write(json.dumps(row, ensure_ascii=False) + "\n")
                    out.flush()
                    print("%-34s %-20s rep=%d pass=%-5s %8.0f ms  %s" % (
                        model, task_dir.name, rep, passed, res["latency_ms"], res["error"] or ""))


if __name__ == "__main__":
    main()
