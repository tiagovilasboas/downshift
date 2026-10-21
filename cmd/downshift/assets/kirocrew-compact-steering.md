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

## Allowlisted families

### Test runners
go test · pytest · vitest · jest · cargo test · playwright test · cypress ·
phpunit · pest · flutter test · dart test · gradle test · maven test

### GitHub CLI (all subcommands)
gh pr · gh run · gh api · gh issue · gh repo · gh auth · gh search · gh workflow ·
gh release list/view · gh gist list/view · gh browse · gh status

### Go toolchain
go build · go run · go install · go mod tidy/download/verify/graph/why ·
go generate · go fmt · go vet · go doc · go version · go env · go list ·
go get · go work · go clean · go tool

### Go linters
staticcheck · golangci-lint run · govulncheck ·
go run honnef.co/go/tools/cmd/staticcheck@latest

### Rust/Cargo toolchain
cargo build · cargo run · cargo check · cargo clippy · cargo fmt --check ·
cargo doc · cargo add · cargo update · cargo tree · cargo audit · cargo outdated ·
cargo clean · cargo fetch · cargo vendor · cargo metadata

### Python tooling
pip list/show/freeze/check · pip3 list/show/freeze/check ·
uv pip list/show/freeze/check/compile/sync · uv run/sync/lock/tree/version ·
ruff check · ruff format --check · mypy · black --check · isort --check ·
pyright · bandit · poetry show/check/lock/export/run · pdm list/show/lock/run

### Build systems
make · gmake · cmake · ninja · bazel build/test/run/query · just · task ·
nx run/build/test/lint · lerna run/exec/list · changesets status/version

### Frontend build/test
tsc · next build · vite build/preview · turbo run ·
npm test/build · pnpm test/build · yarn test/build · bun test/build

### PHP ecosystem
composer validate/show/outdated/audit · artisan test/route:list/config:show ·
sail test/artisan · phpstan · psalm · php-cs-fixer check · pint --test ·
pest · rector --dry-run

### Linters/formatters
eslint · prettier --check · biome check/lint

### Git (all subcommands)
git status · git log · git diff · git show · git branch · git fetch · git pull ·
git push · git commit · git add · git stash · git remote · git tag ·
git config --get/--list · git rev-parse · git ls-files

### Kubernetes/Helm
kubectl get/describe/logs · helm list/status

### Terraform
terraform plan · terraform fmt -check

### Cloud CLI
aws describe/list/get/s3 ls/s3 cp/logs · az list/show/pipelines

### Docker
docker ps · docker logs · docker images · docker build

### GitOps
argocd list/get/diff · kustomize build

### Observability
logcli · sentry-cli · grafana-cli · datadog-ci

### Database
mysql (read queries) · redis (read commands)

### Version managers
nvm list/use/install · rbenv versions/version · pyenv versions/version ·
asdf list/current · rtx/mise list/current/use · fnm list/use · volta list ·
proto list/run · direnv status/allow

## Examples

```bash
downshift compactor -- go test ./...
downshift compactor -- go build -v ./cmd/myapp
downshift compactor -- go run main.go
downshift compactor -- go mod tidy
downshift compactor -- staticcheck ./...
downshift compactor -- go run honnef.co/go/tools/cmd/staticcheck@latest ./...
downshift compactor -- golangci-lint run
downshift compactor -- gh pr list
downshift compactor -- gh run view 12345 --log
downshift compactor -- gh api repos/owner/repo/pulls
downshift compactor -- pnpm test
downshift compactor -- cargo build --release
downshift compactor -- cargo clippy
downshift compactor -- make build
downshift compactor -- kubectl get pods -n production
downshift compactor -- aws s3 ls s3://my-bucket
downshift compactor -- git status
downshift compactor -- mise list
```

## Do not

- Do not pipe a non-allowlisted command through compactor — it is refused.
- Do not use compactor to hide a FAILURE: it keeps the failure summary and exit
  code, so a red run stays red.
- Do not assume the preToolUse hook did this for you — it cannot. You invoke it.

`compactor` is the product name and the official verb; `compact` and `exec` are kept aliases.
