# Pull Requests Guide

Canonical PR workflow guidance for Perles team mode.

## Branching

- Branch from `main`
- Use descriptive branch names (`feat/...`, `fix/...`, `docs/...`)
- Keep each PR focused on one change set

## Local Validation Before Opening PR

```bash
make build
make test
make lint
```

If `frontend/` changed:

```bash
npm run build
npm run test:run
```

If UI snapshots changed, run `make test-update` and review diffs.

## PR Content Expectations

- Explain what changed and why
- Link related issues/tasks
- Include testing notes reviewers can follow
- Include screenshots for visible UI changes
- Update docs when behavior/workflows/config changed

## Template

Use `.github/PULL_REQUEST_TEMPLATE.md` as the required checklist.

The template expects:

- change type
- confirmation of tests/docs/golden updates
- screenshots (if applicable)
- reviewer-oriented test notes

## Review & Merge Readiness

- CI green across supported platforms
- New behavior covered by tests
- Docs updated where needed
- Commit history and PR description are clear

## Team-Mode Authority Check for PRs

When PRs touch workflow/process guidance, verify that team-mode sources remain authoritative:

- Confirm guidance is captured in tracked docs (`AGENTS.md`, `docs/*`), not only in `.coder/*` local files.
- If local stealth exclusions in `.git/info/exclude` hide `.coder/*` or related files, ensure no required team instruction depends on those hidden assets.
- Keep `AGENTS.md` concise as a router and move detailed procedures to dedicated docs in `docs/`.

## Related Docs

- [TESTING.md](./TESTING.md)
- [CODING.md](./CODING.md)
- [CONTRIBUTING.md](../CONTRIBUTING.md)
