# AGENTS.md — Team-Mode Router

Use this file as a routing index. Keep detailed procedures in tracked docs under `docs/`.

## Project Overview

Perles is a Go TUI for beads issue tracking, search/kanban workflows, and multi-agent orchestration.

- Primary stack: Go, Bubble Tea, beads SQLite project data
- Core expectation: execute beads-directed work (`bd ...`) and keep task state current
- Core completion rule: work is not done until changes are pushed and branch is up to date

## Coding

- Read `docs/CODING.md` for build commands, repository structure, and coding conventions.
- Read `CONTRIBUTING.md` for contribution workflow and standards.
- Load **fix-documentation** skill when updating `README.md`, `CONTRIBUTING.md`, or `AGENTS.md`.

## Testing

- Read `docs/TESTING.md` for test strategy, required suites, and golden-test update rules.

## Releases

- Read `docs/RELEASING.md` for versioning, release steps, and verification expectations.

## Monitoring

- Read `docs/MONITORING.md` for debug logging, session artifacts, and operational health checks.
- Read `docs/CONTROL_PLANE.md` for multi-workflow/control-plane architecture and runtime behavior.

## Pull Requests

- Read `docs/PULL-REQUESTS.md` for branch strategy, PR format, and review expectations.
- Load **github-task-sync** skill when syncing beads tasks with GitHub issues/PRs.

## Architecture

- Read `docs/ARCHITECTURE.md` for system design, domain boundaries, and orchestration internals.

## Configuration

- Read `docs/CONFIGURATION.md` for config file schema, defaults, environment variables, and overrides.

## Landing the Plane (Session Completion)

When ending a session, complete all of the following:

1. Track follow-up work in beads (create/update/close tasks and bugs as appropriate).
2. Run quality gates for changed code (tests, lint, build per routed docs).
3. Update task status so beads reflects actual completion state.
4. Push to remote:
   ```bash
   git pull --rebase
   git push
   git status  # must show up to date with origin
   ```
5. Verify final state: changes committed, pushed, and task workflow updated.

**Critical:** Session completion requires a successful `git push`.
