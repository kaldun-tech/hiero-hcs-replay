# Contributing to hiero-hcs-replay

Thank you for your interest in contributing to hiero-hcs-replay! This document provides guidelines for contributing to the project.

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/YOUR_USERNAME/hiero-hcs-replay.git`
3. Create a branch: `git checkout -b feature/your-feature-name`

## Development Setup

```bash
# Install dependencies
go mod download

# Run tests
go test -v ./...

# Run tests with race detection
go test -race ./...

# Run linter (requires golangci-lint)
golangci-lint run
```

## Code Style

- Follow standard Go conventions and idioms
- Run `gofmt` before committing
- All exported types and functions must have godoc comments
- Keep functions focused and small

## Testing

- All new functionality must include tests
- Maintain or improve code coverage
- Use table-driven tests where appropriate
- Mock external dependencies (HTTP, file I/O) in tests

## Pull Request Process

1. Ensure all tests pass: `go test -race ./...`
2. Run the linter: `golangci-lint run`
3. Update documentation if adding new public API
4. Write a clear PR description explaining the change
5. Reference any related issues

## Commit Messages

Use clear, descriptive commit messages:

```
feat: add support for custom mirror node URLs
fix: handle empty inter-arrival arrays
docs: update README with new API examples
test: add tests for pagination handling
```

## Reporting Issues

When reporting bugs, please include:

- Go version (`go version`)
- Operating system
- Steps to reproduce
- Expected vs actual behavior
- Any relevant error messages

## Feature Requests

Feature requests are welcome! Please open an issue describing:

- The use case
- Proposed API (if applicable)
- Any alternatives considered

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
