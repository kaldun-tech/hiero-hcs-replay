# hiero-hcs-replay

[![CI](https://github.com/kaldun-tech/hiero-hcs-replay/actions/workflows/ci.yml/badge.svg)](https://github.com/kaldun-tech/hiero-hcs-replay/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/kaldun-tech/hiero-hcs-replay.svg)](https://pkg.go.dev/github.com/kaldun-tech/hiero-hcs-replay)
[![Go Report Card](https://goreportcard.com/badge/github.com/kaldun-tech/hiero-hcs-replay)](https://goreportcard.com/report/github.com/kaldun-tech/hiero-hcs-replay)

A reusable Go library for HCS (Hedera Consensus Service) timing replay. Fetch real message timing patterns from HCS topics and replay them at configurable speeds to drive realistic load tests against Hedera workloads.

## The Problem

Load testing Hedera applications with synthetic data produces unrealistic results.
Real HCS topics have bursty, irregular message patterns that stress systems
differently than uniform traffic generators.

## The Solution

hiero-hcs-replay captures real timing patterns from production HCS topics and
replays them at any speed. Your load tests now match production behavior.

## Features

- **Fetch real timing data** from any public HCS topic via the Hedera Mirror Node REST API
- **Replay modes**: Sequential (exact reproduction) or Sample (statistically similar)
- **Configurable speedup**: Run tests at 1x, 10x, or any multiplier
- **Zero external dependencies**: Pure Go standard library (no Hedera SDK required)
- **Production-ready**: Comprehensive tests, CI/CD, proper error handling

## Installation

```bash
go get github.com/kaldun-tech/hiero-hcs-replay
```

Requires Go 1.21 or later.

## Quick Start

### Fetch timing from a real HCS topic

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    hcsreplay "github.com/kaldun-tech/hiero-hcs-replay"
)

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
    defer cancel()

    // Fetch timing data from a mainnet HCS topic
    data, err := hcsreplay.FetchTiming(ctx, "0.0.120438", hcsreplay.Mainnet, 1000)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Fetched %d messages from topic %s\n", data.MessageCount, data.TopicID)
    fmt.Printf("Average rate: %.2f msg/s\n", data.AvgRatePerSecond)
    fmt.Printf("P50 inter-arrival: %.1fms, P99: %.1fms\n", data.Stats.P50Ms, data.Stats.P99Ms)

    // Save for later use
    if err := hcsreplay.SaveTiming("timing.json", data); err != nil {
        log.Fatal(err)
    }
}
```

### Replay timing in a load test

```go
package main

import (
    "fmt"
    "log"
    "time"

    hcsreplay "github.com/kaldun-tech/hiero-hcs-replay"
)

func main() {
    // Load previously saved timing data
    data, err := hcsreplay.LoadTiming("timing.json")
    if err != nil {
        log.Fatal(err)
    }

    // Create replay with 10x speedup
    replay := hcsreplay.NewReplay(data, hcsreplay.ModeSample, 10.0)

    fmt.Printf("Effective rate: %.1f req/s\n", replay.EffectiveRate())

    // Simulate load test loop
    for i := 0; i < 10; i++ {
        delay := replay.NextDelay()
        time.Sleep(delay)

        // Your operation here (API call, transaction, etc.)
        fmt.Printf("Operation %d after %v delay\n", i+1, delay.Round(time.Millisecond))
    }
}
```

### Generate synthetic timing for testing

```go
// Generate 1000 samples with avg=50ms, stddev=20ms (log-normal distribution)
data := hcsreplay.GenerateSynthetic(1000, 50.0, 20.0)

replay := hcsreplay.NewReplay(data, hcsreplay.ModeSample, 1.0)
```

## API Reference

### Types

#### `TimingData`
Contains timing distribution data from an HCS topic:
- `TopicID`: HCS topic identifier (e.g., "0.0.120438")
- `Network`: Hedera network (mainnet, testnet, previewnet)
- `MessageCount`: Number of messages in sample
- `TimeSpanSeconds`: Duration covered by sample
- `AvgRatePerSecond`: Average message rate
- `InterArrivalMs`: Inter-arrival times in milliseconds
- `Stats`: Statistical summary (min, max, avg, p50, p90, p99)

#### `Replay`
Stateful replay engine that produces realistic delays:
- `NextDelay()`: Returns the next delay to wait
- `EffectiveRate()`: Returns rate after speedup applied
- `Mode()`: Returns replay mode
- `Speedup()`: Returns speedup factor

### Functions

#### Fetching
```go
// Fetch from public mirror node
FetchTiming(ctx, topicID, network, limit) (*TimingData, error)

// Fetch with custom options (base URL, HTTP client, progress callback)
FetchTimingWithOptions(ctx, topicID, network, limit, opts) (*TimingData, error)
```

#### File I/O
```go
LoadTiming(path) (*TimingData, error)
SaveTiming(path, data) error
ReadTiming(reader) (*TimingData, error)
WriteTiming(writer, data) error
```

#### Replay
```go
NewReplay(data, mode, speedup) *Replay
GenerateSynthetic(count, avgMs, stddevMs) *TimingData
CalculateStats(interArrivals) Stats
```

### Networks
```go
hcsreplay.Mainnet    // mainnet-public.mirrornode.hedera.com
hcsreplay.Testnet    // testnet.mirrornode.hedera.com
hcsreplay.Previewnet // previewnet.mirrornode.hedera.com
```

### Replay Modes
```go
hcsreplay.ModeSequential // Exact order, wraps around
hcsreplay.ModeSample     // Random sampling from distribution
```

## Live Demo

See [hiero-hcs-replay-demo](https://github.com/kaldun-tech/hiero-hcs-replay-demo) for a
complete example that:

1. Fetches timing from a real mainnet HCS topic
2. Creates a test topic on Hedera testnet
3. Replays timing with actual HCS message submissions
4. Generates on-chain proof of activity

This demonstrates the full integration loop with the Hedera SDK.

## Real-World Usage

This library was extracted from [grpc-rest-benchmark](https://github.com/kaldun-tech/grpc-rest-benchmark), a gRPC vs REST performance benchmarking tool for Hedera workloads. See that project for a complete example of using HCS timing replay in production load tests.

## Supported Networks

| Network | Mirror Node URL |
|---------|-----------------|
| Mainnet | `https://mainnet-public.mirrornode.hedera.com` |
| Testnet | `https://testnet.mirrornode.hedera.com` |
| Previewnet | `https://previewnet.mirrornode.hedera.com` |

You can also use custom mirror node URLs via `FetchTimingWithOptions`.

## Network Impact

How hiero-hcs-replay drives Hedera ecosystem growth:

- **Developer Adoption**: Lowers barrier for HCS integration testing
- **Quality Assurance**: Better testing → more robust dApps → higher user confidence
- **TPS Generation**: Load tests using real patterns generate transaction volume
- **Account Creation**: Test harnesses create dedicated Hedera accounts

### Projected Impact
- Target: Adopted by 50+ Hedera projects for load testing
- Each project: 10-100 test accounts, 1K-10K TPS during testing cycles

## Validation

### Origin
Extracted from [grpc-rest-benchmark](https://github.com/kaldun-tech/grpc-rest-benchmark),
a production benchmarking tool for Hedera workloads.

### Community Feedback
- Developed in response to developer need for realistic load testing patterns
- Follows Hedera community best practices (MIT license, DCO compliance)

### On-Chain Proof
- Demo topic: [0.0.8085696](https://hashscan.io/testnet/topic/0.0.8085696)
- See [hiero-hcs-replay-demo](https://github.com/kaldun-tech/hiero-hcs-replay-demo) for live integration

## Roadmap

### v1.0 (Current - Hackathon)
- ✅ Core fetch/replay functionality
- ✅ Sequential and sample modes
- ✅ Synthetic data generation
- ✅ Comprehensive test coverage (91.7%)

### v1.1 (Post-hackathon)
- [ ] CLI wrapper for quick command-line usage
- [ ] Integration with popular Go test frameworks
- [ ] HTS token timing pattern support

### v2.0 (Future)
- [ ] WebSocket support for live traffic capture
- [ ] Prometheus/Grafana metrics export
- [ ] Multi-topic correlation

### Go-To-Market
1. Submit to awesome-hedera curated list
2. Publish blog post on Hedera Developer Portal
3. Propose integration with Hedera Agent Kit
4. Present at Hedera developer meetups

## Design

For architectural decisions and trade-offs, see [DESIGN.md](DESIGN.md).

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

MIT License - see [LICENSE](LICENSE) for details.

## Acknowledgments

Part of the [Hiero](https://hiero.org) ecosystem for Hedera development tools.
