#!/usr/bin/env python3
"""mksettings.py <out> <order>   (env: DOWNSHIFT_BIN, FX_DIR, FX_ROOT; optional DELAY, NO_ALLOW)

Writes a user settings.json with the PreToolUse entries of the named hooks, in the
given order (comma list of downshift,rtk,cheap). Entry shapes:
  downshift: what `downshift setup` writes (one group per matcher)
  rtk:       the published rtk-rewrite.sh run unmodified (against the fixture rtk stub)
  cheap:     what `cheap_subagents.py --install` writes (exec form: command + args)
DELAY=name=secs[,name=secs] wraps that hook in delay-wrap.sh (timing probe).
NO_ALLOW=1 omits the Bash/Agent allow rule so a hook's own `allow` has to supply it."""
import json, os, sys
out, order = sys.argv[1], sys.argv[2].split(",")
bin_, fx, root = os.environ["DOWNSHIFT_BIN"], os.environ["FX_DIR"], os.environ["FX_ROOT"]
hooks = root + "/hooks"
ent = {
  "downshift": [
    {"matcher": "Agent|Task", "hooks": [{"type": "command", "command": f"{bin_} claude-code"}]},
    {"matcher": "Bash", "hooks": [{"type": "command", "command": f"{bin_} claude-code"}]}],
  "rtk": [{"matcher": "Bash", "hooks": [{"type": "command", "command": f"bash {hooks}/rtk-rewrite-claude.sh"}]}],
  "cheap": [{"matcher": "Agent", "hooks": [{"type": "command", "command": sys.executable,
             "args": [hooks + "/cheap_subagents.py"], "timeout": 10, "statusMessage": "routing subagent model"}]}],
}
for kv in filter(None, os.environ.get("DELAY", "").split(",")):
    name, secs = kv.split("=")
    for g in ent[name]:
        for h in g["hooks"]:
            if "args" in h:   # keep the exec form
                h["args"] = [f"{fx}/delay-wrap.sh", secs, h["command"]] + h["args"]; h["command"] = "bash"
            else:
                h["command"] = f"bash {fx}/delay-wrap.sh {secs} " + h["command"]
pre = [g for name in order for g in ent[name]]
perm = {} if os.environ.get("NO_ALLOW") else {"allow": ["Bash", "Agent"]}
json.dump({"permissions": perm, "hooks": {"PreToolUse": pre}}, open(out, "w"), indent=2)
