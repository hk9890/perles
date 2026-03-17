# Monitoring & Diagnostics

Canonical monitoring and troubleshooting references for Perles team mode.

## Perles Runtime Logs

Perles writes runtime logs in both normal and debug runs.

- **Normal run** (`./perles`): logs are enabled at **ERROR** level.
- **Debug run** (`./perles -d` or `PERLES_DEBUG=1 ./perles`): logs are enabled at **DEBUG** level.

This means user-visible errors should still be logged in normal usage, while debug mode adds lower-severity diagnostics.

### Log file location

Default path shape:

`$STATE_ROOT/perles/logs/<project-base>-<short-hash>/YYYY-MM-DD-perles.log`

State root resolution order:

1. `$XDG_STATE_HOME`
2. `~/.local/state`
3. `.` (safe fallback if home cannot be resolved)

You can override the log file path in either mode:

```bash
PERLES_LOG=/path/to/perles.log ./perles
PERLES_LOG=/path/to/perles.log ./perles -d
```

### How project-specific log directories are derived

Perles derives `<project-base>-<short-hash>` from the current working directory at startup:

- `<project-base>`: basename of the normalized working directory path.
- `<short-hash>`: first 8 hex chars of a stable SHA-256 hash of that normalized full path.

This prevents collisions between repositories with the same folder name in different locations while keeping directories human-browsable.

## UI Error Logging Contract (Regression Guard)

When a user sees an error in Perles UI, there must be a corresponding Perles log entry.

- UI-visible failures are logged under category `ui-error`.
- Entries should include `ui_message=<user-visible text or stable prefix>`.
- Suppressed outage toasts are still logged with `toast_suppressed=true`.

### Correlating a UI error with logs

Recommended workflow:

1. Copy the exact UI message (or stable prefix) shown to the user.
2. Search Perles logs for `ui_message=<that text>` and/or `[ui-error]`.
3. Use context fields (`issue_id`, `backend_state`, etc.) to identify the exact occurrence.
4. If needed, correlate to backend root-cause entries (for example `[db]` with `operation=get_comments`).

Example:

- UI shows: `Failed to load comments`
- First search: `ui_message=Failed to load comments`
- Then inspect related backend entries, such as `operation=get_comments`, for root cause details.

### Perles logs vs Dolt server logs

- **Perles logs are primary evidence** for user-visible UI failures and should contain the actionable UI-to-root-cause chain.
- **Dolt server logs are secondary evidence** when deeper database/transport diagnosis is needed (for example SQL engine startup failures, network/socket behavior, or server-side internals not emitted by Perles).

### Requirement for new user-visible errors

Any newly introduced user-visible error string (inline error, banner, toast, modal/action failure) must emit a corresponding log entry at the point the error state is entered (not from `View()` rendering loops).

## Session Artifacts (Orchestration)

Orchestration session storage default:

```text
~/.perles/sessions/
```

Common artifacts:

- `metadata.json`
- `messages.jsonl`
- `coordinator/`
- `workers/`

Use these to reconstruct workflow behavior and investigate failures.

## Health Signals

- Local quality gates: `make test`, `make lint`
- CI state: `.github/workflows/ci.yml`
- Release pipeline state: `.github/workflows/release.yml`

## Team-Mode Operational Verification

Use this checklist when validating migration away from stealth-mode dependence:

- [ ] `.git/info/exclude` reviewed; stealth marker entries are not being used to hide your primary workflow docs.
- [ ] Root `AGENTS.md` is the active router used by contributors.
- [ ] Topic-specific instructions are documented in tracked `docs/*` files.
- [ ] `.coder/*` is treated as optional local supplement only.
- [ ] A clean clone (without local `.coder/*` customizations) still provides complete contributor workflow guidance.

## Control Plane References

For multi-workflow health policies, recovery behavior, and port allocation:

- [docs/CONTROL_PLANE.md](./CONTROL_PLANE.md)
- [ORCHESTRATION.md](../ORCHESTRATION.md)

## Bug Report Guidance

When possible, include:

- `perles --version`
- `go version`
- relevant debug log excerpt or file path
- reproduction steps and config context

## Related Docs

- [CONFIGURATION.md](./CONFIGURATION.md)
- [ARCHITECTURE.md](./ARCHITECTURE.md)
