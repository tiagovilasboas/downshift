#!/usr/bin/env python3
# Copyright (c) 2026 Tiago Vilas Boas.
# SPDX-License-Identifier: Apache-2.0
"""Checks the Downshift themes: allowed tokens, color syntax, WCAG contrast."""
import json
import re
import sys
from pathlib import Path

# Token list from code.claude.com/docs/en/terminal-config#create-a-custom-theme
ALLOWED = set("""claude text inverseText inactive subtle suggestion permission remember
success error warning merged promptBorder planMode autoAccept bashBorder ide fastMode
effortUltra diffAdded diffRemoved diffAddedDimmed diffRemovedDimmed diffAddedWord
diffRemovedWord userMessageBackground userMessageBackgroundHover bashMessageBackgroundColor
memoryBackgroundColor selectionBg composerSidebarBackground rate_limit_fill rate_limit_empty
briefLabelYou briefLabelClaude claudeShimmer warningShimmer permissionShimmer
promptBorderShimmer inactiveShimmer fastModeShimmer""".split())
for c in "red blue green yellow purple orange pink cyan".split():
    ALLOWED.add(f"{c}_FOR_SUBAGENTS_ONLY")
for c in "red orange yellow green blue indigo violet".split():
    ALLOWED.update({f"rainbow_{c}", f"rainbow_{c}_shimmer"})
BASES = {"dark", "light", "dark-daltonized", "light-daltonized", "dark-ansi", "light-ansi"}
ANSI = r"(black|red|green|yellow|blue|magenta|cyan|white)|(blackBright|redBright|greenBright|yellowBright|blueBright|magentaBright|cyanBright|whiteBright)|gray|grey"
COLOR = re.compile(
    r"^(#[0-9a-fA-F]{6}|#[0-9a-fA-F]{3}|rgb\(\s*\d{1,3}\s*,\s*\d{1,3}\s*,\s*\d{1,3}\s*\)"
    r"|ansi256\(\d{1,3}\)|ansi:(" + ANSI + r"))$")
HERE = Path(__file__).resolve().parent.parent / "themes"
# file -> (page background, expected base)
THEMES = {"downshift.json": ("#0A0A0A", "dark"), "downshift-light.json": ("#FFFFFF", "light")}
TEXT_KEYS = ["text", "subtle", "error", "success", "warning"]


def rgb(h):
    h = h.lstrip("#")
    if len(h) == 3:
        h = "".join(c * 2 for c in h)
    return tuple(int(h[i:i + 2], 16) for i in (0, 2, 4))


def lum(h):
    f = lambda v: v / 12.92 if v / 255 <= 0.03928 else None
    out = []
    for v in rgb(h):
        s = v / 255
        out.append(s / 12.92 if s <= 0.03928 else ((s + 0.055) / 1.055) ** 2.4)
    return 0.2126 * out[0] + 0.7152 * out[1] + 0.0722 * out[2]


def ratio(a, b):
    la, lb = sorted((lum(a), lum(b)), reverse=True)
    return (la + 0.05) / (lb + 0.05)


def main():
    errs = []
    for fname, (bg, base) in THEMES.items():
        t = json.loads((HERE / fname).read_text())
        o = t.get("overrides", {})
        if t.get("base") != base:
            errs.append(f"{fname}: base should be {base}")
        if not t.get("name"):
            errs.append(f"{fname}: missing name")
        for k, v in o.items():
            if k not in ALLOWED:
                errs.append(f"{fname}: unknown token {k}")
            if not isinstance(v, str) or not COLOR.match(v):
                errs.append(f"{fname}: invalid color {k}={v}")
        for k in TEXT_KEYS:
            r = ratio(o[k], bg)
            print(f"{fname}: {k} {o[k]} on {bg} = {r:.2f}")
            if r < 4.5:
                errs.append(f"{fname}: {k} contrast {r:.2f} < 4.5")
        r = ratio(o["inactive"], bg)
        print(f"{fname}: inactive {o['inactive']} on {bg} = {r:.2f}")
        if r < 3.0:
            errs.append(f"{fname}: inactive contrast {r:.2f} < 3.0")
        # Diff tokens are backgrounds: the text drawn on them must stay readable.
        for k in [k for k in o if k.startswith("diff")]:
            r = ratio(o["text"], o[k])
            print(f"{fname}: text on {k} {o[k]} = {r:.2f}")
            if r < 4.5:
                errs.append(f"{fname}: text on {k} contrast {r:.2f} < 4.5")
    for e in errs:
        print("FAIL", e)
    print("OK" if not errs else f"{len(errs)} failure(s)")
    return 1 if errs else 0


if __name__ == "__main__":
    sys.exit(main())
