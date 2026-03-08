# Releasing Guide

Operational runbook for fork-owned releases from `hk9890/perles`.

## Scope and distribution strategy

- Fork owner: `hk9890`
- Release channel: **GitHub Releases only** (no Homebrew in this phase)
- First production release: `v0.1.0` bootstrap release stream

This runbook is grounded in:

- `.github/workflows/release.yml`
- `.goreleaser.yml`
- `install.sh`
- `README.md` (public install paths)

## Prerequisites

### GitHub/repository ownership

1. You have maintainer access to `hk9890/perles`.
2. GitHub Actions is enabled for the repository.
3. Tag pushes are allowed from your release branch (normally `main`).

### Local preflight

From repo root, run:

```bash
make build
make test
make lint
```

If release-impacting frontend changes landed, also validate frontend build:

```bash
cd frontend
npm ci
npm run build
```

## Secrets and required settings

Current release workflow uses only:

- `GITHUB_TOKEN` (GitHub Actions default token)

Notes:

- `.github/workflows/release.yml` sets `permissions: contents: write` so GoReleaser can create/update GitHub Releases.
- No additional repo secrets are required for the current GitHub-Releases-only model.

### Agent-verifiable checks (CLI/API)

Use `gh` to verify the release setup that an agent can actually inspect:

```bash
gh repo view --json nameWithOwner,defaultBranchRef,viewerPermission
gh api repos/hk9890/perles/actions/permissions
gh api repos/hk9890/perles/actions/permissions/workflow
gh secret list --app actions --repo hk9890/perles
```

Expected outcomes for this strategy:

- `nameWithOwner` is `hk9890/perles`
- Actions are enabled (`enabled: true`)
- Workflow token default is write (`default_workflow_permissions: write`)
- No custom Actions secrets are required for release (empty `gh secret list` is valid)

### Human-only provisioning checks (maintainer-owned)

These checks cannot be fully proven by an agent and must be confirmed by a maintainer in GitHub settings/UI:

1. Tag-triggered workflows are allowed by current repo/org policy for `v*` pushes.
2. Release maintainers have permission to create and push version tags.
3. If org policy later changes default workflow permissions, repository Actions settings are explicitly set to allow **Read and write** workflow token permissions.

Record completion of these human-only checks on the active release task before cutting a production tag.

## External resources and tokens

- None required for the current GitHub-Releases-only fork strategy.
- If distribution scope expands (for example Homebrew tap, package registry, signing service), create dedicated follow-up issue(s) for each new external dependency and required credential.

## Execution flow

### 1) CI snapshot preflight (no publish)

Use **Actions → Release → Run workflow** (`workflow_dispatch`) with:

- `publish = false`
- Optional `target_ref` (leave empty to test default branch HEAD)

Behavior from `.github/workflows/release.yml`:

- Runs `make test`
- Runs `goreleaser release --snapshot --clean`
- Forces `PUBLISH_GITHUB_RELEASES=false` (build/test validation only)

Use this before cutting a production tag.

### 1b) Local snapshot preflight (no publish)

From repo root:

```bash
make release-preflight
```

`make release-preflight` runs `goreleaser release --snapshot --clean` with `PUBLISH_GITHUB_RELEASES=false`.

What this preflight covers:

- Frontend build path used by releases (`cd frontend && npm ci && npm run build` via GoReleaser hooks)
- Release packaging matrix from `.goreleaser.yml` (linux/darwin, amd64/arm64)
- Archive/checksum generation (`checksums.txt`)
- Changelog/release-note generation logic

Expected success signals:

- Command exits successfully (code 0)
- `.dist/` contains generated archives and `checksums.txt`
- No GoReleaser hook/build/archive errors in output

What this preflight does **not** cover:

- Creating/uploading GitHub Release objects
- GitHub API permission/token behavior for publish operations
- Tag-triggered production execution path (`push` on `v*`)

Run this locally before `workflow_dispatch` or before pushing a production tag when release-related changes land.

### 2) Publish release

Publish is triggered by pushing a tag matching `v*`:

```bash
git checkout main
git pull --rebase
git tag v0.1.0
git push origin v0.1.0
```

Tag push behavior:

- Workflow runs `goreleaser release --clean`
- `PUBLISH_GITHUB_RELEASES=true` enables GitHub Release publishing
- Assets are produced per `.goreleaser.yml` for:
  - `linux`/`darwin`
  - `amd64`/`arm64`
  - plus `checksums.txt`

### 2b) Manual publish fallback (when tag push event is missing)

If tag push does not create a `push`-event run, publish manually via `workflow_dispatch`:

1. Open **Actions → Release → Run workflow**.
2. Set:
   - `publish = true`
   - `target_ref = refs/tags/vX.Y.Z` (preferred) or `vX.Y.Z`
3. Start the run.

Fallback behavior:

- Workflow checks out `target_ref` and publishes from that exact tag/ref.
- It runs `goreleaser release --clean` with `PUBLISH_GITHUB_RELEASES=true`.
- Snapshot mode remains available via `publish=false`.

Guardrail:

- `publish=true` without `target_ref` fails fast to prevent accidental publish from `main`.

## Verification checklist

After publish completes:

1. **Workflow status**: Release workflow is green for the tag.
2. **GitHub Release exists** at `https://github.com/hk9890/perles/releases`.
3. **Assets present**: expected platform archives + `checksums.txt`.
4. **Install script path is valid**:
   - `https://raw.githubusercontent.com/hk9890/perles/main/install.sh`
   - README install command still targets `hk9890/perles`.
5. **Smoke install** (fresh shell recommended):

```bash
curl -sSL https://raw.githubusercontent.com/hk9890/perles/main/install.sh | bash
perles --version
```

## Failure and recovery guidance

### Snapshot preflight fails

- Fix failing tests/build issues on branch.
- Re-run `workflow_dispatch` preflight until green.
- Do not push a release tag until preflight is stable.

### Tag publish fails before release is created

- Fix root cause in branch.
- Create a new patch tag (for example `v0.1.1`) and push it.
- Avoid force-moving/deleting published tags unless absolutely necessary.

If failure is specifically missing tag-triggered workflow execution, use the manual publish fallback in **2b** for the existing tag.

### Release exists but assets are wrong/incomplete

- Correct `.goreleaser.yml` / code / workflow inputs.
- Re-run by cutting the next patch tag (`vX.Y.Z+1`) so history stays auditable.
- Keep `mode: replace` behavior in mind for same-tag reruns, but prefer new tags for recovery.

### Install path regression

- Verify `README.md` and `install.sh` still point to `hk9890/perles`.
- Ship fixes, then publish the next patch tag.

## Bootstrap note for `v0.1.0`

`v0.1.0` is the first fork-owned production release. Keep release notes explicit that distribution is GitHub Releases only in this phase.

## Related docs

- [TESTING.md](./TESTING.md)
- [PULL-REQUESTS.md](./PULL-REQUESTS.md)
