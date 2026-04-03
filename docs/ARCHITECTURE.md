# Architecture Guide

Canonical architecture overview for Perles team mode.

## System Overview

Perles is a Bubble Tea TUI for beads issue tracking, with integrated multi-agent orchestration.

Major domains:

- TUI application and mode controllers
- BQL parsing/execution
- Config, persistence, and filesystem watchers
- Orchestration runtime (clients, process lifecycle, control plane)

## High-Level Structure

- `cmd/` - CLI entrypoints and startup wiring (cobra)
- `internal/app` - root model and mode switching
- `internal/mode/*` - mode controllers implementing a common interface
- `internal/ui/*` - rendering components
- `internal/bql` - lexer/parser/validator/executor
- `internal/orchestration/*` - headless clients + V2 orchestration engine
- `internal/pubsub` - event broker abstraction used across orchestration/UI

## Bubble Tea Interaction Pattern

Modes follow a controller pattern (`Init`, `Update`, `View`, `SetSize`) and communicate through messages. Keep state transitions in `Update` and keep rendering pure in `View`.

## Orchestration Architecture

Key orchestration packages:

- `internal/orchestration/client` - provider-agnostic process interfaces
- provider adapters: `claude`, `amp`, `codex`, `gemini`, `opencode`
- `internal/orchestration/v2` - command handlers, repositories, lifecycle processing
- `internal/orchestration/controlplane` - multi-workflow coordination and health monitoring
- `internal/orchestration/events` - unified `ProcessEvent` model
- `internal/orchestration/session` - on-disk session logs
- `internal/orchestration/workflow` - template registry and built-ins

For user-facing orchestration behavior, see [ORCHESTRATION.md](../ORCHESTRATION.md) and [CONTROL_PLANE.md](./CONTROL_PLANE.md).

## Registry DDD Split

Perles uses DDD layering for registry logic only:

- `internal/domain/registry` - pure domain logic (no I/O)
- `internal/application/registry` - file/template loading + service facade

This separation isolates graph/validation domain rules from infrastructure concerns.

## Eventing and Pub/Sub

Orchestration uses a pub/sub broker to decouple producers (process/runtime) from consumers (TUI, logs, metrics). Subscriptions are context-bound and non-blocking publish semantics are used to avoid deadlocks.

## Data and Runtime Paths

- Beads data source: Dolt SQL server mode discovered from `.beads/metadata.json`, `.beads/dolt/config.yaml`, and `.beads/dolt-server.port` (resolved from flag/env/config/CWD)
- Orchestration sessions: `~/.perles/sessions/` by default
- Optional trace output: `~/.config/perles/traces/traces.jsonl`

## beads runtime compatibility boundary

Perles currently supports beads v1+ projects only when:

- `backend=dolt`
- `dolt_mode=server`

Embedded and shared-server Dolt modes are intentionally unsupported for now. See [BEADS-COMPATIBILITY.md](./BEADS-COMPATIBILITY.md) for support matrix and migration/repair guidance.

## Related Docs

- [CONFIGURATION.md](./CONFIGURATION.md)
- [BEADS-COMPATIBILITY.md](./BEADS-COMPATIBILITY.md)
- [BEADS-V1-SPEC.md](./BEADS-V1-SPEC.md)
- [CODING.md](./CODING.md)
- [MONITORING.md](./MONITORING.md)
- [internal/orchestration/v2/docs/README.md](../internal/orchestration/v2/docs/README.md)
