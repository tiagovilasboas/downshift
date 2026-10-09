#!/usr/bin/env python3
# Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
# SPDX-License-Identifier: Apache-2.0
"""Fixture-only tests for the manual Antigravity evidence sensor.

harness-downshift by Tiago de Carvalho Vilas Boas
https://github.com/tiagovilasboas/downshift
"""

import importlib.util
import json
import sqlite3
import subprocess
import sys
import tempfile
import unittest
from contextlib import closing
from pathlib import Path


SCRIPT = Path(__file__).with_name("collect-antigravity-evidence.py")
SPEC = importlib.util.spec_from_file_location("antigravity_evidence", SCRIPT)
SENSOR = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(SENSOR)


def varint(value):
    encoded = bytearray()
    while value >= 128:
        encoded.append((value & 127) | 128)
        value >>= 7
    return bytes(encoded + bytes([value]))


def field(number, payload):
    return varint((number << 3) | 2) + varint(len(payload)) + payload


def nested(path, payload):
    for number in reversed(path):
        payload = field(number, payload)
    return payload


def invocation(model):
    arguments = {"Subagents": [{"Model": model, "Prompt": "private-prompt-sentinel"}]}
    return field(2, b"invoke_subagent") + field(3, json.dumps(arguments).encode())


def make_parent(connection, child_ref="private-child-id", requested="pro", applied="flash_lite"):
    connection.executescript("CREATE TABLE trajectory_meta(cascade_id TEXT); CREATE TABLE steps(idx INTEGER, step_payload BLOB);")
    connection.execute("INSERT INTO trajectory_meta VALUES (?)", ("private-parent-id",))
    payload = nested((5,), field(4, invocation(requested)) + field(29, invocation(applied)))
    payload += nested((140, 2, 6, 2, 143, 10, 1), child_ref.encode())
    connection.execute("INSERT INTO steps VALUES (?, ?)", (8, payload))
    connection.commit()


def make_child(connection):
    connection.executescript("CREATE TABLE trajectory_meta(cascade_id TEXT); CREATE TABLE executor_metadata(idx INTEGER, data BLOB); CREATE TABLE gen_metadata(idx INTEGER, data BLOB);")
    connection.execute("INSERT INTO trajectory_meta VALUES (?)", ("private-child-id",))
    connection.execute("INSERT INTO executor_metadata VALUES (?, ?)", (0, nested((10, 1, 28), b"gemini-3.5-flash-lite")))
    for index in range(10):
        connection.execute("INSERT INTO gen_metadata VALUES (?, ?)", (index, nested((1, 19), b"gemini-3.5-flash-lite")))
    connection.commit()


class EvidenceSensorTests(unittest.TestCase):
    def setUp(self):
        self.parent = sqlite3.connect(":memory:")
        self.child = sqlite3.connect(":memory:")
        self.addCleanup(self.parent.close)
        self.addCleanup(self.child.close)
        make_parent(self.parent)
        make_child(self.child)

    def test_valid_chain_has_executor_and_generation_proof_without_private_content(self):
        report = SENSOR.collect(self.parent, self.child, 8)
        self.assertEqual(report["project"], "harness-downshift by Tiago de Carvalho Vilas Boas")
        self.assertEqual(report["repository"], "https://github.com/tiagovilasboas/downshift")
        self.assertEqual(report["requested_alias"], "pro")
        self.assertEqual(report["applied_alias"], "flash_lite")
        self.assertEqual(report["effective_model"], "gemini-3.5-flash-lite")
        self.assertTrue(report["child_reference_matches"])
        self.assertEqual(len(report["executor_models"]), 1)
        self.assertEqual(len(report["generation_models"]), 10)
        for key in ("parent_id_sha256", "child_id_sha256", "parent_step_sha256"):
            self.assertRegex(report[key], r"^[0-9a-f]{64}$")
        exported = json.dumps(report)
        for secret in ("private-parent-id", "private-child-id", "private-prompt-sentinel"):
            self.assertNotIn(secret, exported)

    def test_wrong_child_reference_is_rejected(self):
        self.child.execute("UPDATE trajectory_meta SET cascade_id='different-child'")
        with self.assertRaisesRegex(ValueError, "does not reference this child"):
            SENSOR.collect(self.parent, self.child, 8)

    def test_mismatched_executor_and_generation_models_are_rejected(self):
        self.child.execute("UPDATE gen_metadata SET data=? WHERE idx=9", (nested((1, 19), b"another-model"),))
        with self.assertRaisesRegex(ValueError, "inconsistent models"):
            SENSOR.collect(self.parent, self.child, 8)

    def test_missing_generation_or_executor_cannot_prove_honor(self):
        for table in ("gen_metadata", "executor_metadata"):
            with self.subTest(table=table):
                self.child.execute("SAVEPOINT missing_rows")
                self.child.execute("DELETE FROM " + table)
                with self.assertRaisesRegex(ValueError, "missing executor or generation"):
                    SENSOR.collect(self.parent, self.child, 8)
                self.child.execute("ROLLBACK TO missing_rows")

    def test_unchanged_aliases_cannot_prove_a_rewrite(self):
        payload = nested((5,), field(4, invocation("pro")) + field(29, invocation("pro")))
        payload += nested((140, 2, 6, 2, 143, 10, 1), b"private-child-id")
        self.parent.execute("UPDATE steps SET step_payload=?", (payload,))
        with self.assertRaisesRegex(ValueError, "no applied rewrite"):
            SENSOR.collect(self.parent, self.child, 8)

    def test_ambiguous_singular_fields_are_rejected(self):
        with self.assertRaisesRegex(ValueError, "ambiguous singular"):
            SENSOR.at(field(2, b"one") + field(2, b"two"), 2)

    def test_malformed_or_oversized_protobuf_is_rejected(self):
        cases = (b"\x80", b"\x00", b"\x0e", b"\x0a\x03a", b"\xff" * 10,
                 b"x" * (SENSOR.MAX_BLOB + 1), "not-bytes")
        for payload in cases:
            with self.subTest(size=len(payload)):
                with self.assertRaises(ValueError):
                    SENSOR.fields(payload)

    def test_invalid_model_identifier_is_rejected(self):
        self.child.execute("UPDATE executor_metadata SET data=?", (nested((10, 1, 28), b"model\nprivate-prompt-sentinel"),))
        with self.assertRaisesRegex(ValueError, "invalid model identifier"):
            SENSOR.collect(self.parent, self.child, 8)

    def test_row_bound_is_enforced(self):
        for index in range(10, 257):
            self.child.execute("INSERT INTO gen_metadata VALUES (?, ?)", (index, nested((1, 19), b"gemini-3.5-flash-lite")))
        with self.assertRaisesRegex(ValueError, "row bound"):
            SENSOR.collect(self.parent, self.child, 8)

    def test_cli_failure_omits_private_paths_and_traceback(self):
        with tempfile.TemporaryDirectory(prefix="private-path-sentinel-") as directory:
            parent_path = Path(directory) / "parent.sqlite"
            child_path = Path(directory) / "child.sqlite"
            for path, fixture in ((parent_path, make_parent), (child_path, make_child)):
                with closing(sqlite3.connect(path)) as connection:
                    fixture(connection)
            with closing(sqlite3.connect(parent_path)) as connection:
                connection.execute("UPDATE steps SET step_payload=?", (b"\x80",))
                connection.commit()
            result = subprocess.run([sys.executable, str(SCRIPT), "--parent-db", str(parent_path), "--child-db", str(child_path), "--step-index", "8"], capture_output=True, text=True, check=False)
            self.assertEqual(result.returncode, 1)
            self.assertEqual(result.stdout, "")
            self.assertEqual(result.stderr.strip(), "Antigravity evidence unavailable: unsupported schema or inconsistent chain")
            self.assertNotIn(directory, result.stderr)

    def test_cli_deeply_nested_json_fails_with_sanitized_error(self):
        with tempfile.TemporaryDirectory(prefix="private-path-sentinel-") as directory:
            parent_path = Path(directory) / "parent.sqlite"
            child_path = Path(directory) / "child.sqlite"
            for path, fixture in ((parent_path, make_parent), (child_path, make_child)):
                with closing(sqlite3.connect(path)) as connection:
                    fixture(connection)
            arguments = b'{"Subagents":' + b'[' * 1200 + b'0' + b']' * 1200 + b'}'
            call = field(2, b"invoke_subagent") + field(3, arguments)
            with closing(sqlite3.connect(parent_path)) as connection:
                connection.execute("UPDATE steps SET step_payload=?", (nested((5, 4), call),))
                connection.commit()
            result = subprocess.run([sys.executable, str(SCRIPT), "--parent-db", str(parent_path), "--child-db", str(child_path), "--step-index", "8"], capture_output=True, text=True, check=False)
            self.assertEqual(result.returncode, 1)
            self.assertEqual(result.stdout, "")
            self.assertEqual(result.stderr.strip(), "Antigravity evidence unavailable: unsupported schema or inconsistent chain")

    def test_connection_is_read_only(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "fixture.sqlite"
            with closing(sqlite3.connect(path)) as connection:
                connection.execute("CREATE TABLE evidence(value TEXT)")
            connection = SENSOR.connect(path)
            try:
                with self.assertRaises(sqlite3.OperationalError):
                    connection.execute("INSERT INTO evidence VALUES ('mutation')")
            finally:
                connection.close()


if __name__ == "__main__":
    unittest.main()
