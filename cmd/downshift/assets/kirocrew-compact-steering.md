---
inclusion: always
---

# Downshift compactor — always wrap allowlisted commands

Downshift by Tiago Vilas Boas
https://github.com/tiagovilasboas/downshift

KiroCrew's preToolUse hook cannot rewrite a shell command in place (its contract
is allow/block only, no `updated_input`), so compaction is NOT automatic here.
To get it, the agent invokes the command through `downshift compactor` itself.

## Rule

When you run an allowlisted command, ALWAYS run it as:

```
downshift compactor -- <command> [args...]
```

`downshift` resolves from PATH. This cuts clean, verbose output to its
essentials (PASS/FAIL + summary) and preserves the real exit code, so tokens go
to signal, not scrollback.

## Allowlisted families (everything else is refused — run it normally)

**Test runners:**
go test · pytest · vitest · jest · cargo test · playwright test · cypress ·
phpunit · pest · flutter test · gradle test · maven test

**Build tools:**
tsc · next build · vite build · turbo run · gradle build · maven package ·
docker build

**Package manager wrappers:**
npm test/build · pnpm test/build · yarn test/build · bun test/build

**PHP ecosystem:**
composer · artisan · sail test/artisan · phpstan · psalm · php-cs-fixer · pint · rector

**Linters:**
eslint · prettier · biome

**Git (all subcommands):**
git status · git log · git diff · git show · git branch · git fetch · git pull ·
git push · git commit · git add · git stash · git remote · git tag · git config

**Kubernetes/Helm:**
kubectl get/describe/logs · helm list/status

**Terraform:**
terraform plan/fmt

**Cloud CLI:**
aws describe/list/get/s3 ls/s3 cp/logs · az list/show/pipelines

**Docker:**
docker ps/logs/images/build

**GitOps:**
argocd list/get/diff · kustomize build

**Observability:**
logcli · sentry-cli · grafana-cli · datadog-ci

**Database:**
mysql (read queries) · redis (read commands)

## Examples

```
downshift compactor -- go test ./...
downshift compactor -- pnpm test
downshift compactor -- vendor/bin/phpunit --no-coverage
downshift compactor -- kubectl get pods -n production
downshift compactor -- aws s3 ls s3://my-bucket
downshift compactor -- git status
```

## Do not

- Do not pipe a non-allowlisted command through compactor — it is refused.
- Do not use compactor to hide a FAILURE: it keeps the failure summary and exit
  code, so a red run stays red.
- Do not assume the preToolUse hook did this for you — it cannot. You invoke it.

`compactor` is the product name and the official verb; `compact` and `exec` are kept aliases.
