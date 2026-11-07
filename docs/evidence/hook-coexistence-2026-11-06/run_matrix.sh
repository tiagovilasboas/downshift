#!/usr/bin/env bash
# run_matrix.sh  -- Env: FX_ROOT, DOWNSHIFT_BIN, (CLAUDE_BIN). Writes $FX_ROOT/results.tsv.
# Needs the third-party hooks first:  fetch_hooks.sh "$FX_ROOT/hooks"
set -u
FX_DIR=$(cd "$(dirname "$0")" && pwd); : "${FX_ROOT:?}" "${DOWNSHIFT_BIN:?}"
RES=$FX_ROOT/results.tsv
printf 'scenario\tkind\torder\tdelay\tno_allow\twinner_or_model\tdouble_wrapped_or_tag\texit_code\tpermission\thook_finish_order\tdoctor_overlap_rows\n' > "$RES"
BASH_T='{"name":"Bash","input":{"command":"go test ./...","description":"run tests"}}'
ag() { python3 -I -c "
import json,sys
print(json.dumps({'name':'Agent','input':{'subagent_type':'general-purpose','description':'d','prompt':sys.argv[1],'model':'opus'}}))" "$1"; }
run() { # label kind order tool [DELAY] [NO_ALLOW]
  local label=$1 kind=$2 order=$3 tool=$4 delay=${5:-} noallow=${6:-}
  DELAY=$delay NO_ALLOW=$noallow "$FX_DIR/run_scenario.sh" "$label" "$order" "$tool"
  printf '%s\t%s\t%s\t%s\t%s\t%s\n' "$label" "$kind" "$order" "${delay:--}" "${noallow:+yes}" "$(python3 -I "$FX_DIR/summarize.py" "$FX_ROOT/runs/$label" "$kind")" | tee -a "$RES"
}
# --- Downshift + RTK on Bash
for o in downshift rtk downshift,rtk rtk,downshift; do run "bash-${o//,/_}" bash "$o" "$BASH_T"; done
for i in 1 2 3 4 5; do run "bash-natural-ds_rtk-$i" bash downshift,rtk "$BASH_T"; run "bash-natural-rtk_ds-$i" bash rtk,downshift "$BASH_T"; done
for o in downshift,rtk rtk,downshift; do
  run "bash-${o//,/_}-rtk-slow" bash "$o" "$BASH_T" rtk=0.5
  run "bash-${o//,/_}-downshift-slow" bash "$o" "$BASH_T" downshift=0.5
done
for o in downshift rtk downshift,rtk rtk,downshift; do run "bash-noallow-${o//,/_}" bash "$o" "$BASH_T" "" 1; done
# --- Downshift + cheap-subagents on Agent
P1="implement pagination for the users endpoint"
P2="[model:sonnet] rename the userId variable to userIdentifier"
P3="rename the lock helper in the concurrency module"
n=0; for P in "$P1" "$P2" "$P3"; do n=$((n+1)); T=$(ag "$P")
  for o in downshift cheap downshift,cheap cheap,downshift; do run "agent$n-${o//,/_}" agent "$o" "$T"; done
  if [ $n -ge 2 ]; then
    run "agent$n-downshift_cheap-cheap-slow" agent downshift,cheap "$T" cheap=0.5
    run "agent$n-downshift_cheap-downshift-slow" agent downshift,cheap "$T" downshift=0.5
  fi
done
echo "wrote $RES"
