#!/usr/bin/env python3
"""Offline learned baseline for the router: multinomial naive Bayes over
unigrams+bigrams, trained on ONE labelled file and evaluated read-only on others.

Settings are fixed up front (lowercase word tokens, unigrams+bigrams,
Laplace alpha=1, uniform class prior) and are not tuned on any evaluation file.
Pure stdlib so it runs anywhere.

    python3 tools/baseline/nb_tier.py --train benchmark/tasks.json \
        --eval benchmark/holdout.json other.json ...

Each eval file is a JSON array of {"prompt", "label"}; when a file of
regex predictions is given with --regex (TSV: prompt<TAB>LABEL), the script
also reports the upshift-only combination max(regex tier, NB tier).
"""
import argparse, json, math, re, sys
from collections import Counter

LABELS = ["TRIVIAL", "SIMPLE", "MEDIUM", "COMPLEX"]
TIER = {"TRIVIAL": 0, "SIMPLE": 0, "MEDIUM": 1, "COMPLEX": 2}


def feats(text):
    w = re.findall(r"[a-z0-9]+", text.lower())
    return w + [a + "_" + b for a, b in zip(w, w[1:])]


def train(rows, alpha=1.0):
    counts = {l: Counter() for l in LABELS}
    for r in rows:
        counts[r["label"]].update(feats(r["prompt"]))
    vocab = set().union(*counts.values())
    model = {}
    for l in LABELS:
        tot = sum(counts[l].values()) + alpha * len(vocab)
        model[l] = ({f: math.log((c + alpha) / tot) for f, c in counts[l].items()}, math.log(alpha / tot))
    return model, vocab


def predict(model, vocab, text):
    fs = [f for f in feats(text) if f in vocab]
    best = max(LABELS, key=lambda l: sum(model[l][0].get(f, model[l][1]) for f in fs))
    return best if fs else "MEDIUM"  # no known feature: same safe default as the regex router


def metrics(gold, pred):
    n = len(gold)
    tier = sum(TIER[g] == TIER[p] for g, p in zip(gold, pred)) / n
    fr = [p for g, p in zip(gold, pred) if g == "COMPLEX"]
    sm = [p for g, p in zip(gold, pred) if TIER[g] == 0]
    return {
        "n": n,
        "tier_acc": round(tier, 3),
        "frontier_to_mid": f"{sum(TIER[p]==1 for p in fr)}/{len(fr)}",
        "frontier_to_small": f"{sum(TIER[p]==0 for p in fr)}/{len(fr)}",
        "small_to_mid": f"{sum(TIER[p]==1 for p in sm)}/{len(sm)}",
        "small_to_frontier": f"{sum(TIER[p]==2 for p in sm)}/{len(sm)}",
    }


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--train", required=True)
    ap.add_argument("--eval", nargs="+", required=True)
    ap.add_argument("--regex", nargs="*", default=[], help="TSV prompt<TAB>LABEL of regex predictions, one per eval file")
    a = ap.parse_args()
    model, vocab = train(json.load(open(a.train)))
    regex = {}
    for path in a.regex:
        for line in open(path):
            p, l = line.rstrip("\n").split("\t")
            regex[p] = l
    for path in a.eval:
        rows = json.load(open(path))
        gold = [r["label"] for r in rows]
        nb = [predict(model, vocab, r["prompt"]) for r in rows]
        out = {"file": path, "nb": metrics(gold, nb)}
        if regex and all(r["prompt"] in regex for r in rows):
            rx = [regex[r["prompt"]] for r in rows]
            up = [max(x, y, key=lambda l: TIER[l]) for x, y in zip(rx, nb)]
            out["regex"] = metrics(gold, rx)
            out["regex_upshift_by_nb"] = metrics(gold, up)
        print(json.dumps(out))


if __name__ == "__main__":
    sys.exit(main())
