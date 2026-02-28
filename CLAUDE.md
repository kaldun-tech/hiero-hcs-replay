# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

hiero-hcs-replay is a Go library for HCS (Hedera Consensus Service) timing replay. It fetches real message timing patterns from HCS topics via the Hedera Mirror Node REST API and replays them at configurable speeds for realistic load testing.

## Development Commands

```bash
# Run all tests
go test -v ./...

# Run tests with race detection
go test -race ./...

# Run a single test
go test -run TestName ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...

# Lint (requires golangci-lint)
golangci-lint run
```

## Architecture

This is a **library package** (not a CLI/service). Key components:

- `replay.go` - Core types (`TimingData`, `Stats`, `Replay`) and replay logic
- `fetch.go` - HCS message fetching from Hedera Mirror Node REST API

### Public API

**Types:**
- `TimingData` - Timing distribution from an HCS topic
- `Stats` - Statistical summary (min, max, avg, p50, p90, p99)
- `Replay` - Stateful replay engine with `NextDelay()` method
- `Network` - Hedera network constants (Mainnet, Testnet, Previewnet)
- `ReplayMode` - ModeSequential or ModeSample

**Key Functions:**
- `FetchTiming(ctx, topicID, network, limit)` - Fetch from mirror node
- `NewReplay(data, mode, speedup)` - Create replay instance
- `LoadTiming(path)` / `SaveTiming(path, data)` - File I/O
- `GenerateSynthetic(count, avgMs, stddevMs)` - Generate test data

### Data Flow

```
HCS Topic → Mirror Node API → FetchTiming() → TimingData → NewReplay() → NextDelay()
```

## Design Principles

- Zero external dependencies (pure stdlib)
- All public types/functions have godoc comments
- Comprehensive test coverage with mock HTTP server
- Thread-safe replay engine
