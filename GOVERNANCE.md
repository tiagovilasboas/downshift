# Governance — Downshift

## Maintainer

**Tiago de Carvalho Vilas Boas** ([tiagovilasboas](https://github.com/tiagovilasboas)) is the
current maintainer and copyright holder (see [LICENSE](LICENSE)).

## Decision making

- **Product direction:** maintainer, informed by [ROADMAP.md](ROADMAP.md), issues, and
  beta-exit evidence in [docs/BETA-EXIT.md](docs/BETA-EXIT.md).
- **Routing policy changes** that affect cost or safety (signals, safety floor,
  default tiers): require benchmark gates in CI and, when possible, a misroute
  issue or design note in `docs/design/`.
- **Breaking changes:** documented in [CHANGELOG.md](CHANGELOG.md); hook JSON
  contracts and event schema treated as stable during beta unless versioned.

## Releases

- **Tags:** `v*` semver; pre-releases may use `-beta`, `-rc` suffixes.
- **Artifacts:** GitHub Releases via GoReleaser (`.goreleaser.yml`); `install.sh`
  tracks latest tag.
- **Licence:** Apache 2.0 per [LICENSE](LICENSE) and [NOTICE](NOTICE).

## Contributions

See [CONTRIBUTING.md](CONTRIBUTING.md) and [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).

By contributing, you agree to the copyright assignment in `LICENSE` Section 8.

There is no elected steering committee yet. As adoption grows, governance may
move to a lightweight maintainer team documented here.

## Security

See [SECURITY.md](SECURITY.md). Do not report vulnerabilities in public issues.

## Commercial use

See [COMMERCIAL.md](COMMERCIAL.md).
