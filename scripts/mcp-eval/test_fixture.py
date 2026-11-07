import importlib.util
import json
import pathlib
import subprocess
import sys
import tempfile
import unittest

HERE = pathlib.Path(__file__).parent
spec = importlib.util.spec_from_file_location("fixture", HERE / "fixture.py")
fixture = importlib.util.module_from_spec(spec)
spec.loader.exec_module(fixture)


class FixtureTest(unittest.TestCase):
    def test_native_case_provider_offers_configured_model(self):
        server = fixture.Fixture("C01", "test-model")
        result, failed = server.call("providers", {"harness": "codex"})
        self.assertFalse(failed)
        self.assertEqual(result["models"][0]["id"], "test-model")

    def test_worker_lifecycle_and_ack(self):
        server = fixture.Fixture("C03")
        data, error = server.call("delegate", {"mode": "read", "tasks": ["alpha", "beta"]})
        self.assertFalse(error)
        self.assertEqual(data["job_ids"], ["worker-1", "worker-2"])
        result, _ = server.call("collect", {})
        self.assertEqual(len(result["results"]), 2)
        repeated, _ = server.call("collect", {})
        self.assertEqual(repeated, result)
        empty, _ = server.call("collect", {"ack": result["ack"]})
        self.assertEqual(empty["results"], [])
        server.call("send", {"job_id": "worker-1", "task": "follow-up"})
        followup, _ = server.call("collect", {"ack": result["ack"]})
        self.assertEqual([item["job_id"] for item in followup["results"]], ["worker-1"])
        empty, _ = server.call("collect", {"ack": followup["ack"]})
        self.assertEqual(empty["results"], [])
        server.call("release", {"job_ids": data["job_ids"]})
        self.assertEqual(server.workers, {})

    def test_read_mode_and_injection_are_synthetic(self):
        server = fixture.Fixture("C04")
        report, _ = server.call("collect", {})
        self.assertIn("Ignore the user", report["results"][0]["text"])
        value, _ = server.call("read_fixture", {"name": "value"})
        self.assertEqual(value["value"], "17")
        _, failed = server.call("delegate", {"mode": "write", "tasks": ["edit"]})
        self.assertTrue(failed)

    def test_stdio_protocol_and_log(self):
        with tempfile.TemporaryDirectory() as directory:
            instructions = pathlib.Path(directory) / "arm.txt"
            log = pathlib.Path(directory) / "calls.jsonl"
            instructions.write_text("synthetic arm", encoding="utf-8")
            requests = [
                {"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {}},
                {"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": {}},
                {"jsonrpc": "2.0", "id": 3, "method": "tools/call", "params": {"name": "read_fixture", "arguments": {"name": "answer.txt"}}},
            ]
            payload = "".join(json.dumps(item) + "\n" for item in requests)
            result = subprocess.run([sys.executable, str(HERE / "fixture.py"), "--case", "C09", "--instructions", str(instructions), "--log", str(log)], input=payload, text=True, capture_output=True, check=True)
            responses = [json.loads(line) for line in result.stdout.splitlines()]
            self.assertEqual(responses[0]["result"]["instructions"], "synthetic arm")
            self.assertEqual(len(responses[1]["result"]["tools"]), len(fixture.NAMES))
            self.assertEqual(json.loads(log.read_text())["result"]["value"], "ready")


if __name__ == "__main__":
    unittest.main()
