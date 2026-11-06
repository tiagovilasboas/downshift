#!/usr/bin/env python3
"""Measure MCP wire text; this does not measure model tokens or task quality."""
import argparse
import json
import selectors
import subprocess
import sys


def measure(binary):
    requests = [
        {"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {
            "protocolVersion": "2025-03-26", "capabilities": {},
            "clientInfo": {"name": "surface-measurement", "version": "1"}}},
        {"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": {}},
    ]
    responses = {}
    with subprocess.Popen([binary, "mcp", "serve"], stdin=subprocess.PIPE,
                          stdout=subprocess.PIPE, stderr=subprocess.DEVNULL,
                          text=True) as process:
        try:
            with selectors.DefaultSelector() as selector:
                selector.register(process.stdout, selectors.EVENT_READ)
                for request in requests:
                    process.stdin.write(json.dumps(request) + "\n")
                    process.stdin.flush()
                    if not selector.select(timeout=15):
                        raise TimeoutError("MCP server did not respond")
                    response = json.loads(process.stdout.readline())
                    responses[response.get("id")] = response
        finally:
            if process.poll() is None:
                process.kill()
    initialized = responses[1]["result"]
    tools = responses[2]["result"]["tools"]
    instructions = initialized.get("instructions", "")
    descriptions = {tool["name"]: tool.get("description", "") for tool in tools}
    def counts(value):
        return {"characters": len(value), "utf8_bytes": len(value.encode("utf-8"))}
    return {"server_version": initialized["serverInfo"]["version"],
            "instructions": counts(instructions),
            "tool_count": len(descriptions),
            "description_total": counts("".join(descriptions.values())),
            "descriptions": {name: counts(value) for name, value in sorted(descriptions.items())}}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("binary", help="path to the Downshift executable")
    args = parser.parse_args()
    try:
        print(json.dumps(measure(args.binary), indent=2, sort_keys=True))
    except (OSError, subprocess.SubprocessError, ValueError, KeyError) as exc:
        print(f"measurement failed: {exc}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
