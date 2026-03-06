# Contributing to BlackCat

Thank you for your interest in contributing! This guide walks you through setup, development, and submission.

## Prerequisites

- Go 1.25 or later
- `CGO_ENABLED=1` (required for WhatsApp integration)
- Git

## Setup

```bash
git clone https://github.com/StarTower/interstellar.git
cd interstellar
go build -o blackcat ./cmd/blackcat
./blackcat configure
```

## Development Workflow

1. Create a feature branch: `git checkout -b feature/your-feature`
2. Make changes and test locally
3. Follow commit conventions: `feat:`, `fix:`, `docs:`, `refactor:`, `test:`
4. Push to your fork and submit a PR against `master`

## Testing

Run all tests:
```bash
go test ./...
```

Note: Integration tests depend on Redis. Run full suite only if Redis is available.

## Submitting PRs

- Fork the repository
- Create a feature branch from `master`
- Ensure all tests pass
- Provide a clear PR description
- PRs require passing tests before merge

## Code Style

- Use `gofmt` for formatting
- Run `go vet` for linting
- Follow idiomatic Go conventions
- Keep functions focused and well-documented

