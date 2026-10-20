#!/usr/bin/env python3
# Builds summary.md from results.jsonl. Prices are read from the public OpenRouter model list at run time.
import json
import statistics
import urllib.request
from datetime import datetime, timezone
from pathlib import Path

HERE = Path(__file__).resolve().parent
RESULTS = HERE / "results.jsonl"
SUMMARY = HERE / "summary.md"
MODELS = [
    "deepseek/deepseek-v4.1-flash-1017",
    "minimax/minimax-m3.1-flash:free",
    "anthropic/claude-haiku-4.5",
]


def fetch_prices():
    with urllib.request.urlopen("https://openrouter.ai/api/v1/models", timeout=60) as resp:
        data = json.load(resp)["data"]
    prices = {}
    for m in data:
        if m["id"] in MODELS:
            pricing = m.get("pricing") or {}
            prices[m["id"]] = (float(pricing["prompt"]) * 1e6, float(pricing["completion"]) * 1e6)
    return prices


def fmt_cost(value):
    return "n/a" if value is None else "$%.4f" % value


def main():
    rows = [json.loads(line) for line in RESULTS.read_text(encoding="utf-8").splitlines() if line.strip()]
    prices = fetch_prices()
    fetched_at = datetime.now(timezone.utc).isoformat(timespec="minutes")
    tasks = sorted({r["task"] for r in rows})
    lines = []
    lines.append("# Bench OpenRouter: small synthetic coding tasks (2026-10-20)")
    lines.append("")
    lines.append("- Tasks: %d synthetic tasks (`tasks/`), 3 repetitions each, %d calls per model, %d calls total."
                 % (len(tasks), len(rows) // len(MODELS), len(rows)))
    lines.append("- Settings: temperature 0, max_tokens 1500, OpenAI-compatible chat endpoint `https://openrouter.ai/api/v1`.")
    lines.append("- Pass = the returned code passes the task's assert-based `tests.py` in a fresh subprocess with a 10 s timeout.")
    lines.append("- Prices fetched from the public OpenRouter model list at %s (USD per 1M tokens):" % fetched_at)
    for model in MODELS:
        if model in prices:
            lines.append("  - `%s`: input %.4f, output %.4f" % (model, prices[model][0], prices[model][1]))
        else:
            lines.append("  - `%s`: price not found in the list" % model)
    lines.append("- Cost per 10 passing tasks = total cost of all calls of the model (failures included) divided by passes, times 10.")
    lines.append("")
    lines.append("## Per model")
    lines.append("")
    lines.append("| Model | Pass | Median latency (ms) | Mean input tok | Mean output tok | API errors | Cost per 10 passes |")
    lines.append("|---|---|---|---|---|---|---|")
    for model in MODELS:
        mrows = [r for r in rows if r["model"] == model]
        n = len(mrows)
        passes = sum(1 for r in mrows if r["pass"])
        latencies = [r["latency_ms"] for r in mrows]
        in_toks = [r["input_tokens"] for r in mrows if r["input_tokens"] is not None]
        out_toks = [r["output_tokens"] for r in mrows if r["output_tokens"] is not None]
        errors = sum(1 for r in mrows if r["error"])
        cost = None
        if model in prices and in_toks:
            pin, pout = prices[model]
            total = sum(r["input_tokens"] * pin / 1e6 + (r["output_tokens"] or 0) * pout / 1e6 for r in mrows)
            cost = total / passes * 10 if passes else None
        lines.append("| `%s` | %d/%d (%.0f%%) | %.0f | %.0f | %.0f | %d | %s |" % (
            model, passes, n, 100.0 * passes / n if n else 0,
            statistics.median(latencies) if latencies else 0,
            statistics.mean(in_toks) if in_toks else 0,
            statistics.mean(out_toks) if out_toks else 0,
            errors, fmt_cost(cost) if passes else "n/a (0 passes)"))
    lines.append("")
    lines.append("## Pass count per task (out of 3)")
    lines.append("")
    lines.append("| Task | " + " | ".join("`%s`" % m for m in MODELS) + " |")
    lines.append("|---|" + "---|" * len(MODELS))
    for task in tasks:
        cells = []
        for model in MODELS:
            cells.append(str(sum(1 for r in rows if r["model"] == model and r["task"] == task and r["pass"])))
        lines.append("| %s | %s |" % (task, " | ".join(cells)))
    failures = [r for r in rows if not r["pass"]]
    lines.append("")
    lines.append("## Failures")
    lines.append("")
    if not failures:
        lines.append("None.")
    for r in failures:
        reason = r["error"] or r["check_error"] or "unknown"
        lines.append("- `%s` / %s / rep %d: %s" % (r["model"], r["task"], r["rep"], reason))
    lines.append("")
    lines.append("## What this does not show")
    lines.append("")
    lines.append("- n is small: 10 synthetic tasks x 3 repetitions per model. The pass rates have wide intervals and say little about the true rate.")
    lines.append("- The tasks are small, single-function problems with asserts written by the author. They are not real repository work, multi-file changes, or long contexts.")
    lines.append("- Temperature 0 is not fully deterministic on hosted providers; repetitions show variance only partly.")
    lines.append("- Prices are list prices at the fetch time. DeepSeek lists time-of-day (UTC) overrides; the base price is used here.")
    lines.append("- Free tiers may be rate-limited or change routing; latency includes queueing at the provider and network time.")
    lines.append("- Data retention, privacy terms and zero-data-retention (ZDR) status of each provider are not evaluated here.")
    lines.append("- Errors are reported as they occurred; none were retried.")
    lines.append("- Output cap: 1500 max_tokens. %d responses stopped at the cap (finish_reason=length) and %d outputs were empty; these count as failures and are a confound for the models that produce long reasoning output." % (
        sum(1 for r in rows if r.get("finish_reason") == "length"),
        sum(1 for r in rows if (r.get("output_tokens") or 0) == 0 and not r["error"]),
    ))
    lines.append("- Task 02 was run twice. The first run used a checker with a wrong expectation (it asserted that present values return -1), so all 9 of its rows failed for the checker's reason. The checker was fixed and only task 02 was rerun; the discarded rows are kept in results_discarded_task02_checker_bug.jsonl.")
    lines.append("")
    SUMMARY.write_text("\n".join(lines), encoding="utf-8")
    print("wrote", SUMMARY)


if __name__ == "__main__":
    main()
