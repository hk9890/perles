# Beads Compatibility and Support Policy

This document defines the **supported beads contract** for Perles and the migration/repair path for incompatible projects.

## Scope

Perles currently targets beads v1-era projects with a Dolt server runtime. This is an intentional compatibility boundary, not a transient startup warning.

## Support Matrix

| Area | Supported | Not supported (non-goals for now) |
|---|---|---|
| beads version | `v1.0.0+` | `< v1.0.0` |
| backend | `backend=dolt` | non-Dolt backends |
| Dolt runtime mode | `dolt_mode=server` | `dolt_mode=embedded`, `dolt_mode=shared-server` |
| project metadata/layout | `.beads/metadata.json` + server-mode Dolt runtime artifacts | stale/missing metadata, stale port/config artifacts |
| issue model | v1 issue/status/type schema and JSON surfaces used by Perles | pre-v1 schemas/layouts and behavior assumptions |

## Issue model expectations

Perles assumes the beads v1 model implemented by this repository:

- status/type support includes v1 built-ins and custom status/type tables
- BQL pseudo-fields align with v1 behavior
  - `ready=true` follows v1 readiness semantics
  - `blocked=true` follows v1 dependency-blocked semantics

### Upgrade note for existing users

If you have custom board queries in `.perles/config.yaml`, review them after upgrading. In particular, queries using `ready=true` and `blocked=true` may route issues differently than pre-v1 Perles behavior.

## Install sources and local tooling

Current upstream beads source:

- Repository: `https://github.com/gastownhall/beads`
- Releases: `https://github.com/gastownhall/beads/releases`
- Go install: `go install github.com/gastownhall/beads/cmd/bd@latest`

For this repository's compatibility work, a project-local tool pin is committed in `mise.toml`:

```toml
[tools]
"github:gastownhall/beads" = "1.0.0"
```

Use it for deterministic local checks:

```bash
mise x github:gastownhall/beads@1.0.0 -- bd version
```

## Migration / Repair Guidance

### 1) Stale or incompatible layout/runtime

Symptoms:

- Perles startup reports compatibility/runtime errors
- missing/invalid `.beads/metadata.json`
- stale Dolt server connection metadata

Recommended path:

```bash
bd bootstrap
```

Then retry Perles. `bd bootstrap` is the non-destructive repair/setup path expected by Perles startup diagnostics.

### 2) Pre-v1 users or repositories

Symptoms:

- beads version is below `1.0.0`
- pre-v1 schema/runtime assumptions

Recommended path:

1. Upgrade beads CLI/runtime to `v1.0.0+`
2. Run `bd bootstrap` in the project root
3. Verify runtime metadata is Dolt server mode
4. Re-run Perles and review custom board queries (`ready`/`blocked` semantics)

### 3) Embedded/shared-server projects

Perles intentionally does **not** support:

- `dolt_mode=embedded`
- `dolt_mode=shared-server`

Recommended path:

1. Reconfigure project runtime to `dolt_mode=server`
2. Run `bd bootstrap`
3. If needed, run `bd dolt start`
4. Retry Perles

## Why these boundaries exist

Perles startup/runtime and BQL behavior for v1 compatibility are implemented against:

- beads v1 schema/runtime contract
- Dolt server mode metadata and connectivity model

Supporting embedded/shared-server and non-Dolt backends is tracked as future work, not part of current compatibility guarantees.
