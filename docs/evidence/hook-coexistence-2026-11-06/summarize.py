#!/usr/bin/env python3
"""summarize.py <run-dir> <bash|agent> -> one TSV line of what the run showed."""
import json, re, sys
R, kind = sys.argv[1], sys.argv[2]
rows = [json.loads(l) for l in open(R + "/mock.jsonl") if l.strip()]
dbg = open(R + "/debug.log", errors="replace").read()
fix = open(R + "/fixture.log").read()
# order in which the hooks returned, from Claude Code's own debug log
fin = []
for line in dbg.splitlines():
    if "finished with status" in line and "PreToolUse" in line:
        fin.append("rtk" if "rtk-rewrite" in line else "cheap" if "cheap_subagents" in line else "downshift")
# permission outcome of the scripted call
res = [r["tool_result"] for r in rows if r["tool_result"]]
denied = bool(json.load(open(R + "/claude.out")).get("permission_denials"))
doc = json.load(open(R + "/doctor.json"))
overlap = [c for c in doc.get("checks", []) if c.get("id") == "hook overlap"]
dr = ";".join(c["detail"].split(" also rewrites ")[0] for c in overlap) or "-"
if kind == "bash":
    go = re.search(r"go-ran argv: (.*?) \| parent: (.*)", fix)
    parent = go.group(2) if go else ""
    rtk_ran = "rtk-ran" in fix
    compact = re.search(r"\bcompact\b", fix) is not None     # the downshift subcommand, not a path
    if go and compact and not rtk_ran: winner = "downshift"
    elif go and rtk_ran and not compact: winner = "rtk"
    elif go and rtk_ran and compact: winner = "BOTH"
    elif go: winner = "plain"
    else: winner = "none-ran"
    dbl = "YES" if (rtk_ran and compact) else "no"
    code = re.search(r"Exit code (\d+)", res[0]).group(1) if res and "Exit code" in res[0] else ("-" if not res else "0")
    out = [winner, dbl, code, "denied" if denied else "ran"]
else:
    i = [k for k, r in enumerate(rows) if r["scripted_tool_use"]][0]
    c = [r for r in rows[i + 1:] if not r["tool_result"] and r["tail"] != "ok"]
    r = c[0] if c else None
    out = [r["model"].replace("claude-", "").replace("-5-5", "") if r else "NO-SUBAGENT", "tag-kept" if r and r["tag_left"] else "tag-stripped/none", "-", "denied" if denied else "ran"]
print("\t".join(out + [">".join(fin), dr]))
