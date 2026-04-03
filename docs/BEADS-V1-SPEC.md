# Beads v1.0.0 Compatibility Contract for Perles

This document captures the **actual** beads `v1.0.0` contract Perles must support.
It is grounded in upstream source (`gastownhall/beads` tag `v1.0.0`) plus real CLI output from the repo-local mise-pinned toolchain:

- `mise.toml` pins `github:gastownhall/beads = 1.0.0`
- verification command: `mise x github:gastownhall/beads@1.0.0 -- bd version`

---

## 1) Evidence Sources

Primary upstream references:

- Schema (server backend):
  - `internal/storage/dolt/schema.go`
  - `internal/storage/dolt/migrations/015_custom_status_type_tables.go`
- Schema (embedded backend):
  - `internal/storage/embeddeddolt/schema/*.up.sql`
- CLI contracts:
  - `cmd/bd/show.go`
  - `cmd/bd/statuses.go`
  - `cmd/bd/types.go`
  - `cmd/bd/update.go`
  - `internal/types/types.go`
- Readiness/blocking semantics:
  - `internal/storage/dolt/schema.go` (`ready_issues`, `blocked_issues` views)
  - `cmd/bd/protocol/blocked_status_test.go`

Runtime artifacts / command output captured against `bd 1.0.0`:

- `bd init --non-interactive` in a temporary workspace
- `.beads/metadata.json` and `.beads/config.yaml`
- `bd show --json`, `bd statuses --json`, `bd types --json`, `bd context --json`, `bd list --json`

---

## 2) Runtime/Layout Contract (v1.0.0)

## 2.1 Default mode is embedded Dolt

`bd init` defaults to **embedded** mode (not server mode).

Observed metadata example after init:

```json
{
  "database": "dolt",
  "backend": "dolt",
  "dolt_mode": "embedded",
  "dolt_database": "perlesv1spec",
  "project_id": "..."
}
```

Observed context example:

```json
{
  "backend": "dolt",
  "dolt_mode": "embedded",
  "database": "perlesv1spec",
  "bd_version": "1.0.0"
}
```

## 2.2 Embedded vs server layout expectations

- Embedded mode stores data under `.beads/embeddeddolt/...`
- Server mode uses `.beads/dolt/...` and server connection settings
- `.beads/metadata.json` is authoritative for mode/backend identity

Perles currently requires `metadata.backend=dolt` **and** `metadata.dolt_mode=server` (see `internal/beads/infrastructure/dolt_client.go`), so default v1 init state is currently incompatible.

## 2.3 Startup/bootstrap expectations

- `bd bootstrap` is the non-destructive setup path
- Auto-detection behavior includes remote clone, backup restore, JSONL import, or fresh DB create (`cmd/bd/bootstrap.go`)
- `bd init` can operate in embedded or server mode; v1 favors embedded by default (`cmd/bd/init.go`)

---

## 3) Schema Contract Relevant to Perles

The core relational contract Perles depends on is present in v1.0.0:

- `issues`
- `dependencies`
- `labels`
- `comments`
- `config`
- `metadata`

From upstream schema (embedded + server parity):

## 3.1 `issues`

Relevant columns for Perles behavior:

- identity/content: `id`, `title`, `description`, `design`, `acceptance_criteria`, `notes`
- workflow: `status`, `priority`, `issue_type`
- assignment/time: `assignee`, `created_at`, `created_by`, `updated_at`, `closed_at`, `close_reason`
- flags used in BQL/board semantics: `ephemeral`, `pinned`, `is_template`
- scheduling: `defer_until`, `due_at`

## 3.2 `dependencies`

Relevant columns:

- `issue_id`, `depends_on_id`, `type`, `created_at`, `created_by`

Dependency type `blocks` is central for readiness/blocking behavior.

## 3.3 `labels`

- `issue_id`, `label`

## 3.4 `comments`

- `id`, `issue_id`, `author`, `text`, `created_at`

## 3.5 `metadata` + `config`

- generic key/value tables used by beads runtime/config
- v1 also introduces normalized tables:
  - `custom_statuses(name, category)`
  - `custom_types(name)`

Those are created and synced from config updates in `internal/storage/dolt/config.go` and migration `015_custom_status_type_tables.go`.

---

## 4) Readiness / Blocking / Status Category Semantics

## 4.1 Built-in statuses and categories

From `bd statuses --json` and `cmd/bd/statuses.go`:

- `open` → `active`
- `in_progress` → `wip`
- `blocked` → `wip`
- `deferred` → `frozen`
- `closed` → `done`
- `pinned` → `frozen`
- `hooked` → `wip`

## 4.2 Ready semantics in v1

`ready_issues` view (source) includes:

- statuses considered ready:
  - `status = 'open'`
  - OR custom statuses with category `active`
- excludes:
  - dependency-blocked via open blockers
  - ephemeral issues
  - deferred issues (`defer_until > now`)
  - children whose parent is deferred

Notably, `in_progress` is **not** treated as ready.

## 4.3 Blocking semantics in v1

- Dependency-blocked issues are discovered via `dependencies.type='blocks'` + blocker status filtering.
- Protocol test explicitly asserts blocked-by-dependency issues retain stored status (e.g. often `open`) and are not discovered via `--status blocked` alone.

This distinction matters for BQL and board behavior: computed blocked-ness and stored status are different surfaces.

---

## 5) CLI JSON Surfaces Perles Must Track

## 5.1 `bd show <id> --json`

Observed output is a JSON **array** of issue-detail objects.

Top-level fields seen:

- `id`, `title`, `description`, `status`, `priority`, `issue_type`
- `owner`, `created_at`, `created_by`, `updated_at`
- optional relational expansions:
  - `labels: []string`
  - `dependencies: []IssueWithDependencyMetadata`
  - `dependents: []IssueWithDependencyMetadata`
  - `comments: []Comment`
  - `parent`
  - epic-only rollups: `epic_total_children`, `epic_closed_children`, `epic_closeable`

Example (trimmed):

```json
[
  {
    "id": "perlesv1spec-dxi",
    "title": "Child task",
    "status": "in_review",
    "priority": 2,
    "issue_type": "task",
    "labels": ["has:open-questions", "needs:discussion"],
    "dependencies": [
      {
        "id": "perlesv1spec-4t5",
        "dependency_type": "blocks"
      }
    ],
    "comments": [
      {
        "id": "019d...",
        "issue_id": "perlesv1spec-dxi",
        "author": "tester",
        "text": "Investigating v1 contract",
        "created_at": "2026-04-03T17:42:17Z"
      }
    ]
  }
]
```

## 5.2 `bd statuses --json`

Observed shape:

```json
{
  "built_in_statuses": [
    {"name":"open","category":"active","icon":"○","description":"..."}
  ],
  "custom_statuses": [
    {"name":"in_review","category":"active"}
  ]
}
```

## 5.3 `bd types --json`

Observed shape:

```json
{
  "core_types": [
    {"name":"task","description":"..."}
  ],
  "custom_types": ["convoy","agent","role"]
}
```

Core types in v1.0.0 include at least:

- `task`, `bug`, `feature`, `chore`, `epic`, `decision`, `spike`, `story`, `milestone`

---

## 6) Custom Status/Type Behavior in v1.0.0

## 6.1 Storage model

v1.0.0 uses **both**:

- config keys (`status.custom`, `types.custom`)
- normalized tables (`custom_statuses`, `custom_types`)

with synchronization logic in `SetConfig`.

## 6.2 Important observed edge case (v1.0.0)

`bd statuses --json` supports category-annotated format (e.g. `in_review:active`), but `bd update --status` validation currently reads legacy raw names from config (`GetCustomStatuses`) and rejects category-annotated values.

Observed:

- `bd config set status.custom "in_review:active,..."` → shown in `bd statuses --json`
- `bd update <id> --status in_review` → rejected in v1.0.0
- `bd config set status.custom "in_review,qa_testing"` → update accepted

Downstream Perles work should treat this as part of **actual v1.0.0 behavior**, not intended design.

---

## 7) Differences vs Current Perles Assumptions

Key compatibility gaps to address in follow-up tasks:

1. **Runtime mode assumption mismatch**
   - Perles currently expects server mode only.
   - beads v1 defaults to embedded mode.

2. **`bd show --json` field mismatch**
   - Perles domain expects `type` JSON field and dependency arrays like `blocked_by`/`blocks`.
   - beads v1 emits `issue_type`, plus `dependencies`/`dependents` objects.

3. **Readiness semantics mismatch**
   - Perles BQL `ready` currently treats `(status=open OR status=in_progress)` as ready.
   - beads v1 ready contract uses `open` (plus custom `active`) and excludes `in_progress`.

4. **Status/type catalog drift**
   - Perles static UI/help lists older subsets (e.g. status/type lists) and omits v1 built-ins (`pinned`, `hooked`, `spike`, `story`, `milestone`).

5. **Custom status behavior nuance**
   - v1.0.0 currently has a discrepancy between `statuses` display and `update` validation for category-annotated `status.custom` values.

These are the required baseline facts for `perles-38b.1`..`perles-38b.5` implementation tickets.
