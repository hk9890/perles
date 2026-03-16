# Proposal: Orchestrator Ideas

## Overview

This document captures a concrete direction for a beads-native orchestration system with a Mayor-style user experience, but with a strong focus on one specific component for now: the **Execution Service**.

The intended product shape is:

- discuss requirements with an LLM
- create beads epics/tasks from that discussion
- automatically review the plan
- surface open questions in a clean summarized form
- execute the resulting beads work hands-free until done
- run tests/builds/reviews
- commit the work
- remain fully inspectable at every step

For this iteration, the most important design target is a **standalone executor program** that can take a beads issue or epic and run it to completion.

---

## Product Goals

### Desired user flow

1. Discuss feature or change with a planning LLM.
2. Approve the plan.
3. Ask the system to create beads epics/tasks.
4. Let the system auto-review the plan and summarize open questions.
5. Approve execution.
6. Let the system execute continuously until the work is done.
7. Ensure tests, review, commits, and final status updates happen automatically.
8. Allow the user to inspect or attach to any running session at any time.

### Core requirements

- beads is the work graph and durable source of truth
- the executor must be able to run independently of the planning UI
- the executor must not be a black box
- every worker/session must be observable and ideally attachable
- the system should support remote control while running

---

## Architectural Direction

The system should likely be split into separate executables rather than one giant process:

- **planner / mayor**: discussion, plan generation, review, open-question summaries
- **executor**: takes a beads issue or epic and executes it until done
- **dashboard / inspector**: visibility, logs, attach, session inspection
- **runtime adapter(s)**: integration with OpenCode or other agent runtimes

This separation is useful because:

- the executor can run headless in CI, tmux, or background mode
- planning and execution can evolve independently
- failures in the UI layer should not kill active execution
- remote control becomes easier when the executor exposes a stable interface

---

## Focus Component: Execution Service

## Purpose

The Execution Service is a standalone program whose job is:

- take a beads task or epic ID
- inspect the work graph
- execute all ready work
- keep going until done or genuinely blocked
- run tests/builds/reviews
- commit the changes
- update beads as reality changes

In other words, this is the component that turns:

> "execute this beads issue/epic"

into:

> "the work is actively being carried out, supervised, tested, and driven to completion."

---

## Standalone Executable Model

Suggested binary name for discussion purposes:

- `perles-executor`

Example invocation:

```bash
perles-executor run perles-abc12 --mode task
perles-executor run perles-epic42 --mode epic --until-done
```

Possible behavior:

- if given a **task** ID, it executes that one task
- if given an **epic** ID, it repeatedly checks ready tasks and executes the entire epic to completion
- if new tasks become unblocked, it continues automatically
- if blocked by a real question or failure, it records that state and exposes it to the operator

---

## Responsibilities of the Executor

The executor should own:

1. **issue resolution**
   - determine whether the input is a task or epic
   - load children, dependencies, and readiness

2. **worker/session lifecycle**
   - spawn headless agent sessions
   - assign tasks
   - monitor activity and heartbeats
   - replace or nudge stuck workers

3. **continuous execution loop**
   - poll `bd ready`
   - assign ready tasks
   - check for newly unblocked tasks
   - continue until completion

4. **quality gates**
   - run required tests
   - run builds/lint if configured
   - capture evidence

5. **review and verification**
   - trigger code review pass
   - trigger acceptance verification for epics

6. **beads state synchronization**
   - mark tasks in progress
   - add comments with summaries and failures
   - close completed tasks
   - create follow-up bug tasks if needed

7. **operator visibility**
   - expose run status
   - expose worker status
   - expose transcripts/logs
   - allow attach/inspect

---

## Recommended Execution Model

The execution loop could look like this:

1. Start a run for a beads ID.
2. Resolve it to either:
   - single-task execution, or
   - epic execution loop.
3. Load ready tasks from beads.
4. Spawn or reuse worker sessions.
5. Send task instructions to workers.
6. Monitor for:
   - implementation completion
   - test results
   - review verdicts
   - stalls / errors / blockers
7. Update beads.
8. Re-check ready tasks.
9. Repeat until:
   - all work is done, or
   - human input is required.

The executor should stop only when one of these is true:

- the target task is complete
- the target epic and its acceptance review are complete
- the run is explicitly paused/stopped
- the run is blocked on a real human question

---

## OpenCode Runtime Options

The execution layer should not hard-code itself to one specific runtime model. It should use a runtime adapter, but the first target can be OpenCode.

There are at least three possible ways to integrate OpenCode:

### Option A: Long-lived headless OpenCode session (preferred)

If OpenCode can be started in a server/headless/session mode, this is the best fit.

Benefits:

- preserves session continuity
- allows sending follow-up instructions
- allows fetching latest messages
- supports remote control naturally
- better for long-running tasks and review loops

Needed capabilities:

- start session
- send message
- read transcript / latest response
- interrupt or stop
- inspect status
- possibly attach to raw terminal session

### Option B: PTY/tmux wrapped interactive session

If OpenCode does not expose a proper headless control API, the executor can still launch it in a PTY or tmux session and control it indirectly.

Benefits:

- immediately inspectable
- easy to attach to manually
- easy to record transcripts
- works even without a formal server API

Drawbacks:

- command/message injection is less clean
- parsing latest output can be brittle
- richer state queries may be harder

### Option C: `opencode run` one-shot jobs

This is the simplest model if OpenCode has a `run` command for non-interactive execution.

Benefits:

- easy to automate
- deterministic process boundaries
- good for bounded phases

Drawbacks:

- poor session continuity
- weaker interactivity
- weaker live inspection unless the wrapper adds it
- harder to "push the agent until done" in the same ongoing session

### Recommendation

For the system described in this proposal:

- use **long-lived headless sessions** if OpenCode supports them
- otherwise use **tmux/PTY wrapped sessions** as the main runtime model
- reserve **`opencode run`** as a fallback or for narrow sub-steps

The reason is simple: the desired system is not just batch automation; it is an actively supervised, inspectable execution engine.

---

## Communication Interface Suggestions

The executor should expose a clean control interface so it can be driven by:

- the Mayor/planner
- a CLI
- a web dashboard
- manual scripts
- future remote control tools

### Recommendation: Hybrid interface

Use three layers:

1. **local control API** for commands
2. **event stream** for live updates
3. **attach interface** for direct human inspection

### 1. Local control API

Best initial choice:

- **HTTP + JSON over Unix domain socket**

Why:

- easy to use from CLI and web UI
- easy to debug with curl-like tooling
- safer than opening a TCP port by default
- good enough for local orchestration and remote forwarding later

Alternative:

- JSON-RPC over Unix socket

This is also good, especially if the command surface becomes very method-oriented.

### 2. Event stream

Use one of:

- **Server-Sent Events (SSE)**
- **WebSocket**
- newline-delimited JSON event tail

This is used for:

- worker spawned
- task assigned
- message received
- tests started/passed/failed
- review approved/denied
- blocker raised
- run completed

### 3. Attach interface

This should be explicit.

Examples:

- tmux session name
- PTY websocket endpoint
- `perles-executor attach <worker-id>` wrapper

This is the anti-black-box layer.

---

## Suggested Control API Shape

### Run lifecycle

```http
POST   /runs
GET    /runs/{runId}
POST   /runs/{runId}/pause
POST   /runs/{runId}/resume
POST   /runs/{runId}/stop
GET    /runs/{runId}/events
```

Example create-run request:

```json
{
  "issue_id": "perles-epic42",
  "mode": "epic",
  "repo_path": "/path/to/repo",
  "runtime": "opencode",
  "until_done": true
}
```

### Worker/session control

```http
GET    /workers
GET    /workers/{workerId}
POST   /workers/{workerId}/message
POST   /workers/{workerId}/interrupt
POST   /workers/{workerId}/nudge
GET    /workers/{workerId}/transcript
GET    /workers/{workerId}/attach
```

Example send-message request:

```json
{
  "message": "Focus on fixing the failing test first, then continue with the task."
}
```

### Issue-oriented view

```http
GET    /issues/{issueId}/status
GET    /issues/{issueId}/evidence
GET    /issues/{issueId}/workers
POST   /issues/{issueId}/replan
```

This makes it easy for higher-level tools to ask:

- what is happening on this issue?
- which worker is attached to it?
- what is the latest evidence?

---

## Suggested Event Model

The event stream should be structured and append-only.

Example event types:

- `run.started`
- `run.paused`
- `run.completed`
- `run.blocked`
- `issue.loaded`
- `task.assigned`
- `worker.spawned`
- `worker.heartbeat`
- `worker.message`
- `worker.stalled`
- `test.started`
- `test.passed`
- `test.failed`
- `review.approved`
- `review.denied`
- `beads.status_updated`
- `beads.comment_added`

Example event payload:

```json
{
  "type": "task.assigned",
  "timestamp": "2026-03-15T13:30:00Z",
  "run_id": "run-123",
  "issue_id": "perles-abc12",
  "worker_id": "worker-2",
  "summary": "Implement and test task in isolated worktree"
}
```

This stream is ideal for:

- CLI status views
- dashboard timelines
- audit logs
- debugging stalled runs

---

## Suggested Runtime Adapter Interface

The executor should talk to a runtime adapter rather than directly to OpenCode internals.

Illustrative Go interface:

```go
type RuntimeAdapter interface {
    Spawn(ctx context.Context, spec SessionSpec) (*SessionHandle, error)
    SendMessage(ctx context.Context, sessionID string, msg string) error
    GetLatestMessage(ctx context.Context, sessionID string) (*Message, error)
    GetTranscript(ctx context.Context, sessionID string, since int64) ([]Message, error)
    Interrupt(ctx context.Context, sessionID string) error
    Stop(ctx context.Context, sessionID string) error
    AttachInfo(ctx context.Context, sessionID string) (*AttachInfo, error)
    Snapshot(ctx context.Context, sessionID string) (*SessionSnapshot, error)
}
```

This keeps the executor stable even if the underlying runtime changes from:

- OpenCode server
- OpenCode tmux wrapper
- `opencode run`
- another future runtime

---

## Suggested CLI on Top of the Control API

The executor should expose both API and CLI.

Example CLI:

```bash
perles-executor run perles-epic42 --mode epic
perles-executor status run-123
perles-executor events run-123 --follow
perles-executor message worker-2 "Please summarize blockers"
perles-executor attach worker-2
perles-executor transcript worker-2
perles-executor pause run-123
perles-executor resume run-123
```

The CLI can be thin wrappers over the local control API.

---

## Recommended State Model for Executor Runs

### Run states

- `created`
- `resolving_issue`
- `running`
- `paused`
- `blocked_on_human`
- `failed`
- `completed`

### Worker states

- `starting`
- `idle`
- `implementing`
- `testing`
- `reviewing`
- `waiting`
- `stalled`
- `stopped`

### Blocker kinds

- `human_question`
- `test_failure`
- `runtime_failure`
- `review_denied`
- `beads_sync_error`
- `merge_or_git_error`

This helps the operator understand what kind of intervention is needed.

---

## Inspectability Requirements

The executor must not be a black box.

For every active worker/session, the operator should be able to access:

- worker ID
- current task / issue ID
- current state
- latest message
- full transcript
- worktree path
- branch name
- attach target
- last heartbeat
- last test result
- diff summary

Minimum viable inspectability could be:

- tmux session per worker
- transcript file per worker
- `status` and `events --follow` commands
- `attach` command

---

## Recommendation for v1

If the goal is to ship something practical quickly, the best first cut is:

### Executor shape

- standalone binary: `perles-executor`
- input: beads task or epic ID
- local-only runtime initially
- one SQLite runtime database for run/session metadata

### Runtime shape

- OpenCode launched in **tmux-based headless sessions**
- one git worktree per worker
- transcript captured to file and event store

### Control shape

- HTTP/JSON over Unix domain socket
- SSE event stream
- CLI wrappers
- explicit attach command

### Why this is a good v1

- avoids needing a full distributed control plane immediately
- keeps the runtime inspectable
- supports remote control later by forwarding the local socket
- does not depend on undocumented internal OpenCode APIs
- can later upgrade from tmux wrapper to native headless OpenCode if available

---

## Open Questions

These are the main questions still worth resolving before implementation:

1. **Does OpenCode expose a stable headless session API already?**
2. **Can an OpenCode session be resumed and messaged reliably, or do we need a PTY wrapper?**
3. **Should commits happen per task, per worker, or per epic phase?**
4. **Should the executor itself perform reviews, or should review be delegated to separate worker sessions?**
5. **What is the minimum acceptable attach mechanism for v1: tmux attach, transcript only, or full PTY relay?**
6. **How much of the control API should be synchronous versus event-driven?**

---

## Summary

The strongest direction is to make the **Execution Service** a standalone headless program that can take a beads issue or epic and execute it continuously until done, while exposing a clear control and inspection interface.

The recommended communication model is:

- **control API** over local HTTP/JSON on a Unix socket
- **event stream** via SSE or structured log tail
- **direct attach** through tmux/PTY-backed worker sessions

The recommended runtime model is:

- native long-lived OpenCode headless sessions if available
- otherwise tmux/PTY wrapped OpenCode sessions
- use `opencode run` only as a fallback or narrow batch primitive

This gives the system the most important properties:

- beads-native execution
- hands-free automation
- strong operator control
- inspectability instead of black-box behavior
