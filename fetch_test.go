package hcsreplay

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Test constants for topic IDs and timestamps.
const (
	fetchTestTopicID = "0.0.12345"

	// Base timestamp: 2024-01-01 00:00:00 UTC
	tsBase   = "1704067200.000000000"
	ts100ms  = "1704067200.100000000"
	ts200ms  = "1704067200.200000000"
	ts250ms  = "1704067200.250000000"
	ts300ms  = "1704067200.300000000"
	ts350ms  = "1704067200.350000000"
	ts500ms  = "1704067200.500000000"
	ts1000ms = "1704067201.000000000"

	// Error message format for FetchTimingWithOptions tests
	errFormat = "FetchTimingWithOptions() error = %v"
)

// floatEquals checks if two floats are approximately equal (within 0.001).
func floatEquals(a, b float64) bool {
	return math.Abs(a-b) < 0.001
}

// encodeJSONResponse writes a JSON response, failing the test on error.
func encodeJSONResponse(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Errorf("failed to encode response: %v", err)
	}
}

// newTestServer creates an httptest.Server that responds with the given messages.
func newTestServer(t *testing.T, messages []hcsMessage) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		encodeJSONResponse(t, w, hcsResponse{Messages: messages})
	}))
}

// newTestFetchOptions creates FetchOptions configured for the given test server.
func newTestFetchOptions(server *httptest.Server) FetchOptions {
	return FetchOptions{
		BaseURL:      server.URL,
		RequestDelay: 1 * time.Millisecond,
		HTTPClient:   server.Client(),
	}
}

func TestNetworkMirrorNodeURL(t *testing.T) {
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
	messages := []hcsMessage{
		{ConsensusTimestamp: tsBase, SequenceNumber: 1},
		{ConsensusTimestamp: ts100ms, SequenceNumber: 2},
		{ConsensusTimestamp: ts250ms, SequenceNumber: 3},
		{ConsensusTimestamp: ts500ms, SequenceNumber: 4},
		{ConsensusTimestamp: ts1000ms, SequenceNumber: 5},
	}

	server := newTestServer(t, messages)
	defer server.Close()

	ctx := context.Background()
	opts := newTestFetchOptions(server)

	data, err := FetchTimingWithOptions(ctx, fetchTestTopicID, Testnet, 100, opts)
	if err != nil {
		t.Fatalf(errFormat, err)
	}

	if data.TopicID != fetchTestTopicID {
		t.Errorf("TopicID = %q, want %q", data.TopicID, fetchTestTopicID)
	}
	if data.MessageCount != 5 {
		t.Errorf("MessageCount = %d, want 5", data.MessageCount)
	}
	if len(data.InterArrivalMs) != 4 {
		t.Errorf("InterArrivalMs length = %d, want 4", len(data.InterArrivalMs))
	}

	// Verify inter-arrival times (in ms)
	expected := []float64{100, 150, 250, 500}
	for i, want := range expected {
		if !floatEquals(data.InterArrivalMs[i], want) {
			t.Errorf("InterArrivalMs[%d] = %f, want ~%f", i, data.InterArrivalMs[i], want)
		}
	}
}

func TestFetchTimingWithOptionsPagination(t *testing.T) {
	page1Messages := []hcsMessage{
		{ConsensusTimestamp: tsBase, SequenceNumber: 1},
		{ConsensusTimestamp: ts100ms, SequenceNumber: 2},
	}
	page2Messages := []hcsMessage{
		{ConsensusTimestamp: ts200ms, SequenceNumber: 3},
		{ConsensusTimestamp: ts300ms, SequenceNumber: 4},
	}

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		var resp hcsResponse
		if requestCount == 1 {
			resp = hcsResponse{
				Messages: page1Messages,
				Links:    struct{ Next string `json:"next"` }{Next: "/api/v1/topics/0.0.12345/messages?timestamp=gt:" + ts100ms},
			}
		} else {
			resp = hcsResponse{Messages: page2Messages}
		}
		encodeJSONResponse(t, w, resp)
	}))
	defer server.Close()

	ctx := context.Background()
	opts := newTestFetchOptions(server)

	data, err := FetchTimingWithOptions(ctx, fetchTestTopicID, Testnet, 100, opts)
	if err != nil {
		t.Fatalf(errFormat, err)
	}

	if data.MessageCount != 4 {
		t.Errorf("MessageCount = %d, want 4", data.MessageCount)
	}
	if requestCount != 2 {
		t.Errorf("requestCount = %d, want 2", requestCount)
	}
}

func TestFetchTimingWithOptionsErrors(t *testing.T) {
	t.Run("NotFound", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		ctx := context.Background()
		opts := FetchOptions{
			BaseURL:    server.URL,
			HTTPClient: server.Client(),
		}

		_, err := FetchTimingWithOptions(ctx, fetchTestTopicID, Testnet, 100, opts)
		if err == nil {
			t.Error("FetchTimingWithOptions() expected error for 404, got nil")
		}
		if !errors.Is(err, ErrTopicNotFound) {
			t.Errorf("error should wrap ErrTopicNotFound, got: %v", err)
		}
	})

	t.Run("NotEnoughMessages", func(t *testing.T) {
		messages := []hcsMessage{
			{ConsensusTimestamp: tsBase, SequenceNumber: 1},
		}

		server := newTestServer(t, messages)
		defer server.Close()

		ctx := context.Background()
		opts := FetchOptions{
			BaseURL:    server.URL,
			HTTPClient: server.Client(),
		}

		_, err := FetchTimingWithOptions(ctx, fetchTestTopicID, Testnet, 100, opts)
		if err == nil {
			t.Error("FetchTimingWithOptions() expected error for < 2 messages, got nil")
		}
		if !errors.Is(err, ErrNotEnoughMessages) {
			t.Errorf("error should wrap ErrNotEnoughMessages, got: %v", err)
		}
	})

	t.Run("InvalidTopicID", func(t *testing.T) {
		ctx := context.Background()
		opts := FetchOptions{}

		invalidIDs := []string{
			"invalid",
			"0.0",
			"0.0.abc",
			"",
			"0.0.12345.extra",
		}

		for _, topicID := range invalidIDs {
			_, err := FetchTimingWithOptions(ctx, topicID, Testnet, 100, opts)
			if err == nil {
				t.Errorf("FetchTimingWithOptions(%q) expected error, got nil", topicID)
			}
			if !errors.Is(err, ErrInvalidTopicID) {
				t.Errorf("FetchTimingWithOptions(%q) error should wrap ErrInvalidTopicID, got: %v", topicID, err)
			}
		}
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond)
			encodeJSONResponse(t, w, hcsResponse{})
		}))
		defer server.Close()

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		opts := FetchOptions{
			BaseURL:    server.URL,
			HTTPClient: server.Client(),
		}

		_, err := FetchTimingWithOptions(ctx, fetchTestTopicID, Testnet, 100, opts)
		if err == nil {
			t.Error("FetchTimingWithOptions() expected error for cancelled context, got nil")
		}
	})

	t.Run("UnknownNetwork", func(t *testing.T) {
		ctx := context.Background()
		opts := FetchOptions{} // No BaseURL override

		_, err := FetchTimingWithOptions(ctx, fetchTestTopicID, Network("invalid"), 100, opts)
		if err == nil {
			t.Error("FetchTimingWithOptions() expected error for unknown network, got nil")
		}
	})
}

func TestFetchTimingWithOptionsOnProgress(t *testing.T) {
	messages := []hcsMessage{
		{ConsensusTimestamp: tsBase, SequenceNumber: 1},
		{ConsensusTimestamp: ts100ms, SequenceNumber: 2},
		{ConsensusTimestamp: ts200ms, SequenceNumber: 3},
	}

	server := newTestServer(t, messages)
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

	_, err := FetchTimingWithOptions(ctx, fetchTestTopicID, Testnet, 100, opts)
	if err != nil {
		t.Fatalf(errFormat, err)
	}

	if len(progressCalls) != 1 {
		t.Errorf("progressCalls = %d, want 1", len(progressCalls))
	}
	if progressCalls[0] != 3 {
		t.Errorf("progressCalls[0] = %d, want 3", progressCalls[0])
	}
}

func TestFetchTimingWithOptionsLimit(t *testing.T) {
	messages := make([]hcsMessage, 10)
	for i := range messages {
		messages[i] = hcsMessage{
			ConsensusTimestamp: "1704067200.000000000",
			SequenceNumber:     int64(i + 1),
		}
		// Add 100ms between each message
		messages[i].ConsensusTimestamp = "1704067200." + string(rune('0'+i)) + "00000000"
	}

	server := newTestServer(t, messages)
	defer server.Close()

	ctx := context.Background()
	opts := FetchOptions{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}

	// Request only 5 messages
	data, err := FetchTimingWithOptions(ctx, fetchTestTopicID, Testnet, 5, opts)
	if err != nil {
		t.Fatalf(errFormat, err)
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
	if opts.HTTPClient == nil {
		t.Error("HTTPClient should not be nil")
	}
	if opts.HTTPClient.Timeout != 30*time.Second {
		t.Errorf("HTTPClient.Timeout = %v, want 30s", opts.HTTPClient.Timeout)
	}
}

func TestParseConsensusTimestamp(t *testing.T) {
	tests := []struct {
		ts   string
		want float64
	}{
		{tsBase, 1704067200.0},
		{ts500ms, 1704067200.5},
		{"1704067200.123456789", 1704067200.123456789}, // unique test value
		{"1704067200", 1704067200.0},                   // no fractional part
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
	t.Run("Normal", func(t *testing.T) {
		messages := []hcsMessage{
			{ConsensusTimestamp: tsBase},
			{ConsensusTimestamp: ts100ms},
			{ConsensusTimestamp: ts350ms},
			{ConsensusTimestamp: ts1000ms},
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
	})

	t.Run("TooFewMessages", func(t *testing.T) {
		// Empty
		arrivals := calculateInterArrivals([]hcsMessage{})
		if arrivals != nil {
			t.Errorf("expected nil for empty messages, got %v", arrivals)
		}

		// Single message
		arrivals = calculateInterArrivals([]hcsMessage{
			{ConsensusTimestamp: tsBase},
		})
		if arrivals != nil {
			t.Errorf("expected nil for single message, got %v", arrivals)
		}
	})
}
