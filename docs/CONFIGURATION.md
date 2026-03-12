# Configuration Guide

Canonical configuration reference for Perles team mode.

## Config File Resolution Order

At startup (`cmd/root.go`), Perles resolves config in this order:

1. `--config` / `-c` explicit path
2. `.perles/config.yaml` in current directory
3. `~/.config/perles/config.yaml`

If no file is found, Perles creates `.perles/config.yaml` with defaults.

## Key CLI Flags

- `--beads-dir`, `-b` - beads directory override
- `--config`, `-c` - config path override
- `--debug`, `-d` - enable debug logging
- `--port`, `-p` - orchestration API port override

## Important Environment Variables

- `BEADS_DIR` - beads path fallback when `-b` is not set
- `PERLES_DEBUG` - enables debug logging
- `PERLES_LOG` - overrides the centralized debug log file path (default: `$XDG_STATE_HOME/perles/logs/<basename>-<short-hash>/YYYY-MM-DD-perles.log`, fallback `~/.local/state/perles/logs/<basename>-<short-hash>/YYYY-MM-DD-perles.log`)
- `UPDATE_GOLDEN` - used by some tests/golden flows

## Core Config Schema (High-Level)

Top-level fields from `internal/config/config.go`:

- `beads_dir`
- `auto_refresh`
- `ui`
- `theme`
- `views`
- `orchestration`
- `sound`
- `flags`

## Minimal Example

```yaml
auto_refresh: true

ui:
  show_counts: true
  show_status_bar: true
  markdown_style: dark
  vim_mode: false

views:
  - name: Default
    columns:
      - name: Blocked
        type: bql
        query: "status = open and blocked = true"
        color: "#FF8787"

orchestration:
  coordinator_client: claude
  worker_client: claude
  codex:
    model: gpt-5.2-codex
```

## Views and Columns

Column types:

- `bql` (default): requires `query`
- `tree`: requires `issue_id`, optional `tree_mode` (`deps`/`child`)

Validation is enforced by `ValidateColumns` and `ValidateViews`.

## Orchestration Configuration

Supported clients include: `claude`, `amp`, `codex`, `gemini`, `opencode`.

Selection precedence:

- coordinator: `coordinator_client` > `client` > default
- worker: `worker_client` > `client` > default

Notable orchestration sections:

- `orchestration.{coordinator_client,worker_client,observer_*}`
- provider settings under `claude`, `amp`, `codex`, `gemini`, `opencode`
- `session_storage` (`base_dir`, `application_name`)
- `templates.document_path`
- `timeouts.worktree_creation`
- `tracing` (`enabled`, `exporter`, `file_path`, `otlp_endpoint`, `sample_rate`)

## User Actions

`ui.actions.issue_action` allows shell commands bound to issue-context keys.

Current restriction: keys must be numeric `0-9`.

Template variables:

- `{{.ID}}`
- `{{.TitleText}}` (shell-escaped)

Example source: `examples/user-actions.yaml` is referenced in project docs (if present in your branch).

## BQL Quick Reference

Use BQL in search and BQL columns.

- Operators: `=`, `!=`, `<`, `>`, `<=`, `>=`, `~`, `!~`, `in`, `not in`
- Logical: `and`, `or`, `not`
- Clauses: `order by`, `expand up|down|all [depth N|*]`

Examples:

```bql
type = bug and priority = P0
status != closed and ready = true
type = epic expand down depth *
```

## Where to Look for Full Defaults

- `internal/config/config.go` (`Defaults`, validation methods, template)
- `cmd/root.go` (resolution and precedence behavior)
- `cmd/init.go` (`perles init` behavior)

## Related Docs

- [ARCHITECTURE.md](./ARCHITECTURE.md)
- [MONITORING.md](./MONITORING.md)
- [README.md](../README.md)
