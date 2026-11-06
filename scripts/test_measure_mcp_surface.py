"""Tests for MCP wire-surface measurement."""
import importlib.util
import pathlib
import tempfile
import unittest

path = pathlib.Path(__file__).with_name("measure-mcp-surface.py")
spec = importlib.util.spec_from_file_location("measure_mcp_surface", path)
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


class SurfaceTest(unittest.TestCase):
    def test_measures_unicode_and_each_tool(self):
        with tempfile.TemporaryDirectory() as directory:
            binary = pathlib.Path(directory) / "server"
            binary.write_text('''#!/usr/bin/env python3
import json, sys
for line in sys.stdin:
    request = json.loads(line)
    result = ({"serverInfo":{"version":"test"},"instructions":"é"}
              if request["id"] == 1 else
              {"tools":[{"name":"one","description":"éx"}]})
    print(json.dumps({"id":request["id"],"result":result}), flush=True)
''')
            binary.chmod(0o755)
            result = module.measure(str(binary))
        self.assertEqual(result["instructions"], {"characters": 1, "utf8_bytes": 2})
        self.assertEqual(result["descriptions"]["one"], {"characters": 2, "utf8_bytes": 3})
        self.assertEqual(result["tool_count"], 1)


if __name__ == "__main__":
    unittest.main()
