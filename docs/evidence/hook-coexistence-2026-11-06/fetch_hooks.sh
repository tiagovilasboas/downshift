#!/usr/bin/env bash
# fetch_hooks.sh <dest-dir>: download the two third-party hook files used by the
# experiment and verify the SHA-256 of the exact bytes that were read and run on
# 2026-11-06. The files are not copied into this repository. Upstream commit ids
# could not be read from the evidence host, so the hash is the pin.
set -euo pipefail
dest=${1:?usage: fetch_hooks.sh <dest-dir>}; mkdir -p "$dest"
get() { # url file sha256
  curl -fsS -m 60 -o "$dest/$2" "$1"; chmod +x "$dest/$2"
  echo "$3  $dest/$2" | sha256sum -c -
}
get https://raw.githubusercontent.com/The-Guard305/cheap-subagents/main/cheap_subagents.py cheap_subagents.py 9403ce224eb7057cb7f7b7c41eaa59ee0e2f468a143c388a6e86bba14f30f35a
get https://raw.githubusercontent.com/rtk-ai/rtk/master/hooks/claude/rtk-rewrite.sh rtk-rewrite-claude.sh d4cd1e04cbdacd52d24eefb1e1500ed76008b9710ed4e9a1de0bf36b1893267d
