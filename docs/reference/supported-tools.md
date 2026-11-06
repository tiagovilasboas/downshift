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
| rubocop | `rubocop` (read-only, default formatter) |
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
| deno | `deno test`, `deno check`, `deno lint` (no `run`, `install`, `task`, `compile`, `--watch`, `--fix` or remote URL) |
| zig | `zig build`, `zig test` (no `zig run`, no `run*` build step, no `--watch` or `--test-cmd`) |
| lerna | `lerna run`, `lerna exec -- <allowlisted command>`, `lerna list` |
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

## Listings and search

| Command | Description |
|---------|-------------|
| `ls` | Names only; lossy, so any danger word keeps it raw |
| `find` | Indented path tree, lossless; `-exec`, `-delete` and other acting primaries are refused |
| `grep -r`, `rg` | Matches grouped by file (`path:line:text` with `-n`, `path:text` without); up to 20 rows per file and 200 in all, then a count of what was cut; a row is read only under a path operand, so a single-file search stays raw |
| `grep -rl`, `rg -l` | A list of paths, shown as the find tree |
| `grep -rc`, `rg -c` | `path:count` rows; files with a count of 0 folded into one count |

## Reading files

| Command | Description |
|---------|-------------|
| `cat FILE`, `head`/`tail -n N FILE`, `sed -n 'A,Bp' FILE` | A source or Markdown file of 300 lines or more folds to its outline: the first 8 and last 4 lines, every declaration line (`func`, `def`, `class`, `type`, `interface`, `impl`, `export`, `const`, headings) and every line with a danger word, each with its file line number. A note gives the folded count and the exact `sed -n` range that prints the whole text; with CCR on the full text is also under `downshift retrieve`. Short files, other extensions, files with fewer than 3 declarations and secret-looking names stay raw |
| `cat FILE.log`, `tail -n N FILE.log` | A log of 40 lines or more: each run of 3 or more consecutive lines that differ only in timestamps, ids and numbers collapses to its first line, a count and the last line of the run; a run of identical lines to the line and a count. A run of merely similar lines that holds a danger word (`error`, `fail`, ...) is kept whole. A note gives the folded count and the command that prints the whole text |
| `cat FILE.json`, `jq . FILE` | A single JSON document of 4 KiB or more becomes its schema: every key, the type of each value, the length of each array and the shape of its first element (up to 40 keys per object, 6 levels). Values are never shown. A cut document (`head`, `tail`), several documents and invalid JSON stay raw |
| `wc -l FILE...` | Runs through the wrapper; the output is not changed |

## GitLab CLI

| Command | Description |
|---------|-------------|
| `glab mr list\|view\|diff`, `glab issue list\|view`, `glab ci status\|list`, `glab pipeline list` | Read-only. GitLab tokens (`glpat-`, `gldt-`, `glptt-`, `glrt-` and the other `gl*-` kinds, 20+ token characters), `PRIVATE-TOKEN` and `Bearer` values and `user:token@` in a URL are replaced by `[redacted]`; nothing else is cut, because the output formats are not documented (the corpus fixtures are synthetic). `-w`/`--web`, `--show-token`, `--live`, `ci view`, `ci trace`, `api`, `auth`, `config`, `variable`, `job` and every write verb stay unwrapped. The raw output is never offered to the CCR store |

## Fetching a URL

| Command | Description |
|---------|-------------|
| `curl [-sSiIfL] [-m N] [-X GET\|HEAD] URL`, `wget -qO- URL` | A response of 2 KiB or more keeps its status line, key headers (`content-type`, `content-length`, `content-encoding`, `location`, `cache-control`, `etag`, `last-modified`, `retry-after`, `server`) and the first 1,024 and last 512 bytes of the body, with the count of bytes cut; other headers are counted. The value of any header named like a credential (`Set-Cookie`, `Authorization`, `*token*`, `*key*`, `*session*`) is replaced by `[redacted]`. Only read-style forms are wrapped: no output file, upload, form, body, `-H`, `-u`, `-b`, config file or method but GET and HEAD. A status of 400 or more, or more than 30 danger-word lines in the cut stretch, stays raw. The raw response is never offered to the CCR store |

Why 300 lines: over this repository's 1,998 source and Markdown files (measured 2026-11-06), a 200-line cut would reach 12% of the files (35% of the bytes) and a 300-line cut 2.8% (14% of the bytes). At 300, 54 of the 56 files fold and the folded text is 71.5% smaller, with every danger-word line kept. 300 is the size the project already treats as a file too long to read whole, so the fold lands on the long tail where an outline helps and a short file is never hidden.

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
