# Governance — Downshift

## Maintainer

**Tiago Vilas Boas** ([tiagovilasboas](https://github.com/tiagovilasboas)) is the
current maintainer and copyright holder (see [LICENSE](LICENSE)).

## Decision making

- **Product direction:** maintainer, informed by [ROADMAP.md](ROADMAP.md), issues, and
  beta-exit evidence in [docs/reference/beta-exit.md](docs/reference/beta-exit.md).
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

By contributing, you grant the licence in [LICENSE](LICENSE) Section 5 (Contribution
License). You keep your copyright; the project receives a perpetual, worldwide,
non-exclusive licence to use your contribution under Apache 2.0, as stated in
[CONTRIBUTING.md](CONTRIBUTING.md).

There is no elected steering committee yet. As adoption grows, governance may
move to a lightweight maintainer team documented here.

## Security

See [SECURITY.md](SECURITY.md). Do not report vulnerabilities in public issues.
Private reporting is enabled on GitHub (Security → Advisories). The `main` branch
uses required reviews (including CODEOWNERS), CI status checks, conversation
resolution, admin enforcement, and a repository ruleset that blocks force-push
and branch deletion.

## Commercial use

See [COMMERCIAL.md](COMMERCIAL.md).
