# Coding Guide

Canonical coding guidance for Perles team mode.

## Core Commands

From the repository root:

```bash
make build          # Build frontend + Go binary
make build-go       # Build Go binary only
make build-frontend # Build frontend assets only
make run            # Build Go binary and run
make debug          # Run with debug logging
make install        # Install binary with ldflags version
make lint           # Run golangci-lint
make mocks          # Regenerate mocks (mockery)
go fmt ./...        # Format Go code
```

Frontend commands (run in `frontend/`):

```bash
npm run dev
npm run build
```

## Codebase Layout (Developer-Focused)

- `cmd/` - cobra commands and startup wiring
- `internal/app` - root Bubble Tea application model
- `internal/mode/*` - mode controllers (kanban/search/orchestration/dashboard/playground)
- `internal/ui/*` - reusable and mode-specific TUI components
- `internal/orchestration/*` - headless clients, V2 processor, control plane, session/metrics
- `internal/bql` - query lexer/parser/executor/validator
- `internal/config` - config schema, defaults, validation, persistence
- `internal/testutil` - test DB and fixture builders
- `frontend/` - React/Vite session viewer

For architecture-level details, see [ARCHITECTURE.md](./ARCHITECTURE.md).

## Conventions

- Keep packages small and focused.
- File names use snake_case (`foo_bar.go`, `foo_bar_test.go`).
- Exported identifiers use `PascalCase`; internal identifiers use `camelCase`.
- Bubble Tea updates should stay message-driven and deterministic.
- Return errors explicitly and wrap context with `%w`.
- Keep tests adjacent to implementation; snapshots/goldens live in `testdata/`.

## Common Development Patterns

- Add new keys in `internal/keys` and keep help text in sync.
- Prefer dependency injection via service structs/interfaces over global state.
- For generated mocks, run `make mocks` after interface changes.

## Module Path

- Canonical module path for this fork is `github.com/hk9890/perles`.
- Public `go install` and clone instructions must use the fork-owned path.

## Team-Mode Activation (Stop Relying on Stealth)

Team mode is authoritative when workflow guidance comes from tracked files in this repository, not from hidden local overlays.

1. Open `.git/info/exclude`.
2. Locate the `# opencode-coder stealth mode` block.
3. Remove or comment any patterns that hide team-workflow inputs (for example `.coder/`, `.opencode/`, `.beads/`, `ai.package.yaml`) when those patterns are being used as your primary handbook.
4. Keep only truly personal, machine-specific excludes (for example local editor settings).

### Team vs Local Assets

Authoritative, tracked team assets:

- `AGENTS.md` (small router only)
- `docs/CODING.md`
- `docs/TESTING.md`
- `docs/PULL-REQUESTS.md`
- `docs/MONITORING.md`
- other topic docs under `docs/`

Optional local-only supplementary assets:

- `.coder/AGENTS.md`
- `.coder/docs/*`
- other `.coder/*` machine-local helpers

Local-only assets are supplementary. They must not replace tracked team docs as the source of truth.

### Verification Checklist (Team Mode Is Authoritative)

- [ ] `AGENTS.md` routes to tracked `docs/*` for detailed instructions.
- [ ] `AGENTS.md` stays small; topic detail lives in dedicated docs.
- [ ] `.git/info/exclude` does not hide files you depend on as primary project guidance.
- [ ] Commands and workflows you follow are present in tracked docs (`docs/*`, `CONTRIBUTING.md`).
- [ ] Any `.coder/*` content is additive and can be removed without losing required project process.

## Related Docs

- [TESTING.md](./TESTING.md)
- [ARCHITECTURE.md](./ARCHITECTURE.md)
- [CONFIGURATION.md](./CONFIGURATION.md)
- [CONTRIBUTING.md](../CONTRIBUTING.md)
