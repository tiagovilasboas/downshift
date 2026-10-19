#!/usr/bin/env python3
# Copyright (c) 2026 Tiago Vilas Boas.
# SPDX-License-Identifier: Apache-2.0
"""Tests for the catalog price sync: range and move validation.

Downshift by Tiago Vilas Boas
https://github.com/tiagovilasboas/downshift
"""

import contextlib
import importlib.util
import io
import json
import sys
import tempfile
import unittest
from pathlib import Path

SCRIPT = Path(__file__).with_name("sync_catalog_prices.py")
SPEC = importlib.util.spec_from_file_location("sync_catalog_prices", SCRIPT)
SYNC = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(SYNC)

CATALOG = """{
  "version": "1",
  "updated": "2026-01-01",
  "entries": [
    {
      "id": "claude-test-1",
      "harness": "claude-code",
      "tier": "mid",
      "input_cost_per_1m": 3.00,
      "output_cost_per_1m": 15.00
    }
  ]
}
"""


def snapshot(prompt, completion):
    return json.dumps({"data": [{"id": "anthropic/claude-test-1",
                                 "pricing": {"prompt": prompt, "completion": completion}}]})


class SyncValidation(unittest.TestCase):
    def run_sync(self, prompt, completion, *extra):
        with tempfile.TemporaryDirectory() as d:
            cat = Path(d) / "catalog.json"
            cat.write_text(CATALOG)
            snap = Path(d) / "snap.json"
            snap.write_text(snapshot(prompt, completion))
            SYNC.CATALOG = cat
            argv = ["sync", "--snapshot", str(snap), "--today", "2026-10-19", *extra]
            err = io.StringIO()
            old = sys.argv
            sys.argv = argv
            try:
                with contextlib.redirect_stderr(err), contextlib.redirect_stdout(io.StringIO()):
                    rc = SYNC.main()
            finally:
                sys.argv = old
            return rc, err.getvalue(), cat.read_text()

    def assertRefused(self, prompt, completion, *extra, needle=""):
        rc, err, text = self.run_sync(prompt, completion, *extra)
        self.assertEqual(rc, 1, err)
        self.assertIn("catalog untouched", err)
        self.assertIn(needle, err)
        self.assertEqual(text, CATALOG, "catalog must not change on a refusal")

    def test_valid_small_move_is_written(self):
        rc, _, text = self.run_sync("0.0000035", "0.000015")
        self.assertEqual(rc, 0)
        self.assertIn('"input_cost_per_1m": 3.5', text)
        self.assertIn('"prices_as_of": "2026-10-19"', text)

    def test_negative_price(self):
        self.assertRefused("-0.000004", "0.000015", needle="greater than zero")

    def test_nan_and_infinity(self):
        self.assertRefused("NaN", "0.000015", needle="not finite")
        self.assertRefused("0.000003", "inf", needle="not finite")

    def test_not_a_number(self):
        self.assertRefused("abc", "0.000015", needle="not a number")
        self.assertRefused({"x": 1}, "0.000015", needle="not a number")

    def test_unit_mistake_1000x(self):
        self.assertRefused("0.003", "0.000015", needle="exceeds")

    def test_large_move_needs_the_flag(self):
        self.assertRefused("0.00002", "0.000015", needle="--allow-large-change")  # 6.7x up
        self.assertRefused("0.0000005", "0.000015", needle="--allow-large-change")  # 6x down
        rc, _, text = self.run_sync("0.00002", "0.000015", "--allow-large-change")
        self.assertEqual(rc, 0)
        self.assertIn('"input_cost_per_1m": 20.0', text)

    def test_move_inside_the_limit_passes(self):
        rc, _, _ = self.run_sync("0.000012", "0.000015")  # 4x up
        self.assertEqual(rc, 0)

    def test_every_bad_price_is_reported(self):
        rc, err, _ = self.run_sync("-1", "abc")
        self.assertEqual(rc, 1)
        self.assertIn("input_cost_per_1m", err)
        self.assertIn("output_cost_per_1m", err)


if __name__ == "__main__":
    unittest.main()
