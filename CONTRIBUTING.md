# Contributing to cmus-lyric

Thanks for your interest in contributing! This guide will help you get started.

## Development Setup

### Prerequisites

- [Go](https://go.dev/) 1.26+
- [Task](https://taskfile.dev/) (task runner)
- [golangci-lint](https://golangci-lint.run/) v2.x
- [lefthook](https://github.com/evilmartians/lefthook) (git hooks, managed via Go tool)
- A running [cmus](https://cmus.github.io/) instance for testing

### Getting Started

```bash
git clone https://github.com/index-null/cmus-lyric.git
cd cmus-lyric

# One-time setup: install git hooks & verify toolchain
task setup

# Build
task build

# Run
task run
```

> [!IMPORTANT]
> **`task setup` installs git hooks (pre-commit + pre-push).** Without this step, quality checks won't run locally and you'll only find issues in CI. It's a one-time operation — hooks persist after `git clone`.

### Available Tasks

```bash
task setup    # Install git hooks & verify toolchain
task build    # Build binary to bin/
task run      # Build and run
task lint     # Run golangci-lint
task test     # Run tests
task check    # Full quality check (tidy + lint + test)
task clean    # Remove build artifacts
```

## Making Changes

1. Fork the repository and create a feature branch from `master`:
   ```bash
   git checkout -b feat/your-feature
   ```

2. Make your changes. Keep commits focused and atomic.

3. Ensure all checks pass before pushing:
   ```bash
   task check
   ```

4. Push and open a Pull Request against `master`.

## Quality Gates

| Gate | When | What |
|------|------|------|
| **pre-commit** (local) | `git commit` | `golangci-lint fmt` + `lint` + `go build` |
| **pre-push** (local) | `git push` | `golangci-lint` + `go test -race -cover` |
| **CI** (GitHub) | PR & push to master | lint + test + build (parallel, must all pass) |

> [!NOTE]
> CI uses the **same** checks as local hooks, with pinned `golangci-lint v2` to avoid version drift. If hooks pass locally, CI should pass too. CI is the final gate — PRs cannot merge unless all checks are green.

## Branch Naming

| Prefix    | Purpose          |
| --------- | ---------------- |
| `feat/`   | New feature      |
| `fix/`    | Bug fix          |
| `refactor/` | Code refactor  |
| `docs/`   | Documentation    |
| `chore/`  | Maintenance      |

## Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add Kugou lyrics source
fix: handle empty cmus-remote output
docs: update installation instructions
refactor: extract LRC parser into separate package
```

## Code Style

- Formatting is enforced by `golangci-lint fmt` via lefthook pre-commit hook
- Follow [Effective Go](https://go.dev/doc/effective_go) guidelines
- Keep functions small and well-named
- Avoid unnecessary comments — clear code is better than commented code
- Wrap errors with `%w` for proper error chains

## Project Structure

```
cmus-lyric/
├── cmd/lyrics/       # Entry point
├── internal/
│   ├── cmus/         # cmus IPC (Unix socket + exec fallback)
│   ├── lyric/        # Lyric loading, parsing, fetching, caching
│   └── player/       # TUI model, view, styles
├── Taskfile.yml      # Build tasks
└── go.mod
```

## Adding a New Lyrics Source

1. Create a new file in `internal/lyric/` (e.g. `kugou.go`)
2. Implement the fetching logic following existing patterns — return `(lyric, tlyric, error)`
3. Integrate the new source into the fallback chain in `fetchFromLrcLib` / `FetchContent`
4. Add unit tests with `httptest.NewServer` mocks (see `fetch_test.go` for examples)
5. Run `task check` to ensure all lints and tests pass

## Reporting Issues

- Use the [Bug Report](.github/ISSUE_TEMPLATE/bug_report.md) template for bugs
- Use the [Feature Request](.github/ISSUE_TEMPLATE/feature_request.md) template for ideas
- Include your OS, Go version, and cmus version when reporting bugs

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
