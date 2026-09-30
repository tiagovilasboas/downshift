"""Prompt-free operational telemetry for the local planner."""

from __future__ import annotations

import json
import os
from datetime import UTC, datetime
from pathlib import Path
from typing import Any


def _telemetry_path() -> Path:
    configured = os.environ.get("DOWNSHIFT_ORCHESTRATION_TELEMETRY_PATH")
    if configured:
        return Path(configured)
    return Path.home() / ".harness-downshift" / "orchestration-events.jsonl"


def record(state: dict[str, Any]) -> None:
    """Append aggregate counts only; planning errors must not break the caller."""

    event = {
        "timestamp": datetime.now(UTC).isoformat(),
        "status": state.get("status", "rejected"),
        "error_code": state.get("error_code"),
        "planned_delegations": len(state.get("delegations", [])),
    }
    try:
        path = _telemetry_path()
        path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
        flags = os.O_APPEND | os.O_CREAT | os.O_WRONLY
        descriptor = os.open(path, flags, 0o600)
        with os.fdopen(descriptor, "a", encoding="utf-8") as stream:
            stream.write(json.dumps(event, separators=(",", ":")) + "\n")
    except OSError:
        pass
