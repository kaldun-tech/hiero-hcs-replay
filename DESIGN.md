# Design Document

This document explains the architectural decisions and trade-offs in hiero-hcs-replay.

## Core Concept

The library captures **inter-arrival times** (delays between consecutive HCS messages) rather than absolute timestamps, then replays them to drive load tests with realistic timing patterns.

```
Real HCS Topic → Mirror Node API → Inter-arrival times → Replay engine → Load test
```

## Design Decisions

### 1. Library vs CLI/Service

**Chosen**: Pure library package

| Approach | Pros | Cons |
|----------|------|------|
| **Library** (chosen) | Maximum integration flexibility, no deployment overhead, composable | Users must write integration code |
| CLI tool | Immediate usability, scriptable | Less flexible, harder to embed |
| Service | Centralized, multi-client | Operational complexity, network dependency |

**Rationale**: Load testing tools vary widely. A library lets users integrate timing replay into their existing test harnesses however they need.

### 2. Zero External Dependencies

**Chosen**: stdlib only

| Approach | Pros | Cons |
|----------|------|------|
| **Stdlib only** (chosen) | No version conflicts, no supply chain risk, fast builds | Must implement things manually |
| Use dependencies | Richer features (retries, better RNG) | Dependency management, larger binary |

**Rationale**: For a small focused library, the cost of dependencies outweighs benefits. Box-Muller for log-normal distribution is ~10 lines.

### 3. Inter-Arrival Times vs Absolute Timestamps

**Chosen**: Store deltas (ms between messages)

| Approach | Pros | Cons |
|----------|------|------|
| **Inter-arrivals** (chosen) | Directly usable, smaller (N-1 values), privacy-preserving | Can't reconstruct exact timeline |
| Timestamps | Full information preserved, can slice time ranges | Requires transformation for replay, larger |

**Rationale**: The replay use case only needs "how long to wait" - inter-arrivals are the natural representation.

### 4. Two Replay Modes

**Chosen**: `ModeSequential` and `ModeSample`

| Mode | Use Case | Trade-off |
|------|----------|-----------|
| **Sequential** | Reproduce exact traffic pattern | Wraps around, limited duration |
| **Sample** | Long-running tests, statistically similar | Loses exact ordering, won't reproduce specific bursts |

**Rationale**: Different testing needs require different approaches. Sequential for debugging specific scenarios, sample for stress testing.

### 5. Thread Safety via Mutex

**Chosen**: `sync.Mutex` protecting shared state

| Approach | Pros | Cons |
|----------|------|------|
| **Mutex** (chosen) | Simple, correct, familiar | Contention under extreme concurrency |
| Atomic operations | Lock-free, potentially faster | Complex, error-prone |
| Per-goroutine state | No contention | More memory, loses global sequencing |
| Channel-based | Go-idiomatic | Overhead, complexity |

**Rationale**: Mutex is simple and correct. The critical section is tiny (array lookup + increment), so contention is minimal in practice.

### 6. Panic vs Error for Invalid Input

**Chosen**: Panic for programmer errors (`nil` data, invalid params)

| Approach | Pros | Cons |
|----------|------|------|
| **Panic** (chosen) | Fail-fast, cleaner API | Can crash if misused |
| Return errors | Defensive, recoverable | Clutters API, caller might ignore |

**Rationale**: Passing `nil` to `NewReplay` is a bug, not a runtime condition. Panics catch these during development rather than propagating bad state.

### 7. Speedup as Multiplier

**Chosen**: Single factor applied uniformly

| Approach | Pros | Cons |
|----------|------|------|
| **Uniform multiplier** (chosen) | Simple mental model, predictable | Compresses all patterns equally |
| Min/max clamping | Prevents extremes | More parameters to tune |
| Non-linear scaling | Could preserve burst shapes | Complex, harder to reason about |

**Rationale**: Simplicity. Users can reason about "2x speedup = 2x the load" easily.

### 8. JSON for Serialization

**Chosen**: JSON with descriptive field names

| Format | Pros | Cons |
|--------|------|------|
| **JSON** (chosen) | Human readable, universal tooling | Larger, slower parsing |
| Protobuf | Compact, fast, schema evolution | Build complexity, less inspectable |
| CSV | Simplest, spreadsheet-friendly | No nested structure, no metadata |

**Rationale**: Timing data is small (thousands of floats). Human readability and debuggability outweigh performance.

### 9. Log-Normal for Synthetic Data

**Chosen**: Log-normal distribution via Box-Muller

| Distribution | Pros | Cons |
|--------------|------|------|
| **Log-normal** (chosen) | Matches real network traffic patterns (heavy tail, always positive) | More complex than normal |
| Normal | Simpler | Can produce negative values, unrealistic |
| Exponential | Memoryless, simple | Too regular, no burst modeling |
| Recorded replay | Most realistic | Requires real data |

**Rationale**: Network inter-arrival times are empirically log-normal. This gives realistic synthetic data when real topics aren't available.

### 10. Mirror Node Pagination with Delay

**Chosen**: 100ms default delay between paginated requests

| Approach | Pros | Cons |
|----------|------|------|
| **Sequential with delay** (chosen) | Simple, respects rate limits, configurable | Slower for large fetches |
| Parallel fetching | Faster | Complex, may hit rate limits |
| No delay | Fastest | Risk of rate limiting or blocking |

**Rationale**: The mirror node is a shared public resource. Being a good citizen with modest delays avoids issues and the fetch is typically a one-time operation.

## Data Flow

```
┌─────────────────┐     ┌──────────────┐     ┌─────────────┐
│  HCS Topic      │────▶│ Mirror Node  │────▶│ FetchTiming │
│  (0.0.xxxxx)    │     │ REST API     │     │             │
└─────────────────┘     └──────────────┘     └──────┬──────┘
                                                    │
                                                    ▼
┌─────────────────┐     ┌──────────────┐     ┌─────────────┐
│  Load Test      │◀────│   Replay     │◀────│ TimingData  │
│  (your code)    │     │  NextDelay() │     │             │
└─────────────────┘     └──────────────┘     └─────────────┘
```

## Potential Extensions

These features were intentionally omitted to keep the library focused, but could be added:

1. **Burst detection/replay** - Identify and preserve traffic spikes as distinct patterns
2. **Time-of-day patterns** - Model diurnal variation in real traffic
3. **Percentile-based replay** - Target specific latency percentiles rather than full distribution
4. **Warm-up period** - Gradual ramp-up before reaching full replay speed
5. **Multiple topic correlation** - Replay several topics with preserved timing relationships
6. **Streaming fetch** - Process messages as they arrive rather than collecting all first
