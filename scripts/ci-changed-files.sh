#!/usr/bin/env bash
# Lists changed files for the current GitHub Actions event (stdout, one path per line).
set -euo pipefail

event="${GITHUB_EVENT_NAME:-local}"

case "$event" in
  pull_request)
    base="${GITHUB_BASE_SHA:-}"
    head="${GITHUB_HEAD_SHA:-${GITHUB_SHA:-HEAD}}"
    if [ -z "$base" ]; then
      echo "ci-changed-files: GITHUB_BASE_SHA required for pull_request" >&2
      exit 1
    fi
    git diff --name-only "$base" "$head"
    ;;
  push)
    before="${GITHUB_EVENT_BEFORE:-}"
    head="${GITHUB_SHA:-HEAD}"
    if [ -n "$before" ] && [ "$before" != "0000000000000000000000000000000000000000" ]; then
      git diff --name-only "$before" "$head"
    elif git rev-parse --verify HEAD~1 >/dev/null 2>&1; then
      git diff --name-only HEAD~1 HEAD
    else
      git diff --name-only --root HEAD
    fi
    ;;
  workflow_dispatch | schedule)
    # Full suite: treat as all paths touched.
    git ls-files
    ;;
  *)
    git diff --name-only HEAD~1 HEAD 2>/dev/null || git ls-files
    ;;
esac
