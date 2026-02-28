// Package hcsreplay provides HCS (Hedera Consensus Service) timing replay functionality.
//
// This library allows developers to fetch real message timing patterns from HCS topics
// and replay them at configurable speeds to drive realistic load tests against Hedera workloads.
//
// Basic usage:
//
//	// Fetch timing data from a real HCS topic
//	data, err := hcsreplay.FetchTiming(ctx, "0.0.120438", hcsreplay.Mainnet, 1000)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Create a replay instance
//	replay := hcsreplay.NewReplay(data, hcsreplay.ModeSample, 1.0)
//
//	// Use in your load test
//	for {
//	    delay := replay.NextDelay()
//	    time.Sleep(delay)
//	    // ... perform operation
//	}
package hcsreplay

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"sort"
	"sync"
	"time"
)

// ReplayMode determines how delays are selected from the timing distribution.
type ReplayMode string

const (
	// ModeSequential replays delays in the exact order they were recorded,
	// wrapping around when the end is reached. Use this mode to reproduce
	// the exact traffic pattern.
	ModeSequential ReplayMode = "sequential"

	// ModeSample randomly samples delays from the distribution. Use this mode
	// for longer-running tests where you want statistically similar traffic
	// without exact reproduction.
	ModeSample ReplayMode = "sample"
)

// Stats holds statistical summary of inter-arrival times in milliseconds.
type Stats struct {
	MinMs float64 `json:"min_ms"`
	MaxMs float64 `json:"max_ms"`
	AvgMs float64 `json:"avg_ms"`
	P50Ms float64 `json:"p50_ms"`
	P90Ms float64 `json:"p90_ms"`
	P99Ms float64 `json:"p99_ms"`
}

// TimingData represents a timing distribution captured from an HCS topic.
// It contains inter-arrival times (delays between consecutive messages)
// that can be replayed to simulate realistic workloads.
type TimingData struct {
	// TopicID is the HCS topic identifier (e.g., "0.0.120438").
	TopicID string `json:"topic_id"`

	// Network is the Hedera network where the data was captured
	// (mainnet, testnet, or previewnet).
	Network string `json:"network"`

	// MessageCount is the number of messages in the sample.
	MessageCount int `json:"message_count"`

	// TimeSpanSeconds is the total duration covered by the sample.
	TimeSpanSeconds float64 `json:"time_span_seconds"`

	// AvgRatePerSecond is the average message rate.
	AvgRatePerSecond float64 `json:"avg_rate_per_second"`

	// InterArrivalMs contains the inter-arrival times in milliseconds.
	// Each value represents the delay between consecutive messages.
	InterArrivalMs []float64 `json:"inter_arrival_ms"`

	// Stats contains statistical summary of the inter-arrival times.
	Stats Stats `json:"stats"`
}

// Replay provides realistic inter-arrival delays based on HCS timing data.
// It is safe for concurrent use from multiple goroutines.
type Replay struct {
	data    *TimingData
	rng     *rand.Rand
	mu      sync.Mutex
	index   int
	mode    ReplayMode
	speedup float64
}

// NewReplay creates a new timing replay from loaded data.
//
// Parameters:
//   - data: The timing data to replay (from FetchTiming, LoadTiming, or GenerateSynthetic)
//   - mode: ModeSequential for exact replay, ModeSample for random sampling
//   - speedup: Multiplier for replay speed (1.0 = real-time, 2.0 = 2x faster, 0.5 = 2x slower)
//
// If speedup is <= 0, it defaults to 1.0 (real-time).
// Panics if data is nil or has no inter-arrival values.
func NewReplay(data *TimingData, mode ReplayMode, speedup float64) *Replay {
	if data == nil {
		panic("hcsreplay: NewReplay called with nil data")
	}
	if len(data.InterArrivalMs) == 0 {
		panic("hcsreplay: NewReplay called with empty InterArrivalMs")
	}
	if speedup <= 0 {
		speedup = 1.0
	}
	return &Replay{
		data:    data,
		rng:     rand.New(rand.NewSource(time.Now().UnixNano())),
		index:   0,
		mode:    mode,
		speedup: speedup,
	}
}

// NextDelay returns the next inter-arrival delay to use before the next operation.
//
// In ModeSequential, delays are returned in the exact order they were recorded,
// wrapping around to the beginning when the end is reached.
//
// In ModeSample, delays are randomly sampled from the distribution.
//
// The returned delay is adjusted by the speedup factor.
func (r *Replay) NextDelay() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()

	var delayMs float64

	switch r.mode {
	case ModeSequential:
		delayMs = r.data.InterArrivalMs[r.index]
		r.index = (r.index + 1) % len(r.data.InterArrivalMs)
	default: // ModeSample or unknown defaults to sampling
		delayMs = r.data.InterArrivalMs[r.rng.Intn(len(r.data.InterArrivalMs))]
	}

	// Apply speedup factor
	delayMs = delayMs / r.speedup

	return time.Duration(delayMs * float64(time.Millisecond))
}

// Data returns the underlying timing data.
func (r *Replay) Data() *TimingData {
	return r.data
}

// Mode returns the replay mode.
func (r *Replay) Mode() ReplayMode {
	return r.mode
}

// Speedup returns the speedup factor.
func (r *Replay) Speedup() float64 {
	return r.speedup
}

// EffectiveRate returns the effective message rate after applying speedup.
// This is the average rate at which operations will be performed.
func (r *Replay) EffectiveRate() float64 {
	return r.data.AvgRatePerSecond * r.speedup
}

// LoadTiming loads timing data from a JSON file.
func LoadTiming(path string) (*TimingData, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open timing file: %w", err)
	}
	defer f.Close()

	return ReadTiming(f)
}

// ReadTiming reads timing data from an io.Reader.
func ReadTiming(r io.Reader) (*TimingData, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read timing data: %w", err)
	}

	var timing TimingData
	if err := json.Unmarshal(data, &timing); err != nil {
		return nil, fmt.Errorf("failed to parse timing data: %w", err)
	}

	if len(timing.InterArrivalMs) == 0 {
		return nil, fmt.Errorf("timing data has no inter-arrival values")
	}

	return &timing, nil
}

// WriteTiming writes timing data to an io.Writer as JSON.
func WriteTiming(w io.Writer, data *TimingData) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// SaveTiming saves timing data to a JSON file.
func SaveTiming(path string, data *TimingData) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create timing file: %w", err)
	}
	defer f.Close()

	return WriteTiming(f, data)
}

// GenerateSynthetic creates synthetic timing data for testing.
// The distribution follows a log-normal pattern typical of real network traffic.
//
// Parameters:
//   - count: Number of inter-arrival samples to generate (must be > 0)
//   - avgMs: Target average inter-arrival time in milliseconds (must be > 0)
//   - stddevMs: Standard deviation in milliseconds (must be >= 0)
//
// Panics if count <= 0 or avgMs <= 0.
func GenerateSynthetic(count int, avgMs, stddevMs float64) *TimingData {
	if count <= 0 {
		panic("hcsreplay: GenerateSynthetic called with count <= 0")
	}
	if avgMs <= 0 {
		panic("hcsreplay: GenerateSynthetic called with avgMs <= 0")
	}
	if stddevMs < 0 {
		stddevMs = 0
	}
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Generate log-normal distributed inter-arrivals
	interArrivals := make([]float64, count)
	for i := range interArrivals {
		// Box-Muller transform for normal distribution
		u1 := rng.Float64()
		u2 := rng.Float64()
		z := math.Sqrt(-2*math.Log(u1)) * math.Cos(2*math.Pi*u2)

		// Convert to log-normal
		logMean := math.Log(avgMs) - 0.5*math.Log(1+(stddevMs*stddevMs)/(avgMs*avgMs))
		logStd := math.Sqrt(math.Log(1 + (stddevMs*stddevMs)/(avgMs*avgMs)))

		value := math.Exp(logMean + logStd*z)
		if value < 1 {
			value = 1 // Minimum 1ms
		}
		interArrivals[i] = value
	}

	stats := CalculateStats(interArrivals)
	totalMs := sum(interArrivals)
	timeSpanS := totalMs / 1000

	return &TimingData{
		TopicID:          "synthetic",
		Network:          "generated",
		MessageCount:     count,
		TimeSpanSeconds:  timeSpanS,
		AvgRatePerSecond: float64(count) / timeSpanS,
		InterArrivalMs:   interArrivals,
		Stats:            stats,
	}
}

// CalculateStats computes statistics for a slice of inter-arrival times.
func CalculateStats(interArrivals []float64) Stats {
	if len(interArrivals) == 0 {
		return Stats{}
	}

	sorted := make([]float64, len(interArrivals))
	copy(sorted, interArrivals)
	sort.Float64s(sorted)

	return Stats{
		MinMs: sorted[0],
		MaxMs: sorted[len(sorted)-1],
		AvgMs: average(sorted),
		P50Ms: percentile(sorted, 0.50),
		P90Ms: percentile(sorted, 0.90),
		P99Ms: percentile(sorted, 0.99),
	}
}

func average(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	return sum(vals) / float64(len(vals))
}

func sum(vals []float64) float64 {
	var total float64
	for _, v := range vals {
		total += v
	}
	return total
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(p * float64(len(sorted)))
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}
