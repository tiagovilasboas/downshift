#!/usr/bin/env python3
# Builds summary.md from both runs (max_tokens 1500 at the top level, max_tokens 4000 in run-4000/).
# Prices are read from the public OpenRouter model list at run time.
import json
import statistics
import urllib.request
from datetime import datetime, timezone
from pathlib import Path

HERE = Path(__file__).resolve().parent
SUMMARY = HERE / "summary.md"
RUNS = [
    ("1500", HERE / "results.jsonl", 1500),
    ("4000", HERE / "run-4000" / "results.jsonl", 4000),
]
MODELS = [
    "deepseek/deepseek-v4.1-flash-1017",
    "minimax/minimax-m3.1-flash:free",
    "anthropic/claude-haiku-4.5",
]


def load(path):
    return [json.loads(line) for line in path.read_text(encoding="utf-8").splitlines() if line.strip()]


def fetch_prices():
    with urllib.request.urlopen("https://openrouter.ai/api/v1/models", timeout=60) as resp:
        data = json.load(resp)["data"]
    prices = {}
    for m in data:
        if m["id"] in MODELS:
            pricing = m.get("pricing") or {}
            prices[m["id"]] = (float(pricing["prompt"]) * 1e6, float(pricing["completion"]) * 1e6)
    return prices


def model_stats(rows, model, prices):
    mrows = [r for r in rows if r["model"] == model]
    n = len(mrows)
    passes = sum(1 for r in mrows if r["pass"])
    in_toks = [r["input_tokens"] for r in mrows if r["input_tokens"] is not None]
    out_toks = [r["output_tokens"] for r in mrows if r["output_tokens"] is not None]
    cost = None
    if model in prices and in_toks and passes:
        pin, pout = prices[model]
        total = sum(r["input_tokens"] * pin / 1e6 + (r["output_tokens"] or 0) * pout / 1e6 for r in mrows)
        cost = total / passes * 10
    return {
        "n": n,
        "passes": passes,
        "median_ms": statistics.median(r["latency_ms"] for r in mrows) if mrows else 0,
        "mean_in": statistics.mean(in_toks) if in_toks else 0,
        "mean_out": statistics.mean(out_toks) if out_toks else 0,
        "truncated": sum(1 for r in mrows if r.get("finish_reason") == "length"),
        "empty": sum(1 for r in mrows if (r.get("output_tokens") or 0) == 0 and not r["error"]),
        "errors": sum(1 for r in mrows if r["error"]),
        "cost10": cost,
    }


def fmt_cost(value, passes):
    if not passes:
        return "n/a (0 passes)"
    return "n/a" if value is None else "$%.4f" % value


def main():
    runs = [(label, load(path), cap) for label, path, cap in RUNS if path.exists()]
    prices = fetch_prices()
    fetched_at = datetime.now(timezone.utc).isoformat(timespec="minutes")
    tasks = sorted({r["task"] for r in runs[0][1]})
    lines = []
    lines.append("# Bench OpenRouter: small synthetic coding tasks (2026-10-20)")
    lines.append("")
    lines.append("- Tasks: %d synthetic tasks (`tasks/`), 3 repetitions each, 30 calls per model per run." % len(tasks))
    lines.append("- Settings: temperature 0, OpenAI-compatible chat endpoint `https://openrouter.ai/api/v1`. The only difference between runs is max_tokens.")
    lines.append("- Pass = the returned code passes the task's assert-based `tests.py` in a fresh subprocess with a 10 s timeout.")
    lines.append("- Prices fetched from the public OpenRouter model list at %s (USD per 1M tokens):" % fetched_at)
    for model in MODELS:
        if model in prices:
            lines.append("  - `%s`: input %.4f, output %.4f" % (model, prices[model][0], prices[model][1]))
        else:
            lines.append("  - `%s`: price not found in the list" % model)
    lines.append("- Cost per 10 passing tasks = total cost of all calls of the model (failures included) divided by passes, times 10.")
    lines.append("- `Cut` = responses that stopped at max_tokens (finish_reason=length). `Empty` = outputs with zero output tokens and no API error.")
    lines.append("")
    lines.append("## Side by side")
    lines.append("")
    header = "| Model |"
    sep = "|---|"
    for label, _, cap in runs:
        header += " max_tokens %s: pass | median ms | cost/10 pass | cut | empty |" % label
        sep += "---|---|---|---|---|"
    lines.append(header)
    lines.append(sep)
    for model in MODELS:
        cells = []
        for _, rows, _ in runs:
            s = model_stats(rows, model, prices)
            cells.append("%d/%d (%.0f%%) | %.0f | %s | %d | %d" % (
                s["passes"], s["n"], 100.0 * s["passes"] / s["n"] if s["n"] else 0,
                s["median_ms"], fmt_cost(s["cost10"], s["passes"]), s["truncated"], s["empty"]))
        lines.append("| `%s` | %s |" % (model, " | ".join(cells)))
    lines.append("")
    lines.append("## Mean tokens per run")
    lines.append("")
    lines.append("| Model | Run | Mean input tok | Mean output tok | API errors |")
    lines.append("|---|---|---|---|---|")
    for model in MODELS:
        for label, rows, _ in runs:
            s = model_stats(rows, model, prices)
            lines.append("| `%s` | %s | %.0f | %.0f | %d |" % (model, label, s["mean_in"], s["mean_out"], s["errors"]))
    lines.append("")
    lines.append("## Pass count per task (out of 3)")
    lines.append("")
    lines.append("| Task | " + " | ".join("`%s` @%s" % (m, label) for label, _, _ in runs for m in MODELS) + " |")
    lines.append("|---|" + "---|" * (len(runs) * len(MODELS)))
    for task in tasks:
        cells = []
        for _, rows, _ in runs:
            for model in MODELS:
                cells.append(str(sum(1 for r in rows if r["model"] == model and r["task"] == task and r["pass"])))
        lines.append("| %s | %s |" % (task, " | ".join(cells)))
    lines.append("")
    lines.append("## Failures")
    lines.append("")
    for label, rows, _ in runs:
        failures = [r for r in rows if not r["pass"]]
        lines.append("### max_tokens %s: %d failing calls" % (label, len(failures)))
        lines.append("")
        for r in failures:
            reason = r["error"] or r["check_error"] or "unknown"
            if r.get("finish_reason") == "length":
                reason = "cut at max_tokens; " + reason
            lines.append("- `%s` / %s / rep %d: %s" % (r["model"], r["task"], r["rep"], reason))
        lines.append("")
    lines.append("## Notes on the runs")
    lines.append("")
    lines.append("- Run max_tokens 1500 is the first run; its raw files sit at the top level of this folder.")
    lines.append("- Run max_tokens 4000 is in `run-4000/`, with the same tasks, checkers and settings.")
    lines.append("- Task 02 was run twice in the 1500 run. The first run used a checker with a wrong expectation (it asserted that present values return -1), so all 9 of its rows failed for the checker's reason. The checker was fixed and only task 02 was rerun; discarded rows are in results_discarded_task02_checker_bug.jsonl.")
    lines.append("")
    lines.append("## What this does not show")
    lines.append("")
    lines.append("- n is small: 10 synthetic tasks x 3 repetitions per model per run. The pass rates have wide intervals and say little about the true rate.")
    lines.append("- The tasks are small, single-function problems with asserts written by the author. They are not real repository work, multi-file changes, or long contexts.")
    lines.append("- Temperature 0 is not fully deterministic on hosted providers; repetitions show variance only partly.")
    lines.append("- Prices are list prices at the fetch time. DeepSeek lists time-of-day (UTC) overrides; the base price is used here.")
    lines.append("- Free tiers may be rate-limited or change routing; latency includes queueing at the provider and network time.")
    lines.append("- Data retention, privacy terms and zero-data-retention (ZDR) status of each provider are not evaluated here.")
    lines.append("- Errors are reported as they occurred; none were retried.")
    lines.append("")
    SUMMARY.write_text("\n".join(lines), encoding="utf-8")
    print("wrote", SUMMARY)


if __name__ == "__main__":
    main()
