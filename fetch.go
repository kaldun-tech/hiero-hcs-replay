package hcsreplay

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Network represents a Hedera network.
type Network string

const (
	// Mainnet is the Hedera mainnet.
	Mainnet Network = "mainnet"

	// Testnet is the Hedera testnet.
	Testnet Network = "testnet"

	// Previewnet is the Hedera previewnet.
	Previewnet Network = "previewnet"
)

// MirrorNodeURL returns the public mirror node URL for the network.
func (n Network) MirrorNodeURL() string {
	switch n {
	case Mainnet:
		return "https://mainnet-public.mirrornode.hedera.com"
	case Testnet:
		return "https://testnet.mirrornode.hedera.com"
	case Previewnet:
		return "https://previewnet.mirrornode.hedera.com"
	default:
		return ""
	}
}

// FetchOptions configures the FetchTiming operation.
type FetchOptions struct {
	// BaseURL overrides the default mirror node URL.
	// If empty, the public mirror node for the network is used.
	BaseURL string

	// RequestDelay is the delay between paginated API requests.
	// Defaults to 100ms if zero.
	RequestDelay time.Duration

	// HTTPClient is the HTTP client to use.
	// Defaults to http.DefaultClient if nil.
	HTTPClient *http.Client

	// OnProgress is called after each batch of messages is fetched.
	// The parameter is the total number of messages fetched so far.
	OnProgress func(fetched int)
}

// DefaultFetchOptions returns the default fetch options.
func DefaultFetchOptions() FetchOptions {
	return FetchOptions{
		RequestDelay: 100 * time.Millisecond,
		HTTPClient:   http.DefaultClient,
	}
}

// hcsMessage represents a message from the mirror node API.
type hcsMessage struct {
	ConsensusTimestamp string `json:"consensus_timestamp"`
	Message            string `json:"message"`
	SequenceNumber     int64  `json:"sequence_number"`
}

// hcsResponse represents the mirror node API response.
type hcsResponse struct {
	Messages []hcsMessage `json:"messages"`
	Links    struct {
		Next string `json:"next"`
	} `json:"links"`
}

// FetchTiming fetches message timing data from an HCS topic via the Hedera Mirror Node REST API.
//
// Parameters:
//   - ctx: Context for cancellation
//   - topicID: HCS topic ID (e.g., "0.0.120438")
//   - network: Hedera network (Mainnet, Testnet, or Previewnet)
//   - limit: Maximum number of messages to fetch
//
// Returns timing data that can be used with NewReplay.
func FetchTiming(ctx context.Context, topicID string, network Network, limit int) (*TimingData, error) {
	return FetchTimingWithOptions(ctx, topicID, network, limit, DefaultFetchOptions())
}

// FetchTimingWithOptions fetches message timing data with custom options.
func FetchTimingWithOptions(ctx context.Context, topicID string, network Network, limit int, opts FetchOptions) (*TimingData, error) {
	baseURL := opts.BaseURL
	if baseURL == "" {
		baseURL = network.MirrorNodeURL()
	}
	if baseURL == "" {
		return nil, fmt.Errorf("unknown network: %s", network)
	}

	if opts.RequestDelay == 0 {
		opts.RequestDelay = 100 * time.Millisecond
	}
	if opts.HTTPClient == nil {
		opts.HTTPClient = http.DefaultClient
	}

	messages, err := fetchMessages(ctx, opts.HTTPClient, baseURL, topicID, limit, opts.RequestDelay, opts.OnProgress)
	if err != nil {
		return nil, err
	}

	if len(messages) < 2 {
		return nil, fmt.Errorf("not enough messages (%d) to calculate timing", len(messages))
	}

	interArrivals := calculateInterArrivals(messages)
	stats := CalculateStats(interArrivals)

	timestamps := make([]float64, len(messages))
	for i, m := range messages {
		timestamps[i] = parseConsensusTimestamp(m.ConsensusTimestamp)
	}
	sort.Float64s(timestamps)
	timeSpan := timestamps[len(timestamps)-1] - timestamps[0]

	var avgRate float64
	if timeSpan > 0 {
		avgRate = float64(len(messages)) / timeSpan
	}

	return &TimingData{
		TopicID:          topicID,
		Network:          string(network),
		MessageCount:     len(messages),
		TimeSpanSeconds:  timeSpan,
		AvgRatePerSecond: avgRate,
		InterArrivalMs:   interArrivals,
		Stats:            stats,
	}, nil
}

func fetchMessages(ctx context.Context, client *http.Client, baseURL, topicID string, limit int, delay time.Duration, onProgress func(int)) ([]hcsMessage, error) {
	var messages []hcsMessage
	url := fmt.Sprintf("%s/api/v1/topics/%s/messages?limit=100&order=asc", baseURL, topicID)

	for url != "" && len(messages) < limit {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch messages: %w", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			resp.Body.Close()
			return nil, fmt.Errorf("topic %s not found or has no messages", topicID)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("mirror node returned status %d", resp.StatusCode)
		}

		var hcsResp hcsResponse
		if err := json.NewDecoder(resp.Body).Decode(&hcsResp); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		resp.Body.Close()

		if len(hcsResp.Messages) == 0 {
			break
		}

		messages = append(messages, hcsResp.Messages...)

		if onProgress != nil {
			onProgress(len(messages))
		}

		if hcsResp.Links.Next != "" {
			url = baseURL + hcsResp.Links.Next
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		} else {
			url = ""
		}
	}

	if len(messages) > limit {
		messages = messages[:limit]
	}

	return messages, nil
}

func parseConsensusTimestamp(ts string) float64 {
	parts := strings.Split(ts, ".")
	seconds, _ := strconv.ParseInt(parts[0], 10, 64)
	var nanos int64
	if len(parts) > 1 {
		nanos, _ = strconv.ParseInt(parts[1], 10, 64)
	}
	return float64(seconds) + float64(nanos)/1e9
}

func calculateInterArrivals(messages []hcsMessage) []float64 {
	if len(messages) < 2 {
		return nil
	}

	timestamps := make([]float64, len(messages))
	for i, m := range messages {
		timestamps[i] = parseConsensusTimestamp(m.ConsensusTimestamp)
	}
	sort.Float64s(timestamps)

	interArrivals := make([]float64, len(timestamps)-1)
	for i := 1; i < len(timestamps); i++ {
		deltaMs := (timestamps[i] - timestamps[i-1]) * 1000
		interArrivals[i-1] = deltaMs
	}

	return interArrivals
}
