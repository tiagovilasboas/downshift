"""Synthetic DS-04 observer-event schema guardrail; no observer implementation."""

from __future__ import annotations

import re
from datetime import datetime
from typing import Any

SCHEMA_VERSION = "harness-downshift.observer.v1"
FIELDS = frozenset({"schema_version", "evidence_state", "observed_at", "lifecycle", "observer_id_hash", "subject_id_hash", "association_state", "correlation_id"})
HASH = re.compile(r"^[a-f0-9]{64}$")
CORRELATION = re.compile(r"^[a-f0-9]{16,64}$")
FORBIDDEN = frozenset({"prompt", "messages", "transcript", "last_message", "summary", "title", "model", "reasoning_effort", "thread_id", "session_id", "agent_id", "error"})


def validate(event: Any) -> bool:
    if not isinstance(event, dict) or set(event) != FIELDS or FORBIDDEN.intersection(event):
        return False
    if event.get("schema_version") != SCHEMA_VERSION or event.get("evidence_state") != "observed":
        return False
    if not _rfc3339_utc(event.get("observed_at")):
        return False
    if event.get("lifecycle") not in {"completed", "needs_attention", "failed"}:
        return False
    if event.get("association_state") not in {"matched", "ambiguous", "unavailable"}:
        return False
    if not all(isinstance(event.get(field), str) and HASH.fullmatch(event[field]) for field in ("observer_id_hash", "subject_id_hash")):
        return False
    correlation = event.get("correlation_id")
    if event["association_state"] == "matched":
        return isinstance(correlation, str) and bool(CORRELATION.fullmatch(correlation))
    return correlation is None


def _rfc3339_utc(value: Any) -> bool:
    if not isinstance(value, str) or len(value) > 40 or not value.endswith("Z"):
        return False
    try:
        datetime.fromisoformat(value[:-1] + "+00:00")
    except ValueError:
        return False
    return True
