# Monitoring & Diagnostics

Canonical monitoring and troubleshooting references for Perles team mode.

## Debug Logging

Enable debug mode:

```bash
./perles -d
PERLES_DEBUG=1 ./perles
```

Log file path:

- Default: `debug.log` in current directory
- Override: `PERLES_LOG=/path/to/log ./perles -d`

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
- relevant `debug.log` excerpt
- reproduction steps and config context

## Related Docs

- [CONFIGURATION.md](./CONFIGURATION.md)
- [ARCHITECTURE.md](./ARCHITECTURE.md)
