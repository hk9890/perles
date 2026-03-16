# Proposal: Harden Perles Dolt Connectivity Against beads v0.60 Upstream Issues

## Problem Statement

The beads v0.60.0 release (steveyegge/beads tag `v0.60.0`, released 2026-03-12) introduced a major transition to Dolt as the backing database, replacing SQLite. This transition has caused widespread breakage for users, leading to a rollback to v0.59. The upstream issues tracker shows 10+ open issues specifically about Dolt connectivity failures, and users report the move "made beads unusable" (GH#2573).

Perles is a Dolt client that connects to the beads Dolt server over the MySQL wire protocol. While Perles does not own the Dolt server or its schema, it is directly affected by every upstream Dolt issue because its TUI becomes unusable when the Dolt connection fails. The current Perles Dolt client has several fragile patterns that compound the upstream problems: a single-retry reconnection strategy, no periodic health checks, string-based error matching that may miss new failure modes, and limited user-facing diagnostics when connectivity degrades.

This proposal identifies the specific upstream issues, analyzes how they manifest in Perles, and provides an actionable plan to make Perles resilient to the Dolt instability patterns seen in v0.60.

## Research Findings

### Upstream Issues Taxonomy

After analyzing all open Dolt issues on `steveyegge/beads`, the problems fall into **five distinct categories**:

#### Category 1: Server Lifecycle & Port Management
These are the most impactful for Perles since they cause "server unreachable" errors.

| Issue | Title | Impact on Perles |
|-------|-------|-----------------|
| GH#2559 | On system restart, connecting to dolt fails | Perles shows "server unreachable" after reboot; requires `init --force` to recover |
| GH#2598 | Fresh init fails on shared port-0 circuit breaker | Perles can't connect if `bd init` left a stale circuit breaker file at port 0 |
| GH#2595 | `bd dolt start` kills healthy servers from other repos | Multi-repo users see Perles disconnect when another repo starts its server |
| GH#2590 | `bd` ignores configured port, connects to default 3307 | Port routing confusion causes PROJECT IDENTITY MISMATCH errors |

#### Category 2: Concurrency & Panic
| Issue | Title | Impact on Perles |
|-------|-------|-----------------|
| GH#2571 | `bd read` commands panic in Dolt engine when concurrent | Multi-agent setups trigger panics; connection drops on Perles side |

#### Category 3: Clone & Sync
| Issue | Title | Impact on Perles |
|-------|-------|-----------------|
| GH#2580 | `bd init` in fresh clone creates unrelated state; `bd dolt pull` fails with "no common ancestor" | New clone setups show empty issue lists |
| GH#2584 | `bd backup restore` imports all projects' issues into current project | Data integrity issue; wrong issues appear |

#### Category 4: General Usability
| Issue | Title | Impact on Perles |
|-------|-------|-----------------|
| GH#2573 | "Moving to dolt pretty much made beads unusable for me" | Tasks disappear, DB unreachable; epitomizes the v0.60 experience |
| GH#2583 | `bd doctor --dry-run` bypasses Dolt auto-start on cold repos | Diagnostic tools don't work; users can't self-recover |

#### Category 5: Fixes Already in v0.60 (Relevant Context)
v0.60 itself shipped ~40 Dolt bug fixes addressing:
- Ephemeral port assignment replacing hash-derived ports (GH#2415)
- Auto-commit before pull to prevent merge errors (GH#2474)
- Explicit transaction wrapping for DOLT_PULL (GH#2501)
- Metadata merge conflict resolution (GH#2466)
- Shared server mode for multi-repo setups (GH#2416)
- Project identity preflight and bootstrap command (GH#2438, GH#2372)
- Stale server cleanup inside flock (GH#2430)

Despite these fixes, the core problems persist because:
1. Port resolution is complex and has multiple code paths that can diverge
2. Server lifecycle management doesn't safely handle multi-repo scenarios
3. The embedded Dolt engine has thread-safety issues with concurrent reads
4. Fresh clone initialization creates divergent Dolt histories

### Existing Patterns in Perles

#### Dolt Client Architecture (Key Files)
- `internal/beads/infrastructure/dolt_client.go` (641 lines) -- Core connection, reconnection, port resolution
- `internal/beads/infrastructure/bd_dolt_starter.go` (98 lines) -- Delegated `bd dolt start` subprocess launcher
- `internal/beads/application/connectivity_errors.go` (88 lines) -- Error classification for recoverable errors
- `internal/beads/application/ports.go` (58 lines) -- ReadClient, Reconnector interfaces
- `internal/bql/executor.go` -- BQL query executor with retry-after-reconnect
- `internal/bql/text_search.go` -- Full-text search with retry-after-reconnect
- `cmd/root.go` (lines 229-264) -- Startup behavior, beadsDir resolution
- `internal/app/app.go` (lines 640-659, 874-882) -- Connectivity event handling, UI state

#### Connection Flow
1. `NewDoltClient(beadsDir)` resolves connection details from `metadata.json` + `dolt/config.yaml` + `dolt-server.port`
2. Opens MySQL connection via `go-sql-driver/mysql` connector
3. On failure, delegates to `bd dolt start` if host is local
4. After delegated start, re-resolves port (may have changed) and retries with 150ms/300ms/600ms backoff

#### Current Reconnection Strategy
- **Leader-follower pattern**: Only one goroutine reconnects; others wait on a shared channel (`dolt_client.go:422-434`)
- **State broadcasting**: Connectivity transitions published via pubsub (healthy -> reconnecting -> healthy/degraded)
- **Single retry**: Query paths attempt one reconnect + one retry on recoverable errors (`executor.go:135-136`, `text_search.go:142-148`)
- **Full reconnect on failure**: Re-resolves connection details, potentially delegates startup again (`dolt_client.go:486-513`)

#### Error Classification
`IsRecoverableConnectivityError` (connectivity_errors.go:45-87) classifies errors as recoverable via:
- `errors.Is` checks: `sql.ErrConnDone`, `driver.ErrBadConn`, `io.EOF`, `io.ErrUnexpectedEOF`
- `errors.As` for `net.OpError`: `ECONNREFUSED`, `ECONNRESET`, `EPIPE`
- MySQL error codes: 2006 (server gone away), 2013 (lost connection)
- String matching (10 patterns): "connection refused", "broken pipe", etc.

#### Connection Pool Configuration (`dolt_client.go:133-141`)
- Dial timeout: 5s, Read timeout: 5s, Write timeout: 5s
- MaxIdleConns: 1, MaxOpenConns: 4
- ConnMaxIdleTime: 30s, ConnMaxLifetime: 5min

### Files to Modify/Create

#### Files to Modify
- `internal/beads/infrastructure/dolt_client.go` -- Add health monitoring, improve retry logic, add diagnostic context
- `internal/beads/infrastructure/dolt_client_test.go` -- Test new health monitoring and retry behavior
- `internal/beads/application/connectivity_errors.go` -- Add new error patterns for v0.60 failures
- `internal/beads/application/connectivity_errors_test.go` -- Test new error patterns
- `internal/bql/executor.go` -- Configurable retry policy
- `internal/bql/text_search.go` -- Configurable retry policy
- `internal/app/app.go` -- Enhanced connectivity UI, health status display

#### Files to Create
- `internal/beads/infrastructure/health_monitor.go` -- Periodic health check goroutine
- `internal/beads/infrastructure/health_monitor_test.go` -- Tests for health monitor
- `internal/beads/infrastructure/retry_policy.go` -- Configurable retry/backoff policy
- `internal/beads/infrastructure/retry_policy_test.go` -- Tests for retry policy

### Technical Constraints
- Perles does NOT control the Dolt server or schema -- it's a pure client
- The `bd` CLI must be available in PATH for delegated startup
- Connection details can change at runtime (ephemeral ports)
- Port file can be stale (race between server write and client read)
- MySQL wire protocol: no ambient health signal, errors discovered on query
- Test schema in `testutil/db.go` is SQLite -- can't test real Dolt behavior in unit tests
- Package-level vars used for test monkey-patching (not ideal but established pattern)

## Implementation Plan

### Approach

The strategy is **defense-in-depth on the client side**: since Perles can't fix the upstream Dolt server issues, we harden every layer of the connection lifecycle to be resilient to the failure modes we've observed. This means better retry logic, proactive health monitoring, richer error diagnostics, and improved user feedback.

The approach has three pillars:
1. **Proactive health monitoring** -- Detect server unavailability before the user's query fails
2. **Smarter retry with exponential backoff** -- Replace single-retry with configurable multi-retry
3. **Better diagnostics and user feedback** -- When things fail, tell the user exactly what's wrong and how to fix it

### Steps

#### Step 1: Add Configurable Retry Policy with Exponential Backoff
**Rationale:** The current single-retry strategy fails when the Dolt server has brief flaps (down-up-down). Multi-retry with jitter handles the ephemeral port resolution race and transient server restarts seen in GH#2559 and GH#2595.

**Changes:**
- Create `internal/beads/infrastructure/retry_policy.go`:
  - `RetryPolicy` struct with `MaxAttempts`, `InitialBackoff`, `MaxBackoff`, `Jitter`
  - `DefaultQueryRetryPolicy()` -- 3 attempts, 200ms initial, 2s max, 20% jitter
  - `DefaultStartupRetryPolicy()` -- 5 attempts, 250ms initial, 5s max, 10% jitter
  - `Execute(fn)` method with exponential backoff and jitter
- Create `internal/beads/infrastructure/retry_policy_test.go`:
  - Test backoff calculation, jitter bounds, max attempts, early exit on non-recoverable error
- Update `internal/bql/executor.go` and `internal/bql/text_search.go`:
  - Replace hardcoded single-retry with `RetryPolicy.Execute()`

#### Step 2: Add Proactive Health Monitor
**Rationale:** Without a health check, connectivity issues are only discovered when a query fails (GH#2559, GH#2573). A periodic ping detects server unavailability early, triggers reconnection proactively, and gives the UI time to update the status indicator before the user sees an error.

**Changes:**
- Create `internal/beads/infrastructure/health_monitor.go`:
  - `HealthMonitor` struct wrapping a `DoltClient` and a periodic ticker
  - Configurable interval (default 10s, configurable down to 5s for multi-agent)
  - On ping failure: triggers `ReconnectIfRecoverable`
  - On recovery: publishes `ConnectivityStateHealthy` event
  - Graceful shutdown via `Stop()`
- Create `internal/beads/infrastructure/health_monitor_test.go`:
  - Test ping failure triggers reconnect
  - Test recovery broadcasts healthy state
  - Test graceful shutdown stops ticker
- Wire health monitor into `DoltClient` initialization (`dolt_client.go`):
  - Start monitor after successful connection
  - Stop monitor on `Close()`

#### Step 3: Expand Error Classification for v0.60 Failure Modes
**Rationale:** The v0.60 issues introduce new error patterns that the current `IsRecoverableConnectivityError` may not catch -- particularly "project identity mismatch" (GH#2590), "no common ancestor" (GH#2580), and circuit breaker open (GH#2598). While these aren't all "recoverable" in the retry sense, they need distinct classification for appropriate user messaging.

**Changes:**
- Update `internal/beads/application/connectivity_errors.go`:
  - Add `IsProjectMismatchError(err)` -- detects wrong-database connections
  - Add `IsCircuitBreakerError(err)` -- detects open circuit breaker blocking connection
  - Add `IsDoltPanicError(err)` -- detects upstream panic/crash indicators
  - Extend `IsRecoverableConnectivityError` with: "circuit breaker", "no such database"
- Update `internal/beads/application/connectivity_errors_test.go`:
  - Add test cases for each new error pattern
  - Add test cases for new string patterns in recoverable errors

#### Step 4: Add Connection Diagnostic Context
**Rationale:** When connectivity fails, users need actionable diagnostics. The error messages in GH#2559 and GH#2573 show that users don't know what's wrong or how to fix it. Perles should surface the connection details, server status, and recovery suggestions.

**Changes:**
- Update `internal/beads/infrastructure/dolt_client.go`:
  - Add `DiagnosticContext()` method returning structured diagnostics:
    - Current connection details (host, port, database)
    - Last connectivity state and timestamp
    - Whether delegated startup was attempted
    - Port resolution source (config.yaml vs dolt-server.port)
    - Suggestion text for the specific failure mode
  - Enrich `StartupError` with diagnostic suggestion field
- Update `internal/beads/infrastructure/dolt_client_test.go`:
  - Test diagnostic context generation for each failure scenario

#### Step 5: Improve Startup Resilience for Port Resolution Race
**Rationale:** The port resolution race (GH#2598, GH#2590) happens because `dolt-server.port` may not be written yet when Perles tries to read it after delegated startup. The current backoff (150ms/300ms/600ms) may not be enough.

**Changes:**
- Update `internal/beads/infrastructure/dolt_client.go`:
  - In `retryDoltClientConnection`: re-resolve connection details on EACH retry (not just once after delegated startup), since the port file may arrive between retries
  - Increase backoff ceiling: 200ms/500ms/1000ms/2000ms/4000ms (5 attempts)
  - Add port file polling: before first retry, check if `dolt-server.port` exists and wait briefly if not
  - Log each retry attempt with resolved port for debugging
- Update `internal/beads/infrastructure/dolt_client_test.go`:
  - Test port file arrival between retries
  - Test re-resolution picks up changed port

#### Step 6: Surface Connectivity Status in TUI
**Rationale:** The Perles TUI should clearly show the user when the Dolt connection is degraded, reconnecting, or failed, with actionable recovery suggestions -- not just let queries silently fail.

**Changes:**
- Update `internal/app/app.go`:
  - On `ConnectivityStateDegraded`: show persistent status bar message with recovery suggestion
  - On `ConnectivityStateReconnecting`: show animated reconnecting indicator
  - On recovery: briefly show "Reconnected" confirmation
  - Include diagnostic context in error views (port being used, last error, suggestion)

### Testing Strategy

**Unit Tests:**
- Retry policy: backoff calculation, max attempts, jitter bounds, non-recoverable early exit
- Health monitor: ping failure triggers reconnect, recovery broadcasts, graceful shutdown
- Error classification: all new error patterns, edge cases
- Port re-resolution: dynamic port changes between retries
- Diagnostic context: correct details for each failure scenario

**Test Files:**
- `internal/beads/infrastructure/retry_policy_test.go` (new)
- `internal/beads/infrastructure/health_monitor_test.go` (new)
- `internal/beads/infrastructure/dolt_client_test.go` (extend)
- `internal/beads/application/connectivity_errors_test.go` (extend)

**Manual Integration Testing:**
- Kill `dolt sql-server` while Perles is running -- verify reconnect
- Start Perles before `bd dolt start` -- verify delegated startup with new retry
- Run with wrong port in `dolt-server.port` -- verify diagnostic message
- Run with stale port file from previous session -- verify re-resolution

## Risks and Mitigations

- **Risk:** Health monitor adds a periodic goroutine that could leak or cause panics on shutdown
  - **Mitigation:** Use context-based cancellation with `Stop()` method; wire into `DoltClient.Close()`; test graceful shutdown explicitly

- **Risk:** More aggressive retry policy could hammer a struggling server
  - **Mitigation:** Exponential backoff with jitter prevents thundering herd; max backoff of 4s limits total retry window to ~8s; circuit breaker awareness skips retries when server is known-down

- **Risk:** New error classification string patterns could false-positive on unrelated errors
  - **Mitigation:** New patterns are highly specific ("project identity mismatch", "circuit breaker"); tested with exact and near-miss strings; structured checks (`errors.Is`/`errors.As`) preferred over strings where possible

- **Risk:** Port re-resolution on each retry adds filesystem reads
  - **Mitigation:** Reading two small files (`metadata.json` + `dolt-server.port`) is negligible (~1ms); only happens during error recovery, not on hot path; already done once today

- **Risk:** TUI status indicator could flicker during brief reconnect cycles
  - **Mitigation:** Add debounce: don't show "reconnecting" unless state persists for >500ms; "reconnected" confirmation shown for 2s then auto-dismissed

## Acceptance Criteria

- [ ] Configurable retry policy with exponential backoff replaces single-retry in executor and text_search
- [ ] Health monitor periodically pings Dolt and triggers proactive reconnection
- [ ] New error classification covers project mismatch, circuit breaker, and panic errors
- [ ] Connection diagnostic context includes host, port, database, and recovery suggestions
- [ ] Startup retry re-resolves connection details on each attempt (handles port file race)
- [ ] TUI shows connectivity state (degraded/reconnecting/recovered) with actionable messages
- [ ] All new code has unit tests with >80% coverage
- [ ] No dead code: every new function called from production code
- [ ] No test-only helpers: all methods serve production use cases
- [ ] Existing tests continue to pass
- [ ] `make build-go` succeeds

## Review: APPROVED

**Reviewer:** Proposal Reviewer (worker-2)

### Review Summary

- **Research accuracy:** Pass
- **Implementation feasibility:** Pass
- **Testing coverage:** Pass
- **Gaps:** Minor (documented below, none blocking)

### Detailed Verification

#### File Path Accuracy (10/11 correct)

All cited file paths exist and line counts are exact, with one exception:

| Cited Path | Status |
|------------|--------|
| `internal/beads/infrastructure/dolt_client.go` (641 lines) | ✅ Verified |
| `internal/beads/infrastructure/bd_dolt_starter.go` (98 lines) | ✅ Verified |
| `internal/beads/application/connectivity_errors.go` (88 lines) | ✅ Verified |
| `internal/beads/application/ports.go` (58 lines) | ✅ Verified |
| `internal/bql/executor.go` | ✅ Verified |
| `internal/bql/text_search.go` | ✅ Verified |
| `cmd/root.go` | ✅ Verified |
| `internal/app/app.go` | ✅ Verified |
| `testutil/db.go` | ⚠️ Wrong path — correct path is `internal/testutil/db.go` |
| `internal/beads/infrastructure/dolt_client_test.go` | ✅ Verified |
| `internal/beads/application/connectivity_errors_test.go` | ✅ Verified |

#### Pattern Observations (all verified)

1. **Leader-follower reconnect pattern** (dolt_client.go:422-434) — Verified. `beginReconnectAttempt()` uses a shared channel; leader reconnects, followers block on `<-waitCh`.
2. **Pubsub connectivity transitions** — Verified. `ConnectivityState` (3 states), `ConnectivityEvent`, `ConnectivityObserver` interface, published via `setConnectivityState()` (dolt_client.go:467-483), consumed in app.go:640-659.
3. **Single retry in query paths** — Verified. executor.go:117-124 and text_search.go:140-149 both do one reconnect + one retry. (Note: text_search.go line range is 140-149, not 142-148 as cited — off by ~1 line.)
4. **Full reconnect re-resolves details** (dolt_client.go:486-513) — Verified exactly.
5. **Connection pool configuration** (dolt_client.go:133-141) — All 7 values verified exactly.
6. **IsRecoverableConnectivityError** — Verified. Proposal omits the `syscall.ECONNREFUSED`/`ECONNRESET`/`EPIPE` sub-checks within the `net.OpError` branch (not wrong, just incomplete).
7. **Startup behavior / beadsDir resolution** (cmd/root.go:229-264) — Verified exactly.
8. **Connectivity event handling** (app.go:640-659 and 874-883) — Verified. Function ends at line 883 (claimed 882), off by 1.

#### Implementation Feasibility

- **Retry policy extraction:** Feasible. The existing `postStartReadinessBackoff` slice + monkey-patching pattern naturally evolves into a `RetryPolicy` struct. The `executor.go` and `text_search.go` single-retry logic is straightforward to replace.
- **Health monitor:** Feasible. `DoltClient` already has `Close()` for lifecycle management and pubsub for broadcasting. Adding a periodic ping goroutine with context cancellation fits cleanly.
- **Error classification expansion:** Feasible. `connectivity_errors.go` has a clear structure — adding new pattern functions (`IsProjectMismatchError`, `IsCircuitBreakerError`, `IsDoltPanicError`) follows the existing `IsRecoverableConnectivityError` pattern.
- **Port re-resolution on retry:** Feasible. `retryDoltClientConnection` already calls `ResolveConnectionDetails` once; calling it inside the retry loop is a minimal change.
- **TUI connectivity display:** Feasible. `app.go` already handles all three connectivity states via `mapConnectivityState()` and routes them to `mode.BackendState`. The proposal just enriches the existing handling.

#### Testing Strategy

- Strategy is complete and realistic.
- Correctly identifies that Dolt behavior can't be unit-tested (test infra uses in-memory SQLite).
- Manual integration testing checklist covers the key scenarios.
- Proposal to extend existing test files (`dolt_client_test.go`, `connectivity_errors_test.go`) rather than create parallel test suites is the right approach.

#### Risks Assessment

All five identified risks are genuine and mitigations are reasonable. No additional high-priority risks identified.

### Minor Notes (non-blocking)

1. **File path typo:** `testutil/db.go` should be `internal/testutil/db.go` — fix during implementation.
2. **Line number precision:** A few line references are off by 1-2 lines (text_search.go, app.go). This is cosmetic and doesn't affect the implementation plan.
3. **Omitted detail in error classification description:** The `net.OpError` branch also checks `syscall.ECONNREFUSED`, `syscall.ECONNRESET`, and `syscall.EPIPE` — worth noting when expanding error classification to avoid accidentally removing these checks.

**Ready for task breakdown.**

## Task Review: APPROVED

**Reviewer:** Task Reviewer (worker-5)
**Epic:** perles-hj0
**Tasks:** 7 tasks (6 implementation + 1 acceptance review)

### Review Summary

| Criterion | Verdict |
|-----------|---------|
| Task clarity | ✅ Pass |
| Tests included | ✅ Pass |
| Acceptance criteria | ✅ Pass |
| Dependencies | ✅ Correct |
| Proposal alignment | ✅ Pass |

### Proposal Coverage

All 6 implementation steps from the proposal are covered 1:1 by tasks:

| Proposal Step | Task ID | Title |
|---|---|---|
| Step 1 | perles-hj0.1 | Add configurable retry policy with exponential backoff |
| Step 2 | perles-hj0.2 | Add proactive health monitor |
| Step 3 | perles-hj0.3 | Expand error classification for v0.60 failure modes |
| Step 4 | perles-hj0.4 | Add connection diagnostic context |
| Step 5 | perles-hj0.5 | Improve startup resilience for port resolution race |
| Step 6 | perles-hj0.6 | Surface connectivity status in TUI |
| Acceptance | perles-hj0.7 | Acceptance Review: Harden Dolt Connectivity |

No proposal steps are missing. No extra tasks were added that aren't in the proposal.

### Dependency Graph

```
Track A:  hj0.1 (Retry Policy) → hj0.2 (Health Monitor) → hj0.6 (TUI)
Track B:  hj0.3 (Error Classification) → hj0.4 (Diagnostics) → hj0.6 (TUI)
Track C:  hj0.1 (Retry Policy) → hj0.5 (Startup Resilience)
Gate:     All 6 → hj0.7 (Acceptance Review)
```

- Two independent starting points (hj0.1, hj0.3) allow parallel execution
- Dependencies are correct: hj0.5 needs RetryPolicy from hj0.1; hj0.4 needs error classifiers from hj0.3; hj0.6 needs both health monitor and diagnostics
- Acceptance review correctly blocks on all 6 implementation tasks

### Per-Task Review

#### perles-hj0.1 — Add configurable retry policy with exponential backoff
- **Scope:** ✅ Clear — create RetryPolicy struct, two defaults, Execute method; update executor.go and text_search.go
- **Tests:** ✅ Included — retry_policy_test.go with 7 specific test scenarios (backoff, jitter, max attempts, non-recoverable early exit, success, context cancellation, table-driven)
- **Acceptance criteria:** ✅ 11 checkboxes, all testable and specific
- **Alignment:** ✅ Matches proposal Step 1 exactly
- **File paths:** ✅ All 4 files verified to exist (2 create, 2 update)

#### perles-hj0.2 — Add proactive health monitor
- **Scope:** ✅ Clear — create HealthMonitor, wire into DoltClient start/stop lifecycle
- **Tests:** ✅ Included — health_monitor_test.go with 5 test scenarios; dolt_client_test.go extended
- **Acceptance criteria:** ✅ 9 checkboxes, all testable
- **Alignment:** ✅ Matches proposal Step 2 exactly
- **Dependency on hj0.1:** ✅ Correct — health monitor may leverage retry patterns

#### perles-hj0.3 — Expand error classification for v0.60 failure modes
- **Scope:** ✅ Clear — add 3 new classification functions, extend IsRecoverableConnectivityError
- **Tests:** ✅ Included — test tables for each new function plus regression on existing tests
- **Acceptance criteria:** ✅ 9 checkboxes, includes false-positive testing
- **Alignment:** ✅ Matches proposal Step 3 exactly
- **Note:** ✅ Correctly includes the reviewer's note about preserving `net.OpError` branch checks (ECONNREFUSED, ECONNRESET, EPIPE) as an explicit criterion

#### perles-hj0.4 — Add connection diagnostic context
- **Scope:** ✅ Clear — DiagnosticContext struct with 8 fields, method on DoltClient, 4 failure-mode suggestions
- **Tests:** ✅ Included — 3 test scenarios in dolt_client_test.go
- **Acceptance criteria:** ✅ 9 checkboxes, includes cross-task dependency note (TUI in Step 6)
- **Alignment:** ✅ Matches proposal Step 4 exactly
- **Dependency on hj0.3:** ✅ Correct — diagnostics use error classification for suggestion generation

#### perles-hj0.5 — Improve startup resilience for port resolution race
- **Scope:** ✅ Clear — re-resolve on each retry, use StartupRetryPolicy, port file polling, logging
- **Tests:** ✅ Included — 4 test scenarios covering re-resolution and port file race
- **Acceptance criteria:** ✅ 8 checkboxes, all testable
- **Alignment:** ✅ Matches proposal Step 5 exactly
- **Dependency on hj0.1:** ✅ Correct — uses `DefaultStartupRetryPolicy()` from hj0.1

#### perles-hj0.6 — Surface connectivity status in TUI
- **Scope:** ✅ Clear — enhance 3 connectivity states in app.go, add debounce, include diagnostics
- **Tests:** ✅ Included — references existing test suite, `make build-go` as verification
- **Acceptance criteria:** ✅ 10 checkboxes, includes debounce timing and regression check
- **Alignment:** ✅ Matches proposal Step 6 exactly
- **Dependencies on hj0.2 and hj0.4:** ✅ Correct — TUI needs health monitor events and diagnostic context

#### perles-hj0.7 — Acceptance Review: Harden Dolt Connectivity
- **Scope:** ✅ Clear — final verification of all 6 implementation tasks
- **Acceptance criteria:** ✅ 12 checkboxes covering all proposal acceptance criteria plus build/dead-code/regression checks
- **Dependencies:** ✅ Correctly blocks on all 6 implementation tasks
- **Owner:** ✅ Assigned to verifier role

### Minor Observations (non-blocking)

1. **perles-hj0.3 dead code risk:** The new `IsProjectMismatchError`, `IsCircuitBreakerError`, and `IsDoltPanicError` functions won't have production callers until hj0.4 (diagnostic context) uses them for suggestion generation. The "no dead code" criterion on hj0.3 may technically fail if checked in isolation — the implementer should note that hj0.4 will be the production consumer, or the functions should be called within the existing error handling path.

2. **perles-hj0.6 test coverage:** The TUI task's test criterion says "if applicable" for `go test ./internal/app/...`. Since TUI behavior is hard to unit test (Bubble Tea model), the primary verification is `make build-go` and manual integration testing. This is acceptable given the proposal's own testing strategy section acknowledges manual integration testing for TUI behavior.

3. **Task sizing:** All tasks appear achievable in a single implementation session. The largest are hj0.1 (4 files) and hj0.6 (complex TUI logic), but both have clear instructions.

**Ready for implementation.**

## Quick Plan Complete

### Proposal
docs/proposals/2026-03-14-quick-plan-proposal.md

### Epic Created
**ID:** perles-hj0
**Title:** Harden Dolt Connectivity Against v0.60 Upstream Issues

### Tasks (7)

| ID | Title | Status |
|----|-------|--------|
| perles-hj0.1 | Add configurable retry policy with exponential backoff | Ready |
| perles-hj0.2 | Add proactive health monitor | Ready (blocked on hj0.1) |
| perles-hj0.3 | Expand error classification for v0.60 failure modes | Ready |
| perles-hj0.4 | Add connection diagnostic context | Ready (blocked on hj0.3) |
| perles-hj0.5 | Improve startup resilience for port resolution race | Ready (blocked on hj0.1) |
| perles-hj0.6 | Surface connectivity status in TUI | Ready (blocked on hj0.2, hj0.4) |
| perles-hj0.7 | Acceptance Review: Harden Dolt Connectivity | Ready (blocked on hj0.1–hj0.6) |

### Dependencies

```
Track A:  hj0.1 (Retry Policy) → hj0.2 (Health Monitor) → hj0.6 (TUI)
Track B:  hj0.3 (Error Classification) → hj0.4 (Diagnostics) → hj0.6 (TUI)
Track C:  hj0.1 (Retry Policy) → hj0.5 (Startup Resilience)
Gate:     All 6 → hj0.7 (Acceptance Review)
```

Two parallel starting points: **hj0.1** and **hj0.3** can execute concurrently.

### Next Steps
1. Use the "cook" workflow to execute tasks
2. Or manually pick up tasks with `bd update {task-id} --status in_progress`

**Ready for implementation.**
