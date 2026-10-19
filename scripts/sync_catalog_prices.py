#!/usr/bin/env python3
# Copyright (c) 2026 Tiago Vilas Boas.
# SPDX-License-Identifier: Apache-2.0
"""Refresh list prices in internal/catalog/catalog.json from a public source.

The hook must work offline and fast, so prices are embedded in the catalog and
never fetched at run time. This script, run weekly by
.github/workflows/sync-prices.yml, is what keeps that embedded snapshot fresh.

Source: the OpenRouter public model list (https://openrouter.ai/api/v1/models,
no key, pricing is USD per token as strings). Pass --snapshot FILE to read a
saved copy of the same JSON instead (offline runs, tests).

What it edits, for entries of the harnesses named in --harnesses (default
claude-code,codex,kirocrew) whose id equals a source model id exactly (provider
prefix dropped, dots read as dashes: claude-opus-5-5 == anthropic/claude-opus-5.5):

  * input_cost_per_1m / output_cost_per_1m
  * cache_read_cost_per_1m / cache_write_cost_per_1m, only when the source has one

Nothing is inferred: an entry with no exact id match is left untouched, and a
cache price the source lacks is kept as is. Variants (":batch" ids, "~latest"
aliases) are ignored. On success it sets "prices_as_of" (and "updated") to
today's UTC date, which is the freshness signal `downshift doctor` reads.

Usage:
  python3 scripts/sync_catalog_prices.py [--snapshot FILE] [--harnesses a,b]
                                         [--today YYYY-MM-DD] [--allow-large-change]
                                         [--dry-run]

Every price taken from the source is validated before anything is written: it
must be a finite number greater than zero and at most MAX_PER_M USD per 1M
tokens, and it may not move more than MAX_CHANGE_FACTOR times up or down from
the value already in the catalog unless --allow-large-change is given. Any
violation prints one clear line per price and exits 1 without touching the
catalog.

Prints every change; exit 0 on success (changed or not), 1 on a fetch, parse or
validation failure (the catalog is not touched). Stdlib only, no prompts, no
secrets.
"""
import argparse
import datetime
import json
import math
import re
import sys
import urllib.request
from pathlib import Path

CATALOG = Path(__file__).resolve().parent.parent / "internal/catalog/catalog.json"
SOURCE_URL = "https://openrouter.ai/api/v1/models"
KEYS = [
    ("input_cost_per_1m", "prompt"),
    ("output_cost_per_1m", "completion"),
    ("cache_read_cost_per_1m", "input_cache_read"),
    ("cache_write_cost_per_1m", "input_cache_write"),
]


# A list price per 1M tokens above this is a unit mistake, not a price.
MAX_PER_M = 1000.0
# Largest accepted move against the catalog's current value, per run.
MAX_CHANGE_FACTOR = 5.0


class PriceError(ValueError):
    """A source price that must not reach the catalog."""


def norm(name):
    return name.split("/", 1)[-1].replace(".", "-").lower()


def per_m(v):
    return round(float(v) * 1_000_000, 6)


def fmt(x):
    s = f"{x:.6f}".rstrip("0")
    if s.endswith("."):
        s += "0"
    if len(s.split(".")[1]) < 2:
        s += "0"
    return s


def load_source(path):
    if path:
        raw = Path(path).read_text()
    else:
        req = urllib.request.Request(SOURCE_URL, headers={"User-Agent": "downshift-sync-prices"})
        with urllib.request.urlopen(req, timeout=60) as r:
            raw = r.read().decode()
    data = json.loads(raw)
    rows = data["data"] if "data" in data else data["models"]
    prices = {}
    for m in rows:
        mid = m["id"] if "id" in m else None
        if not mid or ":" in mid or mid.startswith("~"):
            continue
        p = m.get("pricing") or {}
        if p.get("prompt") is None or p.get("completion") is None:
            continue
        prices[norm(mid)] = p
    if not prices:
        raise ValueError("source has no priced models")
    return prices


def split_entries(lines):
    """Yield (start, end) line spans of catalog entries (`    {` .. `    }`)."""
    spans, start = [], None
    for i, line in enumerate(lines):
        if start is None and re.fullmatch(r"    \{\s*", line):
            start = i
        elif start is not None and re.fullmatch(r"    \},?\s*", line):
            spans.append((start, i))
            start = None
    return spans


def checked_per_m(raw, label, key):
    """Convert a source USD-per-token price to USD per 1M, rejecting nonsense."""
    try:
        value = per_m(raw)
    except (TypeError, ValueError, OverflowError):
        raise PriceError(f"{label} {key}: not a number ({raw!r})") from None
    if not math.isfinite(value):
        raise PriceError(f"{label} {key}: not finite ({raw!r})")
    if value <= 0:
        raise PriceError(f"{label} {key}: must be greater than zero ({raw!r})")
    if value > MAX_PER_M:
        raise PriceError(f"{label} {key}: {value} USD per 1M exceeds the {MAX_PER_M:g} limit ({raw!r})")
    return value


def check_move(label, key, old, new, allow_large):
    if allow_large or old <= 0:
        return
    factor = new / old
    if factor > MAX_CHANGE_FACTOR or factor < 1 / MAX_CHANGE_FACTOR:
        raise PriceError(
            f"{label} {key}: {old} -> {new} is a {factor:.3g}x move (limit {MAX_CHANGE_FACTOR:g}x); "
            "rerun with --allow-large-change after checking the source"
        )


def sync(lines, prices, harnesses, allow_large=False):
    changes = []
    errors = []
    spans = split_entries(lines)
    # Edit back to front so earlier spans keep their indexes.
    for start, end in reversed(spans):
        block = lines[start + 1 : end]
        text = "\n".join(block)
        mid = re.search(r'"id":\s*"([^"]+)"', text)
        har = re.search(r'"harness":\s*"([^"]+)"', text)
        if not mid or not har or har.group(1) not in harnesses:
            continue
        p = prices.get(norm(mid.group(1)))
        if not p:
            continue
        label = f"{har.group(1)}/{mid.group(1)}"
        for key, field in KEYS:
            if p.get(field) in (None, "", "0"):
                continue
            try:
                value = checked_per_m(p[field], label, key)
            except PriceError as exc:
                errors.append(str(exc))
                continue
            new = fmt(value)
            idx = next((j for j, b in enumerate(block) if re.match(rf'\s*"{key}":', b)), None)
            if idx is not None:
                old = re.search(r":\s*([\d.]+)", block[idx]).group(1)
                if float(old) == float(new):
                    continue
                try:
                    check_move(label, key, float(old), float(new), allow_large)
                except PriceError as exc:
                    errors.append(str(exc))
                    continue
                comma = "," if block[idx].rstrip().endswith(",") else ""
                block[idx] = re.sub(r":\s*[\d.]+,?\s*$", f": {new}{comma}", block[idx])
                changes.append(f"price  {label} {key}: {old} -> {new}")
            else:
                last = max(j for j, b in enumerate(block) if b.strip())
                if not block[last].rstrip().endswith(","):
                    block[last] = block[last].rstrip() + ","
                block.insert(last + 1, f'      "{key}": {new}')
                changes.append(f"price  {label} {key}: (none) -> {new}")
        lines[start + 1 : end] = block
    if errors:
        raise PriceError("\n".join(reversed(errors)))
    return changes


def set_top_level(lines, key, value):
    for i, line in enumerate(lines):
        if re.match(rf'  "{key}":', line):
            new = f'  "{key}": "{value}",'
            if lines[i] == new:
                return False
            lines[i] = new
            return True
    # "prices_as_of" is added right after "updated" the first time.
    for i, line in enumerate(lines):
        if re.match(r'  "updated":', line):
            lines.insert(i + 1, f'  "{key}": "{value}",')
            return True
    raise ValueError(f'catalog has no "updated" line to anchor "{key}"')


def main():
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    ap.add_argument("--snapshot", help="read this saved OpenRouter models JSON instead of fetching")
    ap.add_argument("--harnesses", default="claude-code,codex,kirocrew",
                    help="comma list of harnesses whose prices follow the source")
    ap.add_argument("--today", default=datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%d"),
                    help="date written to prices_as_of (default: today, UTC)")
    ap.add_argument("--allow-large-change", action="store_true",
                    help=f"accept a price that moves more than {MAX_CHANGE_FACTOR:g}x against the catalog")
    ap.add_argument("--dry-run", action="store_true")
    a = ap.parse_args()
    datetime.date.fromisoformat(a.today)

    try:
        prices = load_source(a.snapshot)
    except Exception as exc:  # fetch or parse failure: leave the catalog alone
        print(f"sync_catalog_prices: cannot read price source: {exc}", file=sys.stderr)
        return 1

    lines = CATALOG.read_text().splitlines()
    try:
        changes = sync(lines, prices, set(filter(None, a.harnesses.split(","))), a.allow_large_change)
    except PriceError as exc:
        print(f"sync_catalog_prices: refusing to update, catalog untouched:\n{exc}", file=sys.stderr)
        return 1
    for c in changes:
        print(c)
    for key in ("updated", "prices_as_of"):
        if set_top_level(lines, key, a.today):
            print(f"date   {key} -> {a.today}")
    print(f"{len(changes)} price change(s)")
    if not a.dry_run:
        CATALOG.write_text("\n".join(lines) + "\n")
    return 0


if __name__ == "__main__":
    sys.exit(main())
