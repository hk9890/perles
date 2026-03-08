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

## Reminder

- Always run `make build-go` before telling the user work is done or ready to test.
