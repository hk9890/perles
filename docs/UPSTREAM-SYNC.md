# Upstream Sync Guide

This document defines how this fork evaluates and imports changes from `zjrosen/perles`.

## Purpose

- Keep the fork aligned with upstream where it improves compatibility, stability, UI quality, and maintainability.
- Prefer selective imports over blind upstream merges.
- Preserve this fork's product scope while still benefiting from upstream fixes.

## Support Policy

### In Scope for Upstream Sync

- Stability fixes and regression fixes
- UI and UX fixes
- Support for new beads versions
- Dolt support
- Third-party dependency updates
- Go toolchain updates

### Out of Scope for Upstream Sync

- `beads_rust` backend support
- SQLite support or reintroduction of SQLite-specific code paths
- CLI or orchestration providers beyond the approved set

### Approved Provider Scope

- Primary: OpenCode
- P1: Claude
- P1: Copilot

Do not import support for other providers unless explicitly approved first.

## Upstream Sync Rules

1. **Discuss before import**
   - Always discuss the commits or features we want to bring in before cherry-picking, porting, or reimplementing them.
   - Do not import upstream changes just because they landed on `main`.

2. **Analyze regressions first**
   - Always do impact analysis before importing.
   - Review backend implications, provider implications, config changes, build/test changes, migration risk, and visible UI behavior.
   - If a change could cause regressions, document the risk and discuss it before proceeding.

3. **Import cautiously**
   - Prefer small, focused sync batches.
   - If an upstream commit mixes wanted and unwanted behavior, port only the accepted parts manually instead of taking the whole commit.
   - Never do a wholesale upstream merge without prior review.

4. **Stop and discuss when issues appear**
   - If a sync candidate has hidden scope, conflicts with fork policy, depends on rejected features, or creates unclear behavior, stop and discuss before continuing.
   - Be especially careful with backend, provider, and config changes.

5. **Preserve fork policy during conflict resolution**
   - Do not reintroduce `beads_rust`, SQLite, or unsupported providers while resolving merge or porting conflicts.
   - Keep new backend work aligned with current beads versions and Dolt support only.

6. **Validate every imported batch**
   - Run the relevant build, test, and lint commands after each import batch.
   - Re-check visible UI behavior when importing UI fixes.
   - Confirm supported providers still behave correctly.

## Commit Triage Heuristics

### Usually Accept

- Stability fixes
- UI fixes
- New beads compatibility work
- Dolt support improvements
- Dependency updates
- Go version bumps

### Usually Reject

- `beads_rust` support
- SQLite support
- New provider support outside OpenCode, Claude, and Copilot

### Always Review Carefully

- Large refactors
- Multi-feature commits
- Config removals or defaults changes
- Workflow or orchestration changes
- Commits that mix backend changes with unrelated product behavior

## Default Sync Process

1. Collect candidate upstream commits.
2. Classify each item as **accept**, **reject**, or **needs discussion**.
3. Discuss the accepted candidates before implementation.
4. Perform regression analysis.
5. Import in small batches.
6. Validate the result and discuss any issues before taking the next batch.

## Decision Standard

If a change is useful but conflicts with this fork's scope, we do **not** take it wholesale. We either:

- reject it,
- defer it for discussion, or
- port only the parts that fit this fork.
