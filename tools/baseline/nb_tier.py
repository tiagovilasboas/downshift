#!/usr/bin/env python3
"""Offline learned baseline for the router: multinomial naive Bayes over
unigrams+bigrams, trained on ONE labelled file and evaluated read-only on others.

Settings are fixed up front (lowercase word tokens, unigrams+bigrams,
Laplace alpha=1, uniform class prior) and are not tuned on any evaluation file.
Pure stdlib so it runs anywhere.

    python3 tools/baseline/nb_tier.py --train benchmark/tasks.json \
        --eval benchmark/holdout.json other.json ...

Each eval file is a JSON array of {"prompt", "label"} (labels TRIVIAL/SIMPLE/
MEDIUM/COMPLEX or small/mid/frontier). With --regex (JSON arrays of production
predictions) the script also reports the upshift-only combination: NB's tier
when it is higher than production's and the per-feature log-likelihood margin
is >= --margin. --tune picks that margin on the given files with a fixed rule:
the smallest grid value whose tier accuracy is at most --max-drop below
production.
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


def tier_of(label):
    """Tier index for either label scheme (TRIVIAL..COMPLEX or small/mid/frontier)."""
    return TIER[label] if label in TIER else ["small", "mid", "frontier"].index(label)


def margin(model, vocab, text, hi, lo):
    """Per-feature log-likelihood gap between the best label of tier `hi` and
    the best label of tier `lo`; 0 when there is no known feature."""
    fs = [f for f in feats(text) if f in vocab]
    if not fs:
        return 0.0
    ll = {l: sum(model[l][0].get(f, model[l][1]) for f in fs) for l in LABELS}
    best = lambda t: max(v for l, v in ll.items() if TIER[l] == t)
    return (best(hi) - best(lo)) / len(fs)


def combine(model, vocab, prompts, rx, nb, t):
    """Upshift-only: take NB's tier only when it is higher and the margin is >= t."""
    out = []
    for p, r, n in zip(prompts, rx, nb):
        r, n = tier_of(r), tier_of(n)
        out.append(n if n > r and margin(model, vocab, p, n, r) >= t else r)
    return out


def wilson(k, n, z=1.96):
    if n == 0:
        return (0.0, 0.0)
    p = k / n
    d = 1 + z * z / n
    c = (p + z * z / (2 * n)) / d
    h = z * math.sqrt(p * (1 - p) / n + z * z / (4 * n * n)) / d
    return (round(c - h, 3), round(c + h, 3))


def metrics(gold, pred):
    g = [tier_of(x) for x in gold]
    q = [tier_of(x) for x in pred]
    n = len(g)
    k = sum(a == b for a, b in zip(g, q))
    conf = [[sum(a == i and b == j for a, b in zip(g, q)) for j in range(3)] for i in range(3)]
    return {
        "n": n,
        "tier_acc": round(k / n, 3),
        "tier_acc_ci95": wilson(k, n),
        "frontier_to_lower": f"{conf[2][0] + conf[2][1]}/{sum(conf[2])}",
        "frontier_to_mid": f"{conf[2][1]}/{sum(conf[2])}",
        "frontier_to_small": f"{conf[2][0]}/{sum(conf[2])}",
        "small_to_higher": f"{conf[0][1] + conf[0][2]}/{sum(conf[0])}",
        "small_to_mid": f"{conf[0][1]}/{sum(conf[0])}",
        "small_to_frontier": f"{conf[0][2]}/{sum(conf[0])}",
        "confusion_gold_rows_small_mid_frontier": conf,
    }


GRID = [round(i * 0.02, 2) for i in range(51)]


def load_preds(paths):
    """Production predictions: JSON arrays of {"prompt", "label"}."""
    preds = {}
    for path in paths:
        for r in json.load(open(path)):
            preds[r["prompt"]] = r["label"]
    return preds


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--train", required=True)
    ap.add_argument("--eval", nargs="*", default=[])
    ap.add_argument("--regex", nargs="*", default=[], help="JSON arrays of production predictions {prompt, label}")
    ap.add_argument("--tune", nargs="*", default=[], help="files to choose the upshift margin on (pre-registered rule)")
    ap.add_argument("--max-drop", type=float, default=0.02, help="max tier-accuracy drop vs production when tuning")
    ap.add_argument("--margin", type=float, default=0.0, help="upshift margin for --eval (inf = never upshift)")
    a = ap.parse_args()
    model, vocab = train(json.load(open(a.train)))
    regex = load_preds(a.regex)

    if a.tune:
        rows = [r for path in a.tune for r in json.load(open(path))]
        prompts = [r["prompt"] for r in rows]
        gold = [tier_of(r["label"]) for r in rows]
        rx = [regex[p] for p in prompts]
        nb = [predict(model, vocab, p) for p in prompts]
        base = sum(g == tier_of(x) for g, x in zip(gold, rx)) / len(gold)
        chosen = float("inf")
        for t in GRID:
            up = combine(model, vocab, prompts, rx, nb, t)
            acc = sum(g == u for g, u in zip(gold, up)) / len(gold)
            ups = sum(u != tier_of(x) for u, x in zip(up, rx))
            print(json.dumps({"margin": t, "tier_acc": round(acc, 4), "production": round(base, 4), "upshifts": ups}))
            if chosen == float("inf") and base - acc <= a.max_drop + 1e-9:
                chosen = t
        print(json.dumps({"chosen_margin": chosen, "rule": f"smallest margin with drop <= {a.max_drop}"}))
        return 0

    for path in a.eval:
        rows = json.load(open(path))
        prompts = [r["prompt"] for r in rows]
        gold = [r["label"] for r in rows]
        nb = [predict(model, vocab, p) for p in prompts]
        out = {"file": path, "nb": metrics(gold, nb)}
        if regex and all(p in regex for p in prompts):
            rx = [regex[p] for p in prompts]
            out["regex"] = metrics(gold, rx)
            up = combine(model, vocab, prompts, rx, nb, a.margin)
            out["regex_upshift_by_nb"] = metrics(gold, [["small", "mid", "frontier"][u] for u in up])
            out["margin"] = a.margin
        print(json.dumps(out))


if __name__ == "__main__":
    sys.exit(main())
