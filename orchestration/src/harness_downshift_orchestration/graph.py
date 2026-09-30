"""A local-only LangGraph that plans delegation without choosing models."""

from __future__ import annotations

from typing import Literal
import re

from langgraph.graph import END, START, StateGraph
from typing_extensions import TypedDict

SCHEMA_VERSION = "harness-downshift.orchestration.v1"
MAX_DELEGATES = 8
ALLOWED_SOURCES = frozenset({"cli", "hook", "service"})
REQUEST_FIELDS = frozenset({"source", "task", "delegate_tasks", "max_delegates", "capabilities"})
REQUEST_FIELDS = REQUEST_FIELDS | {"correlation_id"}
CAPABILITY_FIELDS = frozenset({"max_parallel_delegates"})
CORRELATION_ID_PATTERN = re.compile(r"^[a-f0-9]{16,64}$")


class Delegation(TypedDict):
    """A requested subtask; execution remains the caller's responsibility."""

    ordinal: int
    task: str


class DelegationCapabilities(TypedDict, total=False):
    """Caller-owned execution limits; no vendor/model identifiers belong here."""

    max_parallel_delegates: int


class OrchestrationState(TypedDict, total=False):
    """Graph state deliberately excludes provider, model, and credential data."""

    source: str
    correlation_id: str
    task: str
    delegate_tasks: list[str]
    max_delegates: int
    capabilities: DelegationCapabilities
    status: Literal["accepted", "planned", "rejected"]
    error_code: str
    delegations: list[Delegation]
    trace: list[str]


def _validate(state: OrchestrationState) -> OrchestrationState:
    """Validate bounded request metadata before planning."""

    source = state.get("source", "")
    task = state.get("task", "")
    delegate_tasks = state.get("delegate_tasks", [])
    max_delegates = state.get("max_delegates", MAX_DELEGATES)
    capabilities = state.get("capabilities", {})
    correlation_id = state.get("correlation_id", "")
    unknown_fields = set(state).difference(REQUEST_FIELDS)
    if unknown_fields:
        return {"status": "rejected", "error_code": "UNKNOWN_FIELD", "trace": ["validate:rejected"]}
    if not isinstance(source, str) or source not in ALLOWED_SOURCES:
        return {"status": "rejected", "error_code": "INVALID_SOURCE", "trace": ["validate:rejected"]}
    if correlation_id and (not isinstance(correlation_id, str) or not CORRELATION_ID_PATTERN.fullmatch(correlation_id)):
        return {"status": "rejected", "error_code": "INVALID_CORRELATION_ID", "trace": ["validate:rejected"]}
    if not isinstance(task, str) or not task.strip():
        return {"status": "rejected", "error_code": "INVALID_TASK", "trace": ["validate:rejected"]}
    valid_tasks = isinstance(delegate_tasks, list) and all(isinstance(item, str) and item.strip() for item in delegate_tasks)
    if not valid_tasks:
        return {"status": "rejected", "error_code": "INVALID_DELEGATES", "trace": ["validate:rejected"]}
    valid_limit = isinstance(max_delegates, int) and not isinstance(max_delegates, bool) and 0 <= max_delegates <= MAX_DELEGATES
    if not valid_limit:
        return {"status": "rejected", "error_code": "INVALID_MAX_DELEGATES", "trace": ["validate:rejected"]}
    if not isinstance(capabilities, dict):
        return {"status": "rejected", "error_code": "INVALID_CAPABILITIES", "trace": ["validate:rejected"]}
    if set(capabilities).difference(CAPABILITY_FIELDS):
        return {"status": "rejected", "error_code": "UNKNOWN_CAPABILITY", "trace": ["validate:rejected"]}
    capability_limit = capabilities.get("max_parallel_delegates", MAX_DELEGATES)
    if isinstance(capability_limit, bool) or not isinstance(capability_limit, int) or not 0 <= capability_limit <= MAX_DELEGATES:
        return {"status": "rejected", "error_code": "INVALID_CAPABILITY_LIMIT", "trace": ["validate:rejected"]}
    return {"status": "accepted", "trace": ["validate:accepted"]}


def _after_validation(state: OrchestrationState) -> Literal["plan", "finish"]:
    return "plan" if state.get("status") == "accepted" else "finish"


def _plan(state: OrchestrationState) -> OrchestrationState:
    """Produce an execution-free plan with deterministic ordering and bounds."""

    if state.get("status") != "accepted":
        return {"status": "rejected", "error_code": "INVALID_STATE", "trace": ["validate:rejected"]}
    unique_tasks: list[str] = []
    seen: set[str] = set()
    for candidate in state["delegate_tasks"]:
        normalized = candidate.strip()
        if normalized not in seen:
            unique_tasks.append(normalized)
            seen.add(normalized)
    capability_limit = state.get("capabilities", {}).get("max_parallel_delegates", MAX_DELEGATES)
    selected = unique_tasks[: min(state.get("max_delegates", MAX_DELEGATES), capability_limit)]
    return {
        "status": "planned",
        "delegations": [{"ordinal": index, "task": task} for index, task in enumerate(selected, start=1)],
        "trace": ["validate:accepted", "plan:complete"],
    }


def build_graph():
    """Compile the local graph. Nodes never call an LLM, tool, or network."""

    builder = StateGraph(OrchestrationState)
    builder.add_node("validate", _validate)
    builder.add_node("plan", _plan)
    builder.add_edge(START, "validate")
    builder.add_conditional_edges("validate", _after_validation, {"plan": "plan", "finish": END})
    builder.add_edge("plan", END)
    return builder.compile()


def plan(request: OrchestrationState) -> OrchestrationState:
    """Run one local planning turn and reject fields LangGraph would discard."""

    # StateGraph ignores unknown top-level input fields before a node can inspect
    # them, so enforce the external JSON contract at this boundary first.
    if set(request).difference(REQUEST_FIELDS):
        return {"status": "rejected", "error_code": "UNKNOWN_FIELD", "trace": ["validate:rejected"]}
    return build_graph().invoke(request)
