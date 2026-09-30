"""JSON-lines CLI for the local LangGraph delegation planner."""

from __future__ import annotations

import json
import signal
import sys
from typing import Any

from .graph import SCHEMA_VERSION, plan
from .telemetry import record

MAX_INPUT_BYTES = 1 << 20
INPUT_TIMEOUT_SECONDS = 2


class InputTimeoutError(Exception):
    """Raised when stdin does not complete within the local CLI deadline."""


def _read_request() -> bytes:
    def on_timeout(_signum: int, _frame: Any) -> None:
        raise InputTimeoutError

    previous_handler = signal.signal(signal.SIGALRM, on_timeout)
    signal.setitimer(signal.ITIMER_REAL, INPUT_TIMEOUT_SECONDS)
    try:
        return sys.stdin.buffer.read(MAX_INPUT_BYTES + 1)
    finally:
        signal.setitimer(signal.ITIMER_REAL, 0)
        signal.signal(signal.SIGALRM, previous_handler)


def _response(state: dict[str, Any]) -> dict[str, Any]:
    response: dict[str, Any] = {
        "schema_version": SCHEMA_VERSION,
        "status": state.get("status", "rejected"),
        "correlation_id": state.get("correlation_id", ""),
        "delegations": state.get("delegations", []),
        "trace": state.get("trace", []),
        "routing_boundary": "tier_and_model_decisions_remain_in_go",
    }
    if "error_code" in state:
        response["error_code"] = state["error_code"]
    return response


def main() -> int:
    def reject(error_code: str) -> int:
        state = {"status": "rejected", "error_code": error_code}
        record(state)
        print(json.dumps(_response(state), separators=(",", ":")))
        return 2

    try:
        raw = _read_request()
    except InputTimeoutError:
        return reject("INPUT_TIMEOUT")
    if len(raw) > MAX_INPUT_BYTES:
        return reject("INPUT_TOO_LARGE")
    try:
        request = json.loads(raw)
    except json.JSONDecodeError:
        return reject("INVALID_JSON")
    if not isinstance(request, dict):
        return reject("INVALID_REQUEST")
    state = plan(request)
    record(state)
    print(json.dumps(_response(state), separators=(",", ":")))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
