# Supported Tools

Downshift compacts output from 70+ command families. Commands not on this list
pass through unchanged.

## Test Runners

| Tool | Commands |
|------|----------|
| Go | `go test` |
| Rust | `cargo test` |
| Python | `pytest`, `python -m pytest` |
| JavaScript | `vitest`, `jest`, `npm test`, `pnpm test`, `yarn test`, `bun test` |
| PHP | `phpunit`, `pest`, `./vendor/bin/phpunit` |
| Flutter | `flutter test` |
| Dart | `dart test` |
| Gradle | `gradle test`, `./gradlew test` |
| Maven | `mvn test`, `./mvnw test` |
| Playwright | `playwright test`, `npx playwright test` |
| Cypress | `cypress run` |

## Go Toolchain

| Command | Description |
|---------|-------------|
| `go build` | Compile packages |
| `go run` | Compile and run |
| `go install` | Compile and install |
| `go mod tidy` | Add/remove modules |
| `go mod download` | Download modules |
| `go mod verify` | Verify dependencies |
| `go generate` | Run go generate |
| `go fmt` | Format code |
| `go vet` | Report suspicious constructs |
| `go doc` | Show documentation |
| `go version` | Print version |
| `go env` | Print environment |
| `go list` | List packages |
| `go get` | Download modules |
| `go work` | Workspace commands |
| `go clean` | Remove object files |

## Go Linters

| Tool | Commands |
|------|----------|
| staticcheck | `staticcheck ./...` |
| golangci-lint | `golangci-lint run` |
| govulncheck | `govulncheck ./...` |

## Rust/Cargo

| Command | Description |
|---------|-------------|
| `cargo build` | Compile packages |
| `cargo run` | Run binary |
| `cargo check` | Check without building |
| `cargo clippy` | Run lints |
| `cargo fmt` | Format code |
| `cargo doc` | Build documentation |
| `cargo add` | Add dependencies |
| `cargo update` | Update dependencies |
| `cargo tree` | Show dependency tree |
| `cargo audit` | Audit for vulnerabilities |
| `cargo outdated` | Show outdated deps |
| `cargo clean` | Remove artifacts |
| `cargo fetch` | Fetch dependencies |
| `cargo vendor` | Vendor dependencies |
| `cargo metadata` | Output JSON metadata |

## Python

| Tool | Commands |
|------|----------|
| pip | `pip list`, `pip show`, `pip freeze`, `pip check` |
| pip3 | `pip3 list`, `pip3 show`, `pip3 freeze`, `pip3 check` |
| uv | `uv pip list`, `uv run`, `uv sync`, `uv lock`, `uv tree` |
| ruff | `ruff check`, `ruff format --check` |
| mypy | `mypy` |
| black | `black --check` |
| isort | `isort --check` |
| pyright | `pyright` |
| bandit | `bandit` |
| poetry | `poetry show`, `poetry check`, `poetry lock`, `poetry run` |
| pdm | `pdm list`, `pdm show`, `pdm lock`, `pdm run` |

## Build Systems

| Tool | Commands |
|------|----------|
| make | `make`, `gmake` |
| cmake | `cmake` |
| ninja | `ninja` |
| bazel | `bazel build`, `bazel test`, `bazel run`, `bazel query` |
| just | `just` |
| task | `task` |
| nx | `nx run`, `nx build`, `nx test`, `nx lint` |
| lerna | `lerna run`, `lerna exec`, `lerna list` |
| changesets | `changesets status`, `changesets version` |

## Frontend

| Tool | Commands |
|------|----------|
| TypeScript | `tsc` |
| Next.js | `next build` |
| Vite | `vite build`, `vite preview` |
| Turbo | `turbo run` |
| npm | `npm test`, `npm run build` |
| pnpm | `pnpm test`, `pnpm run build` |
| yarn | `yarn test`, `yarn build` |
| bun | `bun test`, `bun build` |

## PHP

| Tool | Commands |
|------|----------|
| Composer | `composer validate`, `composer show`, `composer outdated`, `composer audit` |
| Artisan | `artisan test`, `artisan route:list`, `artisan config:show` |
| Laravel Sail | `sail test`, `sail artisan` |
| PHPStan | `phpstan` |
| Psalm | `psalm` |
| PHP-CS-Fixer | `php-cs-fixer check` |
| Pint | `pint --test` |
| Rector | `rector --dry-run` |

## Linters/Formatters

| Tool | Commands |
|------|----------|
| ESLint | `eslint` |
| Prettier | `prettier --check` |
| Biome | `biome check`, `biome lint` |

## Git

All git subcommands are supported:

`git status`, `git log`, `git diff`, `git show`, `git branch`, `git fetch`,
`git pull`, `git push`, `git commit`, `git add`, `git stash`, `git remote`,
`git tag`, `git config`, `git rev-parse`, `git ls-files`

## Kubernetes/Helm

| Tool | Commands |
|------|----------|
| kubectl | `kubectl get`, `kubectl describe`, `kubectl logs` |
| helm | `helm list`, `helm status` |

## Terraform

| Command | Description |
|---------|-------------|
| `terraform plan` | Show execution plan |
| `terraform fmt -check` | Check formatting |

## Cloud CLI

| Tool | Commands |
|------|----------|
| AWS CLI | `aws describe`, `aws list`, `aws get`, `aws s3 ls`, `aws s3 cp`, `aws logs` |
| Azure CLI | `az list`, `az show`, `az pipelines` |

## Docker

| Command | Description |
|---------|-------------|
| `docker ps` | List containers |
| `docker logs` | View logs |
| `docker images` | List images |
| `docker build` | Build image |

## GitOps

| Tool | Commands |
|------|----------|
| ArgoCD | `argocd list`, `argocd get`, `argocd diff` |
| Kustomize | `kustomize build` |

## Observability

| Tool | Commands |
|------|----------|
| Loki | `logcli` |
| Sentry | `sentry-cli` |
| Grafana | `grafana-cli` |
| Datadog | `datadog-ci` |

## Database

| Tool | Commands |
|------|----------|
| MySQL | Read queries only |
| Redis | Read commands only |

## Version Managers

| Tool | Commands |
|------|----------|
| nvm | `nvm list`, `nvm use`, `nvm install` |
| rbenv | `rbenv versions`, `rbenv version` |
| pyenv | `pyenv versions`, `pyenv version` |
| asdf | `asdf list`, `asdf current` |
| mise | `mise list`, `mise current`, `mise use` |
| fnm | `fnm list`, `fnm use` |
| volta | `volta list` |
| proto | `proto list`, `proto run` |
| direnv | `direnv status`, `direnv allow` |

## GitHub CLI

All `gh` subcommands are supported:

`gh pr`, `gh run`, `gh api`, `gh issue`, `gh repo`, `gh auth`, `gh search`,
`gh workflow`, `gh release`, `gh gist`, `gh browse`, `gh status`

---

## Missing a Tool?

Run `downshift learning suggestions` to see which uncompacted commands you use
most frequently. Then [contribute a compressor](../contributing/compressors.md)
or [open an issue](https://github.com/tiagovilasboas/downshift/issues).
