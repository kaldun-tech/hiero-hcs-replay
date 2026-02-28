# Code Review Checklist

Use this checklist when reviewing code for hiero-hcs-replay.

## API Design

- [ ] Are exported names clear and follow Go conventions?
  - Reference: [Effective Go - Names](https://go.dev/doc/effective_go#names)
- [ ] Is the package name short, lowercase, and descriptive?
  - Reference: [Go Blog - Package Names](https://go.dev/blog/package-names)
- [ ] Are zero values useful? (e.g., `Stats{}`, `FetchOptions{}`)
- [ ] Do functions return errors rather than panic for recoverable conditions?
- [ ] Are option structs used for functions with many parameters?

## Concurrency Safety

- [ ] Are thread safety claims accurate in documentation?
- [ ] Is `sync.Mutex` or `sync.RWMutex` used where state is shared?
- [ ] Is `rand.Rand` protected by mutex? (It's not safe for concurrent use)
  - Reference: [pkg.go.dev/math/rand#Rand](https://pkg.go.dev/math/rand#Rand)
- [ ] Are channels used correctly with proper closing semantics?

## Error Handling

- [ ] Are errors wrapped with `fmt.Errorf("context: %w", err)`?
  - Reference: [Go Blog - Working with Errors](https://go.dev/blog/go1.13-errors)
- [ ] Are sentinel errors defined for conditions callers need to check?
- [ ] Is error handling consistent (don't mix panics and errors)?
- [ ] Are resources cleaned up on error paths? (files, connections)

## Documentation

- [ ] Do all exported types, functions, and constants have godoc comments?
  - Reference: [Go Doc Comments](https://go.dev/doc/comment)
- [ ] Does the package have a doc comment explaining its purpose?
- [ ] Are examples in `example_test.go` runnable and accurate?
- [ ] Are complex algorithms or non-obvious code explained?

## Testing

- [ ] Is test coverage sufficient for critical paths?
- [ ] Are edge cases tested? (empty input, nil, zero values, boundaries)
- [ ] Are tests deterministic? (avoid time-dependent or random failures)
- [ ] Do tests use `t.Helper()` for test helper functions?
- [ ] Are table-driven tests used where appropriate?
  - Reference: [Go Wiki - Table Driven Tests](https://github.com/golang/go/wiki/TableDrivenTests)

## Security

- [ ] Is user input validated before use in URLs, file paths, or commands?
- [ ] Are HTTP client timeouts configured?
- [ ] Is sensitive data (credentials, tokens) never logged or exposed?
- [ ] Are cryptographic functions used correctly? (not applicable here)

## Performance

- [ ] Are allocations minimized in hot paths?
- [ ] Is `sync.Pool` used for frequently allocated objects?
- [ ] Are slices pre-allocated when size is known? (`make([]T, 0, n)`)
- [ ] Is the mutex held for minimal duration?

## Style & Idioms

- [ ] Code formatted with `gofmt`?
- [ ] No unused imports or variables?
- [ ] Short variable names for short scopes, descriptive for longer?
- [ ] Receiver names consistent and short (not `this` or `self`)?
- [ ] Early returns used to reduce nesting?

## References

- [Effective Go](https://go.dev/doc/effective_go) - Official Go style guide
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments) - Common review feedback
- [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md) - Industry best practices
- [Google Go Style Guide](https://google.github.io/styleguide/go/) - Google's internal guide (public)
- [Go Proverbs](https://go-proverbs.github.io/) - Design philosophy
