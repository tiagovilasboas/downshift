#!/usr/bin/env python3
# Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
# SPDX-License-Identifier: Apache-2.0
"""Manual, read-only evidence sensor for Antigravity 2.21.1 local SQLite.

harness-downshift by Tiago de Carvalho Vilas Boas
https://github.com/tiagovilasboas/downshift

This private storage schema is build-specific. It is never used by routing or
automatic honor telemetry. Output omits prompts, paths, and raw conversation IDs.
"""

import argparse
import hashlib
import json
import re
import sqlite3
import sys
from datetime import datetime, timezone
from pathlib import Path


MAX_BLOB = 2 * 1024 * 1024
MODEL = re.compile(r"[A-Za-z0-9][A-Za-z0-9_.:/-]{0,127}\Z")


def digest(value):
    return hashlib.sha256(value.encode() if isinstance(value, str) else value).hexdigest()


def fields(blob):
    if not isinstance(blob, bytes) or len(blob) > MAX_BLOB:
        raise ValueError("unsupported or oversized metadata")
    pos = 0
    result = {}

    def varint():
        nonlocal pos
        value = 0
        for shift in range(0, 70, 7):
            if pos >= len(blob):
                raise ValueError("truncated varint")
            byte = blob[pos]
            pos += 1
            value |= (byte & 127) << shift
            if byte < 128:
                if shift == 63 and byte > 1:
                    raise ValueError("overflowing varint")
                return value
        raise ValueError("overflowing varint")

    while pos < len(blob):
        key = varint()
        number, wire = key >> 3, key & 7
        if not 0 < number < (1 << 29):
            raise ValueError("invalid field number")
        if wire == 0:
            value = varint()
        elif wire in (1, 2, 5):
            size = varint() if wire == 2 else (8 if wire == 1 else 4)
            if size > len(blob) - pos:
                raise ValueError("truncated field")
            value = blob[pos:pos + size]
            pos += size
        else:
            raise ValueError("unsupported wire type")
        result.setdefault(number, []).append(value)
    return result


def at(blob, *path):
    for number in path:
        values = fields(blob).get(number, [])
        if not values:
            return None
        if len(values) != 1:
            raise ValueError("ambiguous singular field")
        blob = values[0]
    return blob


def model(value):
    if not isinstance(value, str) or not MODEL.fullmatch(value):
        raise ValueError("invalid model identifier")
    return value


def invocation(blob, path):
    name = at(blob, *path, 2)
    args = at(blob, *path, 3)
    if name != b"invoke_subagent" or args is None:
        raise ValueError("missing invoke_subagent evidence")
    data = json.loads(args)
    agents = data.get("Subagents")
    if not isinstance(agents, list) or len(agents) != 1:
        raise ValueError("evidence sensor requires exactly one child")
    return model(agents[0].get("Model"))


def connect(path):
    resolved = Path(path).resolve(strict=True)
    if not resolved.is_file():
        raise ValueError("expected a regular SQLite file")
    return sqlite3.connect(resolved.as_uri() + "?mode=ro", uri=True)


def collect(parent, child, step_index):
    parent_id = parent.execute("SELECT cascade_id FROM trajectory_meta").fetchall()
    child_id = child.execute("SELECT cascade_id FROM trajectory_meta").fetchall()
    if len(parent_id) != 1 or len(child_id) != 1:
        raise ValueError("ambiguous trajectory identity")
    parent_id, child_id = parent_id[0][0], child_id[0][0]
    row = parent.execute("SELECT step_payload FROM steps WHERE idx=?", (step_index,)).fetchone()
    if row is None:
        raise ValueError("parent step missing")
    step = row[0]
    requested = invocation(step, (5, 4))
    applied = invocation(step, (5, 29))
    child_reference = at(step, 140, 2, 6, 2, 143, 10, 1)
    if child_reference != child_id.encode():
        raise ValueError("parent step does not reference this child")

    executor_rows = child.execute("SELECT idx,data FROM executor_metadata ORDER BY idx LIMIT 257").fetchall()
    generation_rows = child.execute("SELECT idx,data FROM gen_metadata ORDER BY idx LIMIT 257").fetchall()
    if len(executor_rows) > 256 or len(generation_rows) > 256:
        raise ValueError("evidence exceeds manual sensor row bound")
    executors = []
    generations = []
    for rows, output, path in [(executor_rows, executors, (10, 1, 28)),
                               (generation_rows, generations, (1, 19))]:
        for index, blob in rows:
            value = at(blob, *path)
            if value is not None:
                output.append({"row": index, "model": model(value.decode()),
                               "blob_sha256": digest(blob)})
    if not executors or not generations:
        raise ValueError("missing executor or generation model evidence")
    effective = {item["model"] for item in executors + generations}
    if len(effective) != 1 or requested == applied:
        raise ValueError("inconsistent models or no applied rewrite")
    return {
        "schema_version": 1,
        "project": "harness-downshift by Tiago de Carvalho Vilas Boas",
        "repository": "https://github.com/tiagovilasboas/downshift",
        "observed_at": datetime.now(timezone.utc).isoformat(),
        "source": "manual_antigravity_sqlite",
        "storage_contract": "Antigravity 2.21.1; private protobuf field paths",
        "parent_id_sha256": digest(parent_id),
        "child_id_sha256": digest(child_id),
        "parent_step": step_index,
        "parent_step_sha256": digest(step),
        "requested_alias": requested,
        "applied_alias": applied,
        "effective_model": next(iter(effective)),
        "child_reference_matches": True,
        "executor_models": executors,
        "generation_models": generations,
        "field_paths": {"original_call": "5.4", "applied_call": "5.29",
                        "child_reference": "140.2.6.2.143.10.1",
                        "executor_model": "10.1.28", "generation_model": "1.19"},
        "limits": ["one observed child; no pool membership or billing proof",
                   "offline manual evidence; does not append honor telemetry",
                   "private schema requires revalidation per app build"],
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--parent-db", required=True)
    parser.add_argument("--child-db", required=True)
    parser.add_argument("--step-index", required=True, type=int)
    args = parser.parse_args()
    parent = child = None
    try:
        parent = connect(args.parent_db)
        child = connect(args.child_db)
        report = collect(parent, child, args.step_index)
    except (ValueError, TypeError, AttributeError, KeyError, RecursionError,
            OSError, sqlite3.Error):
        print("Antigravity evidence unavailable: unsupported schema or inconsistent chain", file=sys.stderr)
        return 1
    finally:
        if parent is not None:
            parent.close()
        if child is not None:
            child.close()
    print(json.dumps(report, indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    sys.exit(main())
