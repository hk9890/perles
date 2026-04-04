# beads v1 Manual Verification Runbook

Manual acceptance checks for Perles against a real beads v1 repository.
Scope is intentionally manual only (no automation framework).

## Supported boundary

- beads `v1.0.0+`
- `backend=dolt`
- `dolt_mode=server`
- Not supported: `dolt_mode=embedded`, `dolt_mode=shared-server`, pre-v1/incompatible layouts

## Prerequisites

From `/home/hans/dev/github/perles`:

```bash
mise x github:gastownhall/beads@1.0.0 -- bd version
make build-go
```

Expected:

- `bd version` reports `1.0.0` (or newer v1 if intentionally validating that)
- `make build-go` succeeds

`mise.toml` in this repo pins beads `1.0.0`; use `./bin/perles` for verification.

## Setup: temporary supported v1 repo

```bash
TMP_ROOT="$(mktemp -d)"
TEST_REPO="$TMP_ROOT/perles-v1-manual"
mkdir -p "$TEST_REPO" && cd "$TEST_REPO"
git init

mise x github:gastownhall/beads@1.0.0 -- bd init --non-interactive --server
mise x github:gastownhall/beads@1.0.0 -- bd bootstrap
mise x github:gastownhall/beads@1.0.0 -- bd context --json

mise x github:gastownhall/beads@1.0.0 -- bd create "Ready task" --type task --priority 2
mise x github:gastownhall/beads@1.0.0 -- bd create "Blocked bug" --type bug --priority 1
mise x github:gastownhall/beads@1.0.0 -- bd create "Feature in progress" --type feature --priority 3
mise x github:gastownhall/beads@1.0.0 -- bd update perles-v1-manual-003 --status in_progress
```

Context JSON must show `"backend":"dolt"` and `"dolt_mode":"server"`.

## Positive scenarios

1. **Startup/board load**
   - Run `./bin/perles --beads-dir "$TEST_REPO"` from the Perles repo root.
   - Expect normal startup (not outdated/compat screen) and issue data visible.

2. **Representative BQL queries**
   - In search mode run: `status = open`, `status = in_progress`, `type = bug`, `ready = true`, `blocked = true`.
   - Expect successful execution (no parse/runtime errors) and sensible results.

3. **`bd show --json` contract surface**
   - Run `mise x github:gastownhall/beads@1.0.0 -- bd show perles-v1-manual-001 --json`.
   - Expect command success and JSON array output with v1 issue object shape.

4. **Status/type editing and picker checks**
   - Open issue edit flow in Perles.
   - Confirm status picker includes: `open`, `in_progress`, `blocked`, `deferred`, `closed`, `pinned`, `hooked`.
   - Confirm type picker includes: `task`, `bug`, `feature`, `chore`, `epic`, `decision`, `spike`, `story`, `milestone`.
   - Save a status/type change and confirm refreshed state reflects it.

## Negative scenarios

Use separate temp repos for each check.

1. **Unsupported embedded/shared mode**
   - Create repo with `bd init --non-interactive` (embedded default) or `bd init --non-interactive --shared-server`.
   - Launch Perles against it.
   - Expect compatibility/outdated guidance indicating only `dolt_mode=server` is supported.

2. **Stale schema/layout metadata**
   - In a server-mode repo, run `rm .beads/metadata.json`.
   - Launch Perles.
   - Expect startup/runtime error guidance with repair direction (for example `bd bootstrap`).

3. **Pre-v1 or incompatible layout**
   - Point Perles at a known pre-v1/incompatible beads repo.
   - Expect compatibility error/screen (not silent success).

## Teardown

```bash
rm -rf "$TMP_ROOT"
```
