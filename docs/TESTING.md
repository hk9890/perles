# Testing Guide

Canonical testing guidance for Perles team mode.

## Primary Commands

```bash
make test        # Run all Go tests
make test-v      # Verbose Go tests
make test-update # Refresh golden snapshot files
make lint        # Run golangci-lint
```

Frontend tests (run in `frontend/`):

```bash
npm run test:run   # Vitest one-shot
npm test           # Vitest watch mode
npm run test:e2e   # Playwright
npm run test:e2e:ui
```

## Manual TUI Exploration

Use these when you want to launch a real TUI and interact with it by hand:

```bash
make run         # Build Go binary and launch the main Perles app
make debug       # Launch main app with debug logging enabled
make playground  # Launch the component playground TUI
```

- `make run` / `make debug` exercise the real application shell.
- `make playground` is a component sandbox for trying widgets and interaction patterns quickly.

## Test Types in This Repo

- Unit tests: standard `testing` + `testify/require`
- Message-driven TUI tests: call Bubble Tea `Update` directly with `tea.KeyMsg`, `tea.WindowSizeMsg`, and app-specific messages
- Golden/snapshot tests: `teatest`
- In-process interactive TUI tests: `teatest.NewTestModel`, `.Send(...)`, `.Type(...)`, `.Output()`, and `teatest.WaitFor(...)`
- Property-style tests (targeted areas): `rapid`

## How TUI Testing Works Here

Perles is primarily tested **in process**, not through an external terminal automation harness.

### 1. Deterministic model tests first

Most TUI behavior is tested by constructing a model and sending it messages directly:

- key presses via `tea.KeyMsg`
- terminal resize via `tea.WindowSizeMsg`
- domain/application messages emitted by modes

This is the preferred style for:

- keybinding behavior
- mode switches
- focus changes
- modal open/close logic
- state transitions that should stay deterministic

Typical examples live under:

- `internal/app/*_test.go`
- `internal/mode/*/*_test.go`
- `internal/ui/**/*_test.go`

### 2. Golden tests for rendered output

When the important assertion is the rendered screen, tests usually:

1. build a model with fixed test data
2. set a known terminal size
3. call `View()`
4. compare the output with `teatest.RequireEqualOutput`

This is the main snapshot workflow for modes and shared UI components.

### 3. In-process interactive Bubble Tea tests

For higher-fidelity interaction, the repo can run a Bubble Tea program inside tests using `teatest.NewTestModel(...)` and then:

- send keys/messages with `.Send(...)`
- type text with `.Type(...)`
- inspect output with `.Output()` / `.FinalOutput()`
- wait for expected screen changes with `teatest.WaitFor(...)`

Use this when you want something closer to real user interaction without leaving the Go test process.

### 4. Playground is a testing surface for components

`perles playground` / `make playground` launches `cmd/playground.go`, which hosts reusable UI demos in one Bubble Tea app.

Use playground for:

- manual exploration of widgets
- rapid interaction checks while developing a component
- component-focused golden tests

Playground is **not** a replacement for testing the full Perles application flow. Treat it as a component lab, not as the main app.

### 5. Full-app vs playground guidance

- Test the real app shell and mode integration through `internal/app` and the real mode packages.
- Test reusable widgets and isolated interaction patterns through `internal/mode/playground` and `internal/ui/*`.

### 6. Current scope: no PTY-based TUI E2E harness

The current repository testing strategy is model-based and in-process. There is not currently a dedicated PTY/expect-style harness for driving the compiled `perles` binary from an external pseudo-terminal.

If you need to "start it, see what happens, and press keys," the supported approaches today are:

- manual exploration with `make run`, `make debug`, or `make playground`
- automated model tests with direct `Update(...)`
- automated in-process interaction tests with `teatest`

## TUI Test Stability Guidelines

For stable tests, prefer the following patterns:

- set an explicit terminal size in the test
- use fixed clocks/timestamps for golden output
- mock external dependencies such as clipboard, clients, executors, and control-plane services
- keep Bubble Tea updates message-driven and deterministic
- use golden files only when screen rendering is the thing you care about

## Common Interaction Patterns To Test

Depending on the surface under test, common interactions include:

- navigation keys like `j`, `k`, arrows, `tab`, `enter`, `esc`
- app-level toggles such as search mode, dashboard, or chat panel
- text entry through `tea.KeyMsg` or `teatest.TestModel.Type(...)`
- resize behavior through `tea.WindowSizeMsg`

## Golden File Workflow

When UI output intentionally changes:

1. Run `make test` and review failures
2. Run `make test-update`
3. Review all golden diffs before commit

`make test-update` updates a curated package list from the `Makefile`.

## Recommended Validation Before PR

- Go-only changes: `make test && make lint`
- TUI or interaction changes: `make test` and update goldens if rendered output changed
- Frontend changes: `npm run build && npm run test:run`
- Cross-stack changes: `make build && make test && make lint`

## beads v1.0 compatibility harness (perles-38b)

The repo now includes a compatibility-focused harness that models the beads
v1.0 contract documented in `docs/BEADS-V1-SPEC.md`.

Primary coverage lives in:

- `internal/testutil/db.go` (`NewBeadsV1TestDB`, `BeadsV1Schema`)
- `internal/testutil/presets.go` (`WithBeadsV1CompatibilityData`)
- `internal/beads/infrastructure/*_test.go`
- `internal/bql/executor_test.go`
- `cmd/root_test.go`

Suggested focused run:

```bash
go test ./internal/testutil ./internal/beads/infrastructure ./internal/bql ./cmd
```

### Captured v1 contract artifacts

- `internal/beads/infrastructure/testdata/beads_v1_show_issue.json`
- `internal/beads/infrastructure/testdata/beads_v1_cli_contract.md`

To verify local CLI pin when refreshing live captures:

```bash
mise x github:gastownhall/beads@1.0.0 -- bd version
```

The pin above is committed in repository-local `mise.toml` so compatibility validation does not depend on untracked local tool state.

### Fixture-covered vs live/manual checks

Fixture-covered by tests:

- startup compatibility detection for embedded-mode metadata classification
- issue loading and BQL execution against v1-compatible schema tables
- status/type filtering with v1 built-ins + categorized custom statuses/types
- `bd show --json` parsing path with captured v1 output shape golden

Still requiring live/manual verification against a real beads v1 repo:

- end-to-end `bd` command execution against a running backend (process/env/path)
- Dolt-specific SQL/view behavior parity beyond fixture SQL approximations
- upstream CLI output drift beyond currently captured golden samples

## CI Alignment

CI (`.github/workflows/ci.yml`) currently runs:

- `make build`
- `make test`
- golangci-lint on Ubuntu
- test/build matrix on Ubuntu, macOS, and Windows

## Related Docs

- [CODING.md](./CODING.md)
- [PULL-REQUESTS.md](./PULL-REQUESTS.md)
- [CONTRIBUTING.md](../CONTRIBUTING.md)
