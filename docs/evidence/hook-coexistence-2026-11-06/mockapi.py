#!/usr/bin/env python3
"""Local mock of the Anthropic Messages API for hook experiments. Binds 127.0.0.1 only.
Scripted main turn: a request whose first user text contains MARK gets one tool_use
(MOCK_TOOL json). Any other request (subagent, title, follow-up) gets a plain end_turn.
Every request is appended to MOCK_LOG as JSONL (model, tool names, tool_result text)."""
import json, os, sys, re
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
MARK = "RUN-SCENARIO"
TOOL = json.loads(os.environ["MOCK_TOOL"])
LOG = os.environ["MOCK_LOG"]
def text_of(c):
    if isinstance(c, str): return c
    return " ".join(b.get("text", "") if b.get("type") == "text" else
                    (text_of(b.get("content")) if b.get("type") == "tool_result" else "") for b in c)
class H(BaseHTTPRequestHandler):
    def log_message(self, *a): pass
    def _send(self, code, body, ctype="application/json"):
        self.send_response(code); self.send_header("content-type", ctype)
        self.send_header("content-length", str(len(body))); self.end_headers(); self.wfile.write(body)
    def do_GET(self): self._send(200, b"{}")
    def do_POST(self):
        n = int(self.headers.get("content-length", 0)); raw = self.rfile.read(n)
        if self.headers.get("content-encoding") == "gzip":
            import gzip; raw = gzip.decompress(raw)
        try: req = json.loads(raw)
        except Exception: return self._send(200, b"{}")
        if "count_tokens" in self.path: return self._send(200, json.dumps({"input_tokens": 10}).encode())
        msgs = req.get("messages", []); tools = [t.get("name") for t in req.get("tools", [])]
        first = text_of(msgs[0]["content"]) if msgs else ""
        last = msgs[-1] if msgs else {}
        tr = ""
        if isinstance(last.get("content"), list):
            for b in last["content"]:
                if b.get("type") == "tool_result": tr += text_of(b.get("content")) + " "
        main = MARK in first and not tr and len([m for m in msgs if m["role"] == "user"]) == 1
        with open(LOG, "a") as f:
            f.write(json.dumps({"path": self.path, "model": req.get("model"), "tools": tools[:40],
                "first": first[:100], "tail": first[-140:], "tag_left": "[model:" in first, "tool_result": tr.strip()[:400], "scripted_tool_use": main}) + "\n")
        if main:
            content = [{"type": "tool_use", "id": "toolu_mock1", "name": TOOL["name"], "input": TOOL["input"]}]; stop = "tool_use"
        else:
            content = [{"type": "text", "text": "ok"}]; stop = "end_turn"
        msg = {"id": "msg_mock", "type": "message", "role": "assistant", "model": req.get("model"),
               "content": content, "stop_reason": stop, "stop_sequence": None,
               "usage": {"input_tokens": 10, "output_tokens": 5}}
        if not req.get("stream"): return self._send(200, json.dumps(msg).encode())
        ev = []
        def e(t, d): ev.append(f"event: {t}\ndata: {json.dumps(d)}\n\n")
        e("message_start", {"type": "message_start", "message": {**msg, "content": [], "stop_reason": None}})
        b = content[0]
        if b["type"] == "tool_use":
            e("content_block_start", {"type": "content_block_start", "index": 0, "content_block": {**b, "input": {}}})
            e("content_block_delta", {"type": "content_block_delta", "index": 0, "delta": {"type": "input_json_delta", "partial_json": json.dumps(b["input"])}})
        else:
            e("content_block_start", {"type": "content_block_start", "index": 0, "content_block": {"type": "text", "text": ""}})
            e("content_block_delta", {"type": "content_block_delta", "index": 0, "delta": {"type": "text_delta", "text": "ok"}})
        e("content_block_stop", {"type": "content_block_stop", "index": 0})
        e("message_delta", {"type": "message_delta", "delta": {"stop_reason": stop, "stop_sequence": None}, "usage": {"output_tokens": 5}})
        e("message_stop", {"type": "message_stop"})
        self._send(200, "".join(ev).encode(), "text/event-stream")
if __name__ == "__main__":
    srv = ThreadingHTTPServer(("127.0.0.1", int(sys.argv[1])), H); srv.serve_forever()
