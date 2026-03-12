# Monitoring & Diagnostics

Canonical monitoring and troubleshooting references for Perles team mode.

## Debug Logging

Enable debug mode:

```bash
./perles -d
PERLES_DEBUG=1 ./perles
```

Log file path:

- Default: `$XDG_STATE_HOME/perles/logs/<basename>-<short-hash>/YYYY-MM-DD-perles.log`
- Fallback when `XDG_STATE_HOME` is unset: `~/.local/state/perles/logs/<basename>-<short-hash>/YYYY-MM-DD-perles.log`
- Override: `PERLES_LOG=/path/to/log ./perles -d`

Perles derives the project log directory from the current working directory:

- `<basename>` is the current folder name
- `<short-hash>` is a stable short hash of the normalized full path

This keeps logs for same-named folders in different locations from colliding while still making the directories easy to browse.

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
