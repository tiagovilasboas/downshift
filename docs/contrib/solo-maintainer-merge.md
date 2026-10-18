# Solo maintainer — merging your own pull requests

Downshift `main` requires the **`test`** CI check, **one approving review**, and **CODEOWNERS** (`@tiagovilasboas`). GitHub does not let you approve your own PR, so a PR opened under your account stays unmergeable until one of the options below applies.

## Recommended (Cloud Agent + local)

1. **Settings → Actions → General** → enable **Allow GitHub Actions to create and approve pull requests** (one-time).
2. **Settings → Branches → `main`** → edit protection → either:
   - turn off **Require review from Code Owners** (keep **1 approval**), and add the workflow below; or
   - set **Required approvals** to **0** while you are the only maintainer (simplest for a solo repo).
3. Merge in the GitHub UI or with `gh pr merge <n> --squash --delete-branch`.

Cloud Agents use the **`cursor`** GitHub App for `git push`; they **cannot** change branch rules (403) or approve PRs authored as **tiagovilasboas**.

## Quick merge (local `gh`, after step 2)

```bash
gh pr merge 166 --repo tiagovilasboas/downshift --squash --delete-branch
gh pr merge 168 --repo tiagovilasboas/downshift --squash --delete-branch
```

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift
