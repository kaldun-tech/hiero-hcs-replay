package hcsreplay

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewReplay(t *testing.T) {
	data := &TimingData{
		InterArrivalMs: []float64{100, 200, 300},
	}

	replay := NewReplay(data, ModeSample, 1.0)
	if replay == nil {
		t.Fatal("NewReplay() returned nil")
	}
	if replay.Mode() != ModeSample {
		t.Errorf("Mode() = %q, want %q", replay.Mode(), ModeSample)
	}
	if replay.Speedup() != 1.0 {
		t.Errorf("Speedup() = %f, want 1.0", replay.Speedup())
	}
}

func TestNewReplay_InvalidSpeedup(t *testing.T) {
	data := &TimingData{
		InterArrivalMs: []float64{100, 200, 300},
	}

	// Zero speedup should default to 1.0
	replay := NewReplay(data, ModeSample, 0)
	if replay.Speedup() != 1.0 {
		t.Errorf("Speedup() = %f, want 1.0 (default)", replay.Speedup())
	}

	// Negative speedup should default to 1.0
	replay = NewReplay(data, ModeSample, -5)
	if replay.Speedup() != 1.0 {
		t.Errorf("Speedup() = %f, want 1.0 (default)", replay.Speedup())
	}
}

func TestReplay_NextDelay_Sequential(t *testing.T) {
	data := &TimingData{
		InterArrivalMs: []float64{100, 200, 300},
	}

	replay := NewReplay(data, ModeSequential, 1.0)

	// Sequential mode should return values in order
	expected := []time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
		300 * time.Millisecond,
		100 * time.Millisecond, // Wraps around
	}

	for i, want := range expected {
		got := replay.NextDelay()
		if got != want {
			t.Errorf("NextDelay() #%d = %v, want %v", i, got, want)
		}
	}
}

func TestReplay_NextDelay_Sample(t *testing.T) {
	data := &TimingData{
		InterArrivalMs: []float64{100, 200, 300},
	}

	replay := NewReplay(data, ModeSample, 1.0)

	// Sample mode should return values from the distribution
	validDelays := map[time.Duration]bool{
		100 * time.Millisecond: true,
		200 * time.Millisecond: true,
		300 * time.Millisecond: true,
	}

	for i := 0; i < 10; i++ {
		got := replay.NextDelay()
		if !validDelays[got] {
			t.Errorf("NextDelay() = %v, not in valid set", got)
		}
	}
}

func TestReplay_NextDelay_Speedup(t *testing.T) {
	data := &TimingData{
		InterArrivalMs: []float64{100, 200, 300},
	}

	// 2x speedup means delays should be halved
	replay := NewReplay(data, ModeSequential, 2.0)

	expected := []time.Duration{
		50 * time.Millisecond,  // 100 / 2
		100 * time.Millisecond, // 200 / 2
		150 * time.Millisecond, // 300 / 2
	}

	for i, want := range expected {
		got := replay.NextDelay()
		if got != want {
			t.Errorf("NextDelay() #%d = %v, want %v", i, got, want)
		}
	}
}

func TestReplay_EffectiveRate(t *testing.T) {
	data := &TimingData{
		AvgRatePerSecond: 10.0,
		InterArrivalMs:   []float64{100},
	}

	replay := NewReplay(data, ModeSample, 2.0)
	if got := replay.EffectiveRate(); got != 20.0 {
		t.Errorf("EffectiveRate() = %f, want 20.0", got)
	}
}

func TestReplay_Data(t *testing.T) {
	data := &TimingData{
		TopicID:        "0.0.12345",
		InterArrivalMs: []float64{100},
	}

	replay := NewReplay(data, ModeSample, 1.0)
	if replay.Data().TopicID != "0.0.12345" {
		t.Errorf("Data().TopicID = %q, want %q", replay.Data().TopicID, "0.0.12345")
	}
}

func TestLoadTiming(t *testing.T) {
	data := TimingData{
		TopicID:          "0.0.123456",
		Network:          "testnet",
		MessageCount:     5,
		TimeSpanSeconds:  10.0,
		AvgRatePerSecond: 0.5,
		InterArrivalMs:   []float64{100, 200, 150, 300, 250},
		Stats: Stats{
			MinMs: 100,
			MaxMs: 300,
			AvgMs: 200,
			P50Ms: 200,
			P90Ms: 300,
			P99Ms: 300,
		},
	}

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "timing.json")

	jsonData, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	if err := os.WriteFile(tmpFile, jsonData, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	loaded, err := LoadTiming(tmpFile)
	if err != nil {
		t.Fatalf("LoadTiming() error = %v", err)
	}

	if loaded.TopicID != data.TopicID {
		t.Errorf("TopicID = %q, want %q", loaded.TopicID, data.TopicID)
	}
	if loaded.MessageCount != data.MessageCount {
		t.Errorf("MessageCount = %d, want %d", loaded.MessageCount, data.MessageCount)
	}
	if len(loaded.InterArrivalMs) != len(data.InterArrivalMs) {
		t.Errorf("InterArrivalMs length = %d, want %d", len(loaded.InterArrivalMs), len(data.InterArrivalMs))
	}
}

func TestLoadTiming_NotFound(t *testing.T) {
	_, err := LoadTiming("/nonexistent/path/timing.json")
	if err == nil {
		t.Error("LoadTiming() expected error for non-existent file, got nil")
	}
}

func TestLoadTiming_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "invalid.json")

	if err := os.WriteFile(tmpFile, []byte("not valid json"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	_, err := LoadTiming(tmpFile)
	if err == nil {
		t.Error("LoadTiming() expected error for invalid JSON, got nil")
	}
}

func TestLoadTiming_EmptyInterArrivals(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "empty.json")

	data := TimingData{
		TopicID:        "0.0.123",
		InterArrivalMs: []float64{}, // Empty
	}
	jsonData, _ := json.Marshal(data)
	if err := os.WriteFile(tmpFile, jsonData, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	_, err := LoadTiming(tmpFile)
	if err == nil {
		t.Error("LoadTiming() expected error for empty inter-arrivals, got nil")
	}
}

func TestReadTiming(t *testing.T) {
	data := TimingData{
		TopicID:        "0.0.12345",
		InterArrivalMs: []float64{100, 200},
	}
	jsonData, _ := json.Marshal(data)

	loaded, err := ReadTiming(bytes.NewReader(jsonData))
	if err != nil {
		t.Fatalf("ReadTiming() error = %v", err)
	}

	if loaded.TopicID != data.TopicID {
		t.Errorf("TopicID = %q, want %q", loaded.TopicID, data.TopicID)
	}
}

func TestWriteTiming(t *testing.T) {
	data := &TimingData{
		TopicID:        "0.0.12345",
		Network:        "testnet",
		InterArrivalMs: []float64{100, 200},
	}

	var buf bytes.Buffer
	if err := WriteTiming(&buf, data); err != nil {
		t.Fatalf("WriteTiming() error = %v", err)
	}

	// Read it back
	loaded, err := ReadTiming(&buf)
	if err != nil {
		t.Fatalf("ReadTiming() error = %v", err)
	}

	if loaded.TopicID != data.TopicID {
		t.Errorf("TopicID = %q, want %q", loaded.TopicID, data.TopicID)
	}
}

func TestSaveTiming(t *testing.T) {
	data := &TimingData{
		TopicID:        "0.0.12345",
		Network:        "testnet",
		InterArrivalMs: []float64{100, 200},
	}

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "output.json")

	if err := SaveTiming(tmpFile, data); err != nil {
		t.Fatalf("SaveTiming() error = %v", err)
	}

	loaded, err := LoadTiming(tmpFile)
	if err != nil {
		t.Fatalf("LoadTiming() error = %v", err)
	}

	if loaded.TopicID != data.TopicID {
		t.Errorf("TopicID = %q, want %q", loaded.TopicID, data.TopicID)
	}
}

func TestGenerateSynthetic(t *testing.T) {
	data := GenerateSynthetic(100, 50.0, 20.0)

	if data == nil {
		t.Fatal("GenerateSynthetic() returned nil")
	}
	if data.TopicID != "synthetic" {
		t.Errorf("TopicID = %q, want %q", data.TopicID, "synthetic")
	}
	if data.Network != "generated" {
		t.Errorf("Network = %q, want %q", data.Network, "generated")
	}
	if data.MessageCount != 100 {
		t.Errorf("MessageCount = %d, want 100", data.MessageCount)
	}
	if len(data.InterArrivalMs) != 100 {
		t.Errorf("InterArrivalMs length = %d, want 100", len(data.InterArrivalMs))
	}

	// Verify all values are positive
	for i, v := range data.InterArrivalMs {
		if v < 1 {
			t.Errorf("InterArrivalMs[%d] = %f, want >= 1", i, v)
		}
	}

	// Stats should be populated
	if data.Stats.MinMs <= 0 {
		t.Error("Stats.MinMs should be > 0")
	}
	if data.Stats.MaxMs < data.Stats.MinMs {
		t.Error("Stats.MaxMs should be >= MinMs")
	}
	if data.Stats.AvgMs <= 0 {
		t.Error("Stats.AvgMs should be > 0")
	}
}

func TestCalculateStats(t *testing.T) {
	// Test with known values
	vals := []float64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	stats := CalculateStats(vals)

	if stats.MinMs != 10 {
		t.Errorf("MinMs = %f, want 10", stats.MinMs)
	}
	if stats.MaxMs != 100 {
		t.Errorf("MaxMs = %f, want 100", stats.MaxMs)
	}
	if stats.AvgMs != 55 {
		t.Errorf("AvgMs = %f, want 55", stats.AvgMs)
	}
	// P50 should be around index 5 (60)
	if stats.P50Ms != 60 {
		t.Errorf("P50Ms = %f, want 60", stats.P50Ms)
	}
}

func TestCalculateStats_Empty(t *testing.T) {
	stats := CalculateStats([]float64{})
	if stats.MinMs != 0 || stats.MaxMs != 0 || stats.AvgMs != 0 {
		t.Error("CalculateStats([]) should return zero stats")
	}
}

func TestNewReplay_NilData(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("NewReplay(nil) should panic")
		}
	}()
	NewReplay(nil, ModeSample, 1.0)
}

func TestNewReplay_EmptyInterArrivals(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("NewReplay with empty InterArrivalMs should panic")
		}
	}()
	data := &TimingData{InterArrivalMs: []float64{}}
	NewReplay(data, ModeSample, 1.0)
}

func TestGenerateSynthetic_InvalidCount(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("GenerateSynthetic(0, ...) should panic")
		}
	}()
	GenerateSynthetic(0, 50.0, 20.0)
}

func TestGenerateSynthetic_InvalidAvgMs(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("GenerateSynthetic(..., 0, ...) should panic")
		}
	}()
	GenerateSynthetic(100, 0, 20.0)
}

func TestHelperFunctions(t *testing.T) {
	// Test average
	vals := []float64{10, 20, 30, 40, 50}
	avg := average(vals)
	if avg != 30.0 {
		t.Errorf("average() = %f, want 30.0", avg)
	}

	// Test average empty
	if avg := average([]float64{}); avg != 0 {
		t.Errorf("average([]) = %f, want 0", avg)
	}

	// Test sum
	s := sum(vals)
	if s != 150.0 {
		t.Errorf("sum() = %f, want 150.0", s)
	}

	// Test percentile
	sorted := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	p50 := percentile(sorted, 0.50)
	if p50 != 6.0 {
		t.Errorf("percentile(50) = %f, want 6.0", p50)
	}

	p90 := percentile(sorted, 0.90)
	if p90 != 10.0 {
		t.Errorf("percentile(90) = %f, want 10.0", p90)
	}

	// Test percentile empty
	if p := percentile([]float64{}, 0.50); p != 0 {
		t.Errorf("percentile([]) = %f, want 0", p)
	}
}
