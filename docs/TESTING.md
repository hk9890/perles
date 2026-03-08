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

## Test Types in This Repo

- Unit tests: standard `testing` + `testify/require`
- Golden/snapshot tests: `teatest`
- Property-style tests (targeted areas): `rapid`

## Golden File Workflow

When UI output intentionally changes:

1. Run `make test` and review failures
2. Run `make test-update`
3. Review all golden diffs before commit

`make test-update` updates a curated package list from the `Makefile`.

## Recommended Validation Before PR

- Go-only changes: `make test && make lint`
- Frontend changes: `npm run build && npm run test:run`
- Cross-stack changes: `make build && make test && make lint`

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
