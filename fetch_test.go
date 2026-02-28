package hcsreplay

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// floatEquals checks if two floats are approximately equal (within 0.001).
func floatEquals(a, b float64) bool {
	return math.Abs(a-b) < 0.001
}

func TestNetwork_MirrorNodeURL(t *testing.T) {
	tests := []struct {
		network Network
		wantURL string
	}{
		{Mainnet, "https://mainnet-public.mirrornode.hedera.com"},
		{Testnet, "https://testnet.mirrornode.hedera.com"},
		{Previewnet, "https://previewnet.mirrornode.hedera.com"},
		{Network("unknown"), ""},
	}

	for _, tt := range tests {
		t.Run(string(tt.network), func(t *testing.T) {
			if got := tt.network.MirrorNodeURL(); got != tt.wantURL {
				t.Errorf("MirrorNodeURL() = %q, want %q", got, tt.wantURL)
			}
		})
	}
}

func TestFetchTimingWithOptions(t *testing.T) {
	// Create a mock server
	messages := []hcsMessage{
		{ConsensusTimestamp: "1704067200.000000000", SequenceNumber: 1},
		{ConsensusTimestamp: "1704067200.100000000", SequenceNumber: 2},
		{ConsensusTimestamp: "1704067200.250000000", SequenceNumber: 3},
		{ConsensusTimestamp: "1704067200.500000000", SequenceNumber: 4},
		{ConsensusTimestamp: "1704067201.000000000", SequenceNumber: 5},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := hcsResponse{Messages: messages}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	ctx := context.Background()
	opts := FetchOptions{
		BaseURL:      server.URL,
		RequestDelay: 1 * time.Millisecond,
		HTTPClient:   server.Client(),
	}

	data, err := FetchTimingWithOptions(ctx, "0.0.12345", Testnet, 100, opts)
	if err != nil {
		t.Fatalf("FetchTimingWithOptions() error = %v", err)
	}

	if data.TopicID != "0.0.12345" {
		t.Errorf("TopicID = %q, want %q", data.TopicID, "0.0.12345")
	}
	if data.MessageCount != 5 {
		t.Errorf("MessageCount = %d, want 5", data.MessageCount)
	}
	if len(data.InterArrivalMs) != 4 {
		t.Errorf("InterArrivalMs length = %d, want 4", len(data.InterArrivalMs))
	}

	// Verify inter-arrival times (in ms) - use approximate comparison for float precision
	expected := []float64{100, 150, 250, 500}
	for i, want := range expected {
		if !floatEquals(data.InterArrivalMs[i], want) {
			t.Errorf("InterArrivalMs[%d] = %f, want ~%f", i, data.InterArrivalMs[i], want)
		}
	}
}

func TestFetchTimingWithOptions_Pagination(t *testing.T) {
	page1Messages := []hcsMessage{
		{ConsensusTimestamp: "1704067200.000000000", SequenceNumber: 1},
		{ConsensusTimestamp: "1704067200.100000000", SequenceNumber: 2},
	}
	page2Messages := []hcsMessage{
		{ConsensusTimestamp: "1704067200.200000000", SequenceNumber: 3},
		{ConsensusTimestamp: "1704067200.300000000", SequenceNumber: 4},
	}

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		var resp hcsResponse
		if requestCount == 1 {
			resp = hcsResponse{
				Messages: page1Messages,
				Links:    struct{ Next string `json:"next"` }{Next: "/api/v1/topics/0.0.12345/messages?timestamp=gt:1704067200.100000000"},
			}
		} else {
			resp = hcsResponse{Messages: page2Messages}
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	ctx := context.Background()
	opts := FetchOptions{
		BaseURL:      server.URL,
		RequestDelay: 1 * time.Millisecond,
		HTTPClient:   server.Client(),
	}

	data, err := FetchTimingWithOptions(ctx, "0.0.12345", Testnet, 100, opts)
	if err != nil {
		t.Fatalf("FetchTimingWithOptions() error = %v", err)
	}

	if data.MessageCount != 4 {
		t.Errorf("MessageCount = %d, want 4", data.MessageCount)
	}
	if requestCount != 2 {
		t.Errorf("requestCount = %d, want 2", requestCount)
	}
}

func TestFetchTimingWithOptions_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	ctx := context.Background()
	opts := FetchOptions{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}

	_, err := FetchTimingWithOptions(ctx, "0.0.12345", Testnet, 100, opts)
	if err == nil {
		t.Error("FetchTimingWithOptions() expected error for 404, got nil")
	}
}

func TestFetchTimingWithOptions_NotEnoughMessages(t *testing.T) {
	messages := []hcsMessage{
		{ConsensusTimestamp: "1704067200.000000000", SequenceNumber: 1},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(hcsResponse{Messages: messages})
	}))
	defer server.Close()

	ctx := context.Background()
	opts := FetchOptions{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}

	_, err := FetchTimingWithOptions(ctx, "0.0.12345", Testnet, 100, opts)
	if err == nil {
		t.Error("FetchTimingWithOptions() expected error for < 2 messages, got nil")
	}
}

func TestFetchTimingWithOptions_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		json.NewEncoder(w).Encode(hcsResponse{})
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	opts := FetchOptions{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}

	_, err := FetchTimingWithOptions(ctx, "0.0.12345", Testnet, 100, opts)
	if err == nil {
		t.Error("FetchTimingWithOptions() expected error for cancelled context, got nil")
	}
}

func TestFetchTimingWithOptions_OnProgress(t *testing.T) {
	messages := []hcsMessage{
		{ConsensusTimestamp: "1704067200.000000000", SequenceNumber: 1},
		{ConsensusTimestamp: "1704067200.100000000", SequenceNumber: 2},
		{ConsensusTimestamp: "1704067200.200000000", SequenceNumber: 3},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(hcsResponse{Messages: messages})
	}))
	defer server.Close()

	var progressCalls []int
	ctx := context.Background()
	opts := FetchOptions{
		BaseURL:      server.URL,
		RequestDelay: 1 * time.Millisecond,
		HTTPClient:   server.Client(),
		OnProgress: func(fetched int) {
			progressCalls = append(progressCalls, fetched)
		},
	}

	_, err := FetchTimingWithOptions(ctx, "0.0.12345", Testnet, 100, opts)
	if err != nil {
		t.Fatalf("FetchTimingWithOptions() error = %v", err)
	}

	if len(progressCalls) != 1 {
		t.Errorf("progressCalls = %d, want 1", len(progressCalls))
	}
	if progressCalls[0] != 3 {
		t.Errorf("progressCalls[0] = %d, want 3", progressCalls[0])
	}
}

func TestFetchTimingWithOptions_UnknownNetwork(t *testing.T) {
	ctx := context.Background()
	opts := FetchOptions{} // No BaseURL override

	_, err := FetchTimingWithOptions(ctx, "0.0.12345", Network("invalid"), 100, opts)
	if err == nil {
		t.Error("FetchTimingWithOptions() expected error for unknown network, got nil")
	}
}

func TestFetchTimingWithOptions_Limit(t *testing.T) {
	messages := make([]hcsMessage, 10)
	for i := range messages {
		messages[i] = hcsMessage{
			ConsensusTimestamp: "1704067200.000000000",
			SequenceNumber:     int64(i + 1),
		}
		// Add 100ms between each message
		messages[i].ConsensusTimestamp = "1704067200." + string(rune('0'+i)) + "00000000"
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(hcsResponse{Messages: messages})
	}))
	defer server.Close()

	ctx := context.Background()
	opts := FetchOptions{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}

	// Request only 5 messages
	data, err := FetchTimingWithOptions(ctx, "0.0.12345", Testnet, 5, opts)
	if err != nil {
		t.Fatalf("FetchTimingWithOptions() error = %v", err)
	}

	if data.MessageCount != 5 {
		t.Errorf("MessageCount = %d, want 5 (limited)", data.MessageCount)
	}
}

func TestDefaultFetchOptions(t *testing.T) {
	opts := DefaultFetchOptions()

	if opts.RequestDelay != 100*time.Millisecond {
		t.Errorf("RequestDelay = %v, want 100ms", opts.RequestDelay)
	}
	if opts.HTTPClient != http.DefaultClient {
		t.Error("HTTPClient should be http.DefaultClient")
	}
}

func TestParseConsensusTimestamp(t *testing.T) {
	tests := []struct {
		ts   string
		want float64
	}{
		{"1704067200.000000000", 1704067200.0},
		{"1704067200.500000000", 1704067200.5},
		{"1704067200.123456789", 1704067200.123456789},
		{"1704067200", 1704067200.0},
	}

	for _, tt := range tests {
		t.Run(tt.ts, func(t *testing.T) {
			got := parseConsensusTimestamp(tt.ts)
			if got != tt.want {
				t.Errorf("parseConsensusTimestamp(%q) = %f, want %f", tt.ts, got, tt.want)
			}
		})
	}
}

func TestCalculateInterArrivals(t *testing.T) {
	messages := []hcsMessage{
		{ConsensusTimestamp: "1704067200.000000000"},
		{ConsensusTimestamp: "1704067200.100000000"},
		{ConsensusTimestamp: "1704067200.350000000"},
		{ConsensusTimestamp: "1704067201.000000000"},
	}

	arrivals := calculateInterArrivals(messages)

	expected := []float64{100, 250, 650}
	if len(arrivals) != len(expected) {
		t.Fatalf("length = %d, want %d", len(arrivals), len(expected))
	}

	for i, want := range expected {
		if !floatEquals(arrivals[i], want) {
			t.Errorf("arrivals[%d] = %f, want ~%f", i, arrivals[i], want)
		}
	}
}

func TestCalculateInterArrivals_TooFewMessages(t *testing.T) {
	// Empty
	arrivals := calculateInterArrivals([]hcsMessage{})
	if arrivals != nil {
		t.Errorf("expected nil for empty messages, got %v", arrivals)
	}

	// Single message
	arrivals = calculateInterArrivals([]hcsMessage{
		{ConsensusTimestamp: "1704067200.000000000"},
	})
	if arrivals != nil {
		t.Errorf("expected nil for single message, got %v", arrivals)
	}
}
