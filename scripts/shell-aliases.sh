#!/bin/bash
# Downshift shell aliases — transparent compaction for agent environments
# Source this file in your shell rc (~/.bashrc, ~/.zshrc) or agent's shell init.
#
# Usage:
#   source /path/to/shell-aliases.sh
#   # or
#   eval "$(downshift enable)"
#
# These aliases intercept allowlisted commands and route them through the
# compactor automatically, so agents get compressed output without remembering
# to wrap each command.

# Only activate if downshift is available
if ! command -v downshift &>/dev/null; then
  return 0 2>/dev/null || exit 0
fi

# Kill switch: set DOWNSHIFT_SHELL_ALIASES=0 to disable
if [[ "${DOWNSHIFT_SHELL_ALIASES:-1}" == "0" ]]; then
  return 0 2>/dev/null || exit 0
fi

# Helper that decides whether to compact or passthrough
__downshift_wrap() {
  local cmd="$1"
  shift
  
  # Skip if already inside a compactor call (prevent double-wrap)
  if [[ -n "${DOWNSHIFT_COMPACTOR_ACTIVE:-}" ]]; then
    command "$cmd" "$@"
    return $?
  fi
  
  # Skip if in an interactive terminal session (user is typing directly)
  # This preserves full output for human debugging while compacting for agents
  if [[ -t 1 ]] && [[ -z "${DOWNSHIFT_FORCE_COMPACT:-}" ]]; then
    command "$cmd" "$@"
    return $?
  fi
  
  export DOWNSHIFT_COMPACTOR_ACTIVE=1
  downshift compactor -- "$cmd" "$@"
  local rc=$?
  unset DOWNSHIFT_COMPACTOR_ACTIVE
  return $rc
}

# === Test runners ===
go()      { __downshift_wrap go "$@"; }
cargo()   { __downshift_wrap cargo "$@"; }
pytest()  { __downshift_wrap pytest "$@"; }
vitest()  { __downshift_wrap vitest "$@"; }
jest()    { __downshift_wrap jest "$@"; }
phpunit() { __downshift_wrap phpunit "$@"; }
pest()    { __downshift_wrap pest "$@"; }

# === Build tools ===
make()    { __downshift_wrap make "$@"; }
npm()     { __downshift_wrap npm "$@"; }
pnpm()    { __downshift_wrap pnpm "$@"; }
yarn()    { __downshift_wrap yarn "$@"; }
bun()     { __downshift_wrap bun "$@"; }
tsc()     { __downshift_wrap tsc "$@"; }

# === Git ===
git()     { __downshift_wrap git "$@"; }

# === Linters ===
eslint()     { __downshift_wrap eslint "$@"; }
prettier()   { __downshift_wrap prettier "$@"; }
staticcheck(){ __downshift_wrap staticcheck "$@"; }
golangci-lint() { __downshift_wrap golangci-lint "$@"; }

# === Cloud/Infra ===
kubectl() { __downshift_wrap kubectl "$@"; }
docker()  { __downshift_wrap docker "$@"; }
terraform() { __downshift_wrap terraform "$@"; }
aws()     { __downshift_wrap aws "$@"; }

# === PHP ===
composer() { __downshift_wrap composer "$@"; }
artisan()  { __downshift_wrap artisan "$@"; }
php()      { __downshift_wrap php "$@"; }

# === Python ===
pip()     { __downshift_wrap pip "$@"; }
pip3()    { __downshift_wrap pip3 "$@"; }
uv()      { __downshift_wrap uv "$@"; }
ruff()    { __downshift_wrap ruff "$@"; }
mypy()    { __downshift_wrap mypy "$@"; }

# === GitHub CLI ===
gh()      { __downshift_wrap gh "$@"; }

# Export functions for subshells
export -f __downshift_wrap go cargo pytest vitest jest phpunit pest 2>/dev/null
export -f make npm pnpm yarn bun tsc git 2>/dev/null
export -f eslint prettier staticcheck golangci-lint 2>/dev/null
export -f kubectl docker terraform aws 2>/dev/null
export -f composer artisan php pip pip3 uv ruff mypy gh 2>/dev/null
