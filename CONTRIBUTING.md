# Contributing to cmus-lyric

Thanks for your interest in contributing! This guide will help you get started.

## Development Setup

### Prerequisites

- [Go](https://go.dev/) 1.26+
- [Task](https://taskfile.dev/) (task runner)
- [golangci-lint](https://golangci-lint.run/)
- [lefthook](https://github.com/evilmartians/lefthook) (git hooks)
- A running [cmus](https://cmus.github.io/) instance for testing

### Getting Started

```bash
git clone https://github.com/index-null/cmus-lyric.git
cd cmus-lyric

# Install git hooks
lefthook install

# Build
task build

# Run
task run
```

### Available Tasks

```bash
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

- Run `gofmt` (enforced by lefthook pre-commit hook)
- Follow [Effective Go](https://go.dev/doc/effective_go) guidelines
- Keep functions small and well-named
- Avoid unnecessary comments — clear code is better than commented code

## Project Structure

```
cmus-lyric/
├── cmd/lyrics/       # Entry point
├── internal/
│   ├── player/       # TUI model, cmus IPC, rendering
│   └── lyric/        # Lyric fetching (LRCLIB, Netease)
├── Taskfile.yml      # Build tasks
└── go.mod
```

## Adding a New Lyrics Source

1. Create a new file in `internal/lyric/` (e.g. `kugou.go`)
2. Implement the fetching logic following existing patterns in the package
3. Integrate the new source into the fallback chain
4. Add tests

## Reporting Issues

- Use the [Bug Report](.github/ISSUE_TEMPLATE/bug_report.md) template for bugs
- Use the [Feature Request](.github/ISSUE_TEMPLATE/feature_request.md) template for ideas
- Include your OS, Go version, and cmus version when reporting bugs

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
