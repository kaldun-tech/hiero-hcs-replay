# Code Review Checklist

Use this checklist when reviewing code for hiero-hcs-replay.

## API Design

- [x] Are exported names clear and follow Go conventions?
  - Reference: [Effective Go - Names](https://go.dev/doc/effective_go#names)
  - `TimingData`, `Stats`, `Replay`, `FetchOptions`, `ReplayMode`, `Network` all follow conventions
- [x] Is the package name short, lowercase, and descriptive?
  - Reference: [Go Blog - Package Names](https://go.dev/blog/package-names)
  - `hcsreplay` - short, lowercase, descriptive
- [x] Are zero values useful? (e.g., `Stats{}`, `FetchOptions{}`)
  - `FetchOptions{}` applies sensible defaults (100ms delay, http.DefaultClient)
- [x] Do functions return errors rather than panic for recoverable conditions?
  - Panics only for programmer errors (nil data, invalid params) - documented in DESIGN.md
- [x] Are option structs used for functions with many parameters?
  - `FetchOptions` for `FetchTimingWithOptions`

## Concurrency Safety

- [x] Are thread safety claims accurate in documentation?
  - `Replay` documented as "safe for concurrent use from multiple goroutines"
- [x] Is `sync.Mutex` or `sync.RWMutex` used where state is shared?
  - `Replay.mu` protects index and rng
- [x] Is `rand.Rand` protected by mutex? (It's not safe for concurrent use)
  - Reference: [pkg.go.dev/math/rand#Rand](https://pkg.go.dev/math/rand#Rand)
  - Yes, protected in `NextDelay()`
- N/A Are channels used correctly with proper closing semantics?
  - No channels used in this library

## Error Handling

- [x] Are errors wrapped with `fmt.Errorf("context: %w", err)`?
  - Reference: [Go Blog - Working with Errors](https://go.dev/blog/go1.13-errors)
  - Yes, e.g., `fmt.Errorf("failed to fetch messages: %w", err)`
- [x] Are sentinel errors defined for conditions callers need to check?
  - `ErrTopicNotFound`, `ErrNotEnoughMessages`, `ErrInvalidTopicID` defined
- [x] Is error handling consistent (don't mix panics and errors)?
  - Consistent policy: panics for programmer errors, errors for runtime conditions
- [x] Are resources cleaned up on error paths? (files, connections)
  - `resp.Body.Close()` called before returning errors in `fetchMessages`

## Documentation

- [x] Do all exported types, functions, and constants have godoc comments?
  - Reference: [Go Doc Comments](https://go.dev/doc/comment)
  - All exported items documented
- [x] Does the package have a doc comment explaining its purpose?
  - Yes, at top of replay.go
- [x] Are examples in `example_test.go` runnable and accurate?
  - Yes, with `// Output:` comments for verification
- [x] Are complex algorithms or non-obvious code explained?
  - Box-Muller transform for log-normal distribution is commented

## Testing

- [x] Is test coverage sufficient for critical paths?
  - FetchTiming, Replay, file I/O all covered
- [x] Are edge cases tested? (empty input, nil, zero values, boundaries)
  - Tests for: not enough messages, context cancellation, 404, limit enforcement
- [x] Are tests deterministic? (avoid time-dependent or random failures)
  - Mock HTTP servers, range checks for random values
- [x] Do tests use `t.Helper()` for test helper functions?
  - `writeTestFile`, `encodeJSONResponse`, `newTestServer`, `newTestFetchOptions` all use `t.Helper()`
- [x] Are table-driven tests used where appropriate?
  - Reference: [Go Wiki - Table Driven Tests](https://github.com/golang/go/wiki/TableDrivenTests)
  - `TestNetwork_MirrorNodeURL`, `TestParseConsensusTimestamp`

## Security

- [x] Is user input validated before use in URLs, file paths, or commands?
  - Topic IDs validated against `^\d+\.\d+\.\d+$` pattern before use
- [x] Are HTTP client timeouts configured?
  - `DefaultFetchOptions()` provides 30s timeout; users can override
- [x] Is sensitive data (credentials, tokens) never logged or exposed?
  - No sensitive data handled
- N/A Are cryptographic functions used correctly?
  - No cryptography in this library

## Performance

- [x] Are allocations minimized in hot paths?
  - `NextDelay()` does not allocate
- N/A Is `sync.Pool` used for frequently allocated objects?
  - Not needed for this use case
- [x] Are slices pre-allocated when size is known? (`make([]T, 0, n)`)
  - `interArrivals := make([]float64, len(timestamps)-1)`
- [x] Is the mutex held for minimal duration?
  - Only array lookup and index increment in critical section

## Style & Idioms

- [x] Code formatted with `gofmt`?
  - CI enforces formatting
- [x] No unused imports or variables?
  - golangci-lint checks this
- [x] Short variable names for short scopes, descriptive for longer?
  - Follows convention throughout
- [x] Receiver names consistent and short (not `this` or `self`)?
  - `r` for Replay, `n` for Network
- [x] Early returns used to reduce nesting?
  - Yes, in error handling paths

## References

- [Effective Go](https://go.dev/doc/effective_go) - Official Go style guide
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments) - Common review feedback
- [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md) - Industry best practices
- [Google Go Style Guide](https://google.github.io/styleguide/go/) - Google's internal guide (public)
- [Go Proverbs](https://go-proverbs.github.io/) - Design philosophy
