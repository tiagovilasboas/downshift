<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# ST and UI: Site CSP and CI, and pane visual checks

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Part of the [roadmap to 9](README.md). The ST tasks land in `downshift-site`;
the UI tasks land in this repo's `downshift-agents` mod.

## Site problem

- **CSP is too permissive.** `python3 -I scripts/csp-hashes.py --check` fails:
  - `script-src` pins 575 hashes, and 1 is in use;
  - `style-src` pins 5, and 4 are in use.

  No hash is missing, so nothing is broken today, but 574 stale script hashes
  widen what the browser would run.
- **Site CI checks almost nothing.** `.github/workflows/ci.yml` runs
  `pnpm install` and the type-check only. It never runs:
  - `csp-hashes.py --check`;
  - `sitemap-lastmod`, which fails now because the companions `lastmod` is
    stale;
  - `next build`.
- **Two copies of every docs page.** Nothing generates the static
  `docs/*/index.html` from the MDX. Changes go to one and not the other:
  [tiagovilasboas/downshift-site#74](https://github.com/tiagovilasboas/downshift-site/pull/74)
  touched only the MDX, so the install, report and harness-matrix HTML still
  lack the next release's content.
- **Version bumped by hand.** `softwareVersion` is `0.6.0` at
  `index.html:33` and `pt-br/index.html:33`, while the latest release is
  v0.7.2.

## Pane problem

- `claude plugin test` asserts the component tree only. Nothing checks that
  the pane renders as designed cards.
- `card.tsx` is the only framer (`FRAME_MIN=32`). A component that prints raw
  text outside a card passes every test, against the owner rule that
  everything in the band and pane is a card.
- There are no golden renders, so a layout regression at a narrow width goes
  unseen.
- The mod tests run only in the local pre-push hook, not in CI.

## Tasks

| ID | Title | Pts | Depends on | Phase | Repo |
|----|-------|-----|------------|-------|------|
| ST-1 | Rehash the CSP and add a required static-checks CI job | 2 | none | 1 | site |
| ST-2 | lefthook runs the same static checks locally | 1 | ST-1 | 1 | site |
| ST-5 | Owner decision: which file is the docs source | 1 | none | 1 | site |
| ST-4 | Re-sync the install, report and harness-matrix HTML | 2 | ST-5 | 2 | site |
| ST-3 | `docs-parity.py`: MDX and HTML say the same thing | 3 | ST-5 | 2 | site |
| ST-6 | Generate the HTML from the MDX | 3 | ST-5 picks generation | 2 | site |
| ST-7 | `next build` in CI | 2 | ST-1 | 2 | site |
| ST-8 | `set-version` script and a release-triggered PR | 2 | ST-1 | 3 | site |
| UI-1 | Raw-text guard: every visible node sits in a card | 3 | none | 1 | downshift |
| UI-2 | Golden text render at fixed widths | 5 | UI-1 | 2 | downshift |
| UI-3 | Release QA checklist with screenshots | 2 | none | 1 | downshift |
| UI-4 | Mod tests in CI | 2 | UI-1 | 3 | downshift |

### ST-1: CSP rehash and static-checks job (2)

**Design**

- Run `csp-hashes.py` in write mode, so the policy pins exactly the hashes in
  use.
- Add a `static-checks` job that runs:
  - `csp-hashes.py --check`;
  - the `sitemap-lastmod` check;
  - the type-check.
- Fix the companions `lastmod` in the same PR.
- The job is the check that `merge-when-green.sh` waits for.

**Acceptance**

- An inline script added without a hash fails CI.
- A stale hash fails CI.
- The live CSP header lists 1 script hash and 4 style hashes.

### ST-2: lefthook (1)

**Design**

- `lefthook.yml` with a version pinned in `package.json`:
  - pre-commit runs the type-check on staged files;
  - pre-push runs the static-checks job.
- `pnpm hooks` installs it, and the README says so.

### ST-5: Source of truth (owner decision, 1)

Pick one:

- **A. HTML is the source (recommended).** Delete the MDX app. The served
  pages are the static HTML, and the MDX copy is only drift risk.
- **B. MDX is the source.** Build the HTML from it (ST-6).

ST-3, ST-4 and ST-6 wait on this answer.

### ST-4: Re-sync three pages (2)

**Design**

- Bring the install, report and harness-matrix HTML in line with the MDX
  changes from
  [tiagovilasboas/downshift-site#74](https://github.com/tiagovilasboas/downshift-site/pull/74)
  and [tiagovilasboas/downshift-site#75](https://github.com/tiagovilasboas/downshift-site/pull/75).
- Under option A this is the last manual sync before the MDX is deleted.

### ST-3: docs-parity.py (3, option B only)

**Design**

- Compare the headings and code blocks of each MDX page with its HTML page.
- CI fails on a difference.
- Under option A this becomes a check that no MDX is left.

### ST-6: Generate HTML (3, option B only)

**Design**

- A build step renders the MDX into `docs/*/index.html` and rehashes the CSP.
- CI fails if the committed HTML differs from what the step generates.

### ST-7: next build in CI (2)

**Design**

- Add `next build` to the CI job, with a pnpm store cache.
- If option A deletes the Next app, this task is dropped.

### ST-8: Version script (2)

**Design**

- `scripts/set-version.py <v>` updates `softwareVersion`, the download links
  and the changelog link in both locales.
- A workflow started by a downshift release (`repository_dispatch`, or a
  manual dispatch) opens the PR. It never merges it.

**Acceptance**

- After a release, a PR appears that bumps every version string.

### UI-1: Raw-text card guard (3)

**Design**

- A test walks the rendered tree of every pane and band view and fails when a
  text node is not inside a `Card` from `card.tsx`.
- A test-only allowlist covers the card title and body slots.

**Tests**

- A fixture view with a bare string fails.
- Every current view passes, or is fixed in the same PR.

**Acceptance**

- The owner rule that the band and pane are drawn as cards is enforced in
  code.

### UI-2: Golden text render (5)

**Design**

- Render each view at widths 32, 60 and 100 columns to plain text, and commit
  the goldens.
- The test diffs against the goldens and refreshes them with an update flag.
- The fixtures use data with no real paths, names or prompts.

**Acceptance**

- A one-column layout shift fails, with a readable diff.

### UI-3: Release QA checklist (2)

**Design**

- A `docs/contrib/release-qa.md` section: open the pane in a new session, take
  screenshots at a narrow and a wide width, and check them against the goldens
  by eye.
- The release notes PR links the screenshots.
- This is the visual check the 8/10 review found missing.

### UI-4: Mod tests in CI (2)

**Design**

- Run the mod's own tests (not `claude plugin test`, which needs the
  harness) in a CI job.
- If the plugin test runner cannot run headless on a GitHub runner, say so in
  the job and keep it in pre-push.

**Acceptance**

- A broken mod test fails CI, not only pre-push.
