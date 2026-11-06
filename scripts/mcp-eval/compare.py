#!/usr/bin/env python3
"""Compare independently scored MCP instruction runs; missing data never passes."""
import argparse
import json
import sys


def compare(cases, records, repetitions=3):
    expected = {(case, arm, rep) for case in cases for arm in ("A", "B")
                for rep in range(1, repetitions + 1)}
    indexed = {}
    for row in records:
        key = (row["case"], row["arm"], row["repetition"])
        if key not in expected or key in indexed:
            raise ValueError(f"unexpected or duplicate run: {key}")
        if "accepted" not in row or (row["accepted"] is not None and type(row["accepted"]) is not bool):
            raise ValueError(f"invalid acceptance: {key}")
        if type(row.get("critical_failure")) is not bool:
            raise ValueError(f"missing critical-failure assessment: {key}")
        indexed[key] = row
    missing = sorted(expected - indexed.keys())
    unavailable = sorted(key for key, row in indexed.items() if row.get("accepted") is None)
    critical = sorted(key for key, row in indexed.items() if key[1] == "B" and row["critical_failure"])
    baseline_failures = sorted(key for key, row in indexed.items() if key[1] == "A" and (row["accepted"] is False or row["critical_failure"]))
    regressions = []
    for case in cases:
        base = [indexed.get((case, "A", rep), {}).get("accepted") for rep in range(1, repetitions + 1)]
        candidate = [indexed.get((case, "B", rep), {}).get("accepted") for rep in range(1, repetitions + 1)]
        if all(value is not None for value in base + candidate) and sum(candidate) < sum(base):
            regressions.append(case)
    verdict = ("regression" if critical or regressions else "incomplete" if missing or unavailable
               else "baseline_failed" if baseline_failures else "no_observed_regression")
    return {"verdict": verdict, "missing": missing, "not_evaluated": unavailable,
            "critical_candidate_failures": critical, "baseline_failures": baseline_failures, "acceptance_regressions": regressions,
            "note": "Exploratory results do not prove equivalence; tokens and cost require separate measurements."}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("cases", help="JSON case manifest")
    parser.add_argument("records", help="JSON array of scored runs")
    args = parser.parse_args()
    try:
        with open(args.cases, encoding="utf-8") as source:
            manifest = json.load(source)
        cases = [case["id"] for case in manifest["cases"]]
        with open(args.records, encoding="utf-8") as source:
            records = json.load(source)
        if not isinstance(cases, list) or not cases or len(set(cases)) != len(cases):
            raise ValueError("cases must be a nonempty array of unique IDs")
        result = compare(cases, records)
        print(json.dumps(result, indent=2))
        return 0 if result["verdict"] == "no_observed_regression" else 1
    except (OSError, ValueError, KeyError, TypeError) as error:
        print(f"comparison failed: {error}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    sys.exit(main())
