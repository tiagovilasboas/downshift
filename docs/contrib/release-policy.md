<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Release policy

Decision of the Author, 2026-11-07. Four versions (0.10.0 to 0.11.2) went out within twelve hours, each cut by a different agent session. To an outside reader that reads as instability, not speed. These rules keep the cadence steady while the project is in beta.

## Who cuts a release

- **Only the Author, or an agent the Author has told in that session to cut a named version.** An agent session never creates a tag, a GitHub release or a `chore(release)` commit on its own initiative, and never because a change "looks releasable".
- Finishing a fix is not a reason to release it. Merge it to `main` under `[Unreleased]` in the [CHANGELOG](../../CHANGELOG.md) and stop.

## When a patch is warranted

A patch release (`0.x.y`) needs one of:

- a bug reported by a user, or reproduced on a supported harness;
- a security fix;
- a broken install or release pipeline.

Documentation, polish, doctor wording, refactors and new tests wait for the next release that has a reason of its own.

## Cadence

- At most **one release per day**, and in practice one per week.
- A minor bump (`0.x.0`) is for a new user-visible capability; the Author chooses it.
- A tag is never moved or deleted once the Go proxy may have cached it. A bad tag is fixed forward with a new version.

## Checklist for the release the Author asked for

1. `[Unreleased]` moved into the new `## [x.y.z] - date` section, with no heading left out.
2. `docs/releases/vx.y.z.md` written, and the README, `install.sh` and install guide point at the new version.
3. PR merged through `checks/merge-when-green.sh`, never `--auto`.
4. Annotated tag on the merge commit; the release workflow must finish `release`, `verify-release` (Linux, macOS, Windows) and `publish-release`.
5. The published release is not a draft, has all assets, and its checksum and cosign bundle verify.
