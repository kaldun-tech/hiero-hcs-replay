package hcsreplay_test

import (
	"bytes"
	"context"
	"fmt"
	"time"

	hcsreplay "github.com/kaldun-tech/hiero-hcs-replay"
)

func ExampleNewReplay() {
	// Create timing data (normally from FetchTiming or LoadTiming)
	data := &hcsreplay.TimingData{
		TopicID:          "0.0.12345",
		Network:          "testnet",
		MessageCount:     5,
		AvgRatePerSecond: 2.0,
		InterArrivalMs:   []float64{100, 200, 150, 300, 250},
	}

	// Create replay with 2x speedup
	replay := hcsreplay.NewReplay(data, hcsreplay.ModeSample, 2.0)

	// Use in load test
	for i := 0; i < 3; i++ {
		delay := replay.NextDelay()
		fmt.Printf("Operation %d: wait %v\n", i+1, delay.Truncate(time.Millisecond))
	}

	// Output varies due to random sampling
}

func ExampleNewReplay_sequential() {
	data := &hcsreplay.TimingData{
		InterArrivalMs: []float64{100, 200, 300},
	}

	// Sequential mode replays in exact order
	replay := hcsreplay.NewReplay(data, hcsreplay.ModeSequential, 1.0)

	for i := 0; i < 4; i++ {
		delay := replay.NextDelay()
		fmt.Printf("%v\n", delay)
	}
	// Output:
	// 100ms
	// 200ms
	// 300ms
	// 100ms
}

func ExampleGenerateSynthetic() {
	// Generate synthetic timing for testing
	data := hcsreplay.GenerateSynthetic(100, 50.0, 20.0)

	fmt.Printf("Generated %d samples\n", data.MessageCount)
	fmt.Printf("Average: %.1fms\n", data.Stats.AvgMs)
	// Output varies due to random generation
}

func ExampleWriteTiming() {
	data := &hcsreplay.TimingData{
		TopicID:          "0.0.12345",
		Network:          "testnet",
		MessageCount:     3,
		TimeSpanSeconds:  0.6,
		AvgRatePerSecond: 5.0,
		InterArrivalMs:   []float64{100, 200, 300},
		Stats: hcsreplay.Stats{
			MinMs: 100,
			MaxMs: 300,
			AvgMs: 200,
			P50Ms: 200,
			P90Ms: 300,
			P99Ms: 300,
		},
	}

	var buf bytes.Buffer
	_ = hcsreplay.WriteTiming(&buf, data)

	// Read it back
	loaded, _ := hcsreplay.ReadTiming(&buf)
	fmt.Printf("TopicID: %s\n", loaded.TopicID)
	// Output:
	// TopicID: 0.0.12345
}

func ExampleCalculateStats() {
	interArrivals := []float64{50, 100, 150, 200, 250, 300, 350, 400, 450, 500}
	stats := hcsreplay.CalculateStats(interArrivals)

	fmt.Printf("Min: %.0fms\n", stats.MinMs)
	fmt.Printf("Max: %.0fms\n", stats.MaxMs)
	fmt.Printf("Avg: %.0fms\n", stats.AvgMs)
	fmt.Printf("P50: %.0fms\n", stats.P50Ms)
	// Output:
	// Min: 50ms
	// Max: 500ms
	// Avg: 275ms
	// P50: 300ms
}

func ExampleFetchTiming() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Fetch timing from a real HCS topic
	// Note: This example would actually make network calls
	data, err := hcsreplay.FetchTiming(ctx, "0.0.120438", hcsreplay.Mainnet, 100)
	if err != nil {
		// Handle error (topic not found, network issues, etc.)
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Fetched %d messages from %s\n", data.MessageCount, data.TopicID)
}

func ExampleFetchTimingWithOptions() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	opts := hcsreplay.FetchOptions{
		RequestDelay: 200 * time.Millisecond, // Slower rate limiting
		OnProgress: func(fetched int) {
			fmt.Printf("Fetched %d messages...\n", fetched)
		},
	}

	data, err := hcsreplay.FetchTimingWithOptions(ctx, "0.0.120438", hcsreplay.Mainnet, 100, opts)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Done: %d messages\n", data.MessageCount)
}

func ExampleReplay_EffectiveRate() {
	data := &hcsreplay.TimingData{
		AvgRatePerSecond: 10.0,
		InterArrivalMs:   []float64{100},
	}

	// 5x speedup
	replay := hcsreplay.NewReplay(data, hcsreplay.ModeSample, 5.0)

	fmt.Printf("Original rate: %.1f msg/s\n", data.AvgRatePerSecond)
	fmt.Printf("Effective rate: %.1f msg/s\n", replay.EffectiveRate())
	// Output:
	// Original rate: 10.0 msg/s
	// Effective rate: 50.0 msg/s
}
