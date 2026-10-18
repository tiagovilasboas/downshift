#!/usr/bin/env python3
# Copyright (c) 2026 Tiago Vilas Boas.
# SPDX-License-Identifier: Apache-2.0
"""Fill catalog cache prices (and align Codex list prices) from a price snapshot.

Reads an OpenRouter snapshot (research/prices/effort-capabilities-*.json in the
downshift-labs repo: pricing is USD per token as strings) and edits
internal/catalog/catalog.json in place:

  * cache_read_cost_per_1m / cache_write_cost_per_1m are written for every entry
    whose id equals a snapshot model id (provider prefix dropped, dots read as
    dashes, so claude-opus-5-5 == anthropic/claude-opus-5.5). Entries with no
    exact id match are left untouched; nothing is invented or inferred.
  * input/output prices are overwritten only for harnesses named in --align
    (default: codex), again only on an exact id match.

Usage:
  python3 scripts/sync_catalog_prices.py SNAPSHOT.json [--align codex] [--dry-run]

Prints every change. Stdlib only.
"""
import argparse
import json
import re
import sys
from pathlib import Path

CATALOG = Path(__file__).resolve().parent.parent / "internal/catalog/catalog.json"


def norm(name):
    return name.split("/", 1)[-1].replace(".", "-")


def per_m(v):
    return round(float(v) * 1_000_000, 6)


def fmt(x):
    s = f"{x:.6f}".rstrip("0")
    if s.endswith("."):
        s += "0"
    if len(s.split(".")[1]) < 2:
        s += "0"
    return s


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("snapshot")
    ap.add_argument("--align", default="codex", help="comma list of harnesses whose input/output prices follow the snapshot")
    ap.add_argument("--dry-run", action="store_true")
    a = ap.parse_args()
    snap = json.loads(Path(a.snapshot).read_text())
    align = set(filter(None, a.align.split(",")))
    prices = {norm(k): v["pricing"] for k, v in snap["models"].items()}

    out, cur, matched = [], {}, set()
    for line in CATALOG.read_text().splitlines():
        if re.search(r'"cache_(read|write)_cost_per_1m"', line):
            continue  # regenerated below
        m = re.search(r'"(id|harness)":\s*"([^"]+)"', line)
        if m:
            if m.group(1) == "id":
                cur = {"id": m.group(2)}
            else:
                cur["harness"] = m.group(2)
        p = prices.get(norm(cur.get("id", ""))) if cur else None
        if p and '"input_cost_per_1m"' in line and cur.get("harness") in align:
            new = fmt(per_m(p["prompt"]))
            if float(re.search(r":\s*([\d.]+)", line).group(1)) != float(new):
                print(f"price  {cur['harness']}/{cur['id']} input -> {new}")
                line = re.sub(r":\s*[\d.]+", f": {new}", line)
        if p and '"output_cost_per_1m"' in line:
            if cur.get("harness") in align:
                new = fmt(per_m(p["completion"]))
                if float(re.search(r":\s*([\d.]+)", line).group(1)) != float(new):
                    print(f"price  {cur['harness']}/{cur['id']} output -> {new}")
                    line = re.sub(r":\s*[\d.]+", f": {new}", line)
            extra = []
            if p.get("input_cache_read"):
                extra.append(f'      "cache_read_cost_per_1m": {fmt(per_m(p["input_cache_read"]))}')
            if p.get("input_cache_write"):
                extra.append(f'      "cache_write_cost_per_1m": {fmt(per_m(p["input_cache_write"]))}')
            if extra:
                line = line.rstrip().rstrip(",") + ",\n" + ",\n".join(extra)
                matched.add((cur["harness"], cur["id"]))
        out.append(line)
    print("cache prices set for:", ", ".join(f"{h}/{i}" for h, i in sorted(matched)))
    if not a.dry_run:
        CATALOG.write_text("\n".join(out) + "\n")


if __name__ == "__main__":
    sys.exit(main())
