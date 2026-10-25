#!/usr/bin/env bash
# One-shot: gh auth (if needed), open PR, wait for CI, squash merge.
set -euo pipefail
REPO="tiagovilasboas/downshift"
HEAD="integrate/local-main-2026-10-25"
BASE="main"

if ! gh auth status -h github.com >/dev/null 2>&1; then
  echo "Complete GitHub CLI login in this Terminal window (device code)."
  gh auth login -h github.com -p ssh -w
fi
gh auth switch -u tiagovilasboas 2>/dev/null || true
echo "Logged in as: $(gh api user -q .login)"

PR="$(gh pr list --repo "$REPO" --head "$HEAD" --json number -q '.[0].number' 2>/dev/null || true)"
if [[ -z "$PR" || "$PR" == "null" ]]; then
  gh pr create --repo "$REPO" --head "$HEAD" --base "$BASE" \
    --title "Integrate local main: docs, gh run compactor, progress band" \
    --body "Integrates local merges (roadmaps, gh run compactor, progress band, Cursor docs, CHANGELOG). Includes test(models) state dir isolation."
  PR="$(gh pr list --repo "$REPO" --head "$HEAD" --json number -q '.[0].number')"
fi
echo "PR #${PR}: https://github.com/${REPO}/pull/${PR}"

GATE="${HOME}/Github/harness-core/checks/merge-when-green.sh"
if [[ -x "$GATE" ]]; then
  "$GATE" "$REPO" "$PR"
else
  gh pr checks "$PR" --repo "$REPO" --watch
  gh pr merge "$PR" --repo "$REPO" --squash --delete-branch
fi

git -C "${HOME}/Github/downshift" fetch origin
git -C "${HOME}/Github/downshift" checkout main
git -C "${HOME}/Github/downshift" reset --hard "origin/${BASE}"
echo "Done. local main = origin/main"
