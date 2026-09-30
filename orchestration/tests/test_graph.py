import unittest
import json
import os
import subprocess
import sys
import tempfile

from harness_downshift_orchestration import SCHEMA_VERSION, plan
from harness_downshift_orchestration.telemetry import record


class OrchestrationGraphTests(unittest.TestCase):
    def run_cli(self, payload: bytes) -> tuple[subprocess.CompletedProcess[bytes], dict, dict]:
        with tempfile.TemporaryDirectory() as directory:
            event_path = os.path.join(directory, "events.jsonl")
            environment = os.environ | {"DOWNSHIFT_ORCHESTRATION_TELEMETRY_PATH": event_path}
            completed = subprocess.run(
                [sys.executable, "-m", "harness_downshift_orchestration"],
                input=payload,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                check=False,
                env=environment,
            )
            response = json.loads(completed.stdout)
            with open(event_path, encoding="utf-8") as stream:
                event = json.loads(stream.read())
            return completed, response, event

    def test_plans_unique_tasks_within_requested_bound(self) -> None:
        result = plan({"source": "hook", "task": "Investigate a bounded change", "delegate_tasks": ["inspect API", "inspect API", "run tests"], "max_delegates": 1})
        self.assertEqual(result["status"], "planned")
        self.assertEqual(result["delegations"], [{"ordinal": 1, "task": "inspect API"}])
        self.assertEqual(result["trace"], ["validate:accepted", "plan:complete"])

    def test_rejects_invalid_request_without_a_plan(self) -> None:
        result = plan({"source": "hook", "task": "", "delegate_tasks": []})
        self.assertEqual(result["status"], "rejected")
        self.assertEqual(result["error_code"], "INVALID_TASK")
        self.assertNotIn("delegations", result)

    def test_schema_version_is_stable(self) -> None:
        self.assertEqual(SCHEMA_VERSION, "harness-downshift.orchestration.v1")

    def test_honours_vendor_neutral_capability_limit(self) -> None:
        result = plan({"source": "service", "task": "Plan work", "delegate_tasks": ["one", "two"], "max_delegates": 2, "capabilities": {"max_parallel_delegates": 1}})
        self.assertEqual(result["delegations"], [{"ordinal": 1, "task": "one"}])

    def test_preserves_valid_correlation_id(self) -> None:
        correlation_id = "0123456789abcdef0123456789abcdef"
        result = plan({"source": "hook", "correlation_id": correlation_id, "task": "Plan work", "delegate_tasks": []})
        self.assertEqual(result["correlation_id"], correlation_id)

    def test_rejects_invalid_correlation_id(self) -> None:
        result = plan({"source": "hook", "correlation_id": "untrusted prompt", "task": "Plan work", "delegate_tasks": []})
        self.assertEqual(result["error_code"], "INVALID_CORRELATION_ID")

    def test_rejects_unknown_contract_fields(self) -> None:
        result = plan({"source": "cli", "task": "Plan work", "delegate_tasks": [], "provider": "untrusted"})
        self.assertEqual(result["error_code"], "UNKNOWN_FIELD")

    def test_rejects_non_string_source_without_throwing(self) -> None:
        result = plan({"source": ["hook"], "task": "Plan work", "delegate_tasks": []})
        self.assertEqual(result["error_code"], "INVALID_SOURCE")

    def test_telemetry_excludes_task_text(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            path = os.path.join(directory, "events.jsonl")
            previous = os.environ.get("DOWNSHIFT_ORCHESTRATION_TELEMETRY_PATH")
            os.environ["DOWNSHIFT_ORCHESTRATION_TELEMETRY_PATH"] = path
            try:
                record({"status": "planned", "delegations": [{"ordinal": 1, "task": "private task"}]})
            finally:
                if previous is None:
                    del os.environ["DOWNSHIFT_ORCHESTRATION_TELEMETRY_PATH"]
                else:
                    os.environ["DOWNSHIFT_ORCHESTRATION_TELEMETRY_PATH"] = previous
            with open(path, encoding="utf-8") as stream:
                event = json.loads(stream.read())
            self.assertEqual(event["planned_delegations"], 1)
            self.assertNotIn("private task", event.values())

    def test_cli_preserves_correlation_id_in_stdout(self) -> None:
        correlation_id = "0123456789abcdef0123456789abcdef"
        completed, response, event = self.run_cli(json.dumps({"source": "hook", "correlation_id": correlation_id, "task": "Plan work", "delegate_tasks": []}).encode())
        self.assertEqual(completed.returncode, 0)
        self.assertEqual(response["correlation_id"], correlation_id)
        self.assertEqual(event["status"], "planned")

    def test_cli_early_rejections_are_prompt_free_telemetry(self) -> None:
        cases = [
            (b"{", "INVALID_JSON"),
            (b"[]", "INVALID_REQUEST"),
            (b"x" * ((1 << 20) + 1), "INPUT_TOO_LARGE"),
        ]
        for payload, error_code in cases:
            with self.subTest(error_code=error_code):
                completed, response, event = self.run_cli(payload)
                self.assertEqual(completed.returncode, 2)
                self.assertEqual(response["error_code"], error_code)
                self.assertEqual(event["status"], "rejected")
                self.assertEqual(event["error_code"], error_code)
                self.assertEqual(event["planned_delegations"], 0)
                self.assertNotIn("task", event)
