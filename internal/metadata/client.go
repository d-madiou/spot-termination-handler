package metadata

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/d-madiou/spot-termination-handler/internal/config"
)

// TerminationNotice is sent on the channel when a termination is detected
type TerminationNotice struct {
	DetectedAt time.Time
}

// Client polls the EC2 metadata endpoint for spot termination notices
type Client struct {
	httpClient   *http.Client
	baseURL      string
	pollInterval time.Duration
}

// NewClient constructs a metadata Client from config
func NewClient(cfg *config.Config) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 3 * time.Second, // must be well under pollInterval
		},
		baseURL:      cfg.MetadataURL,
		pollInterval: cfg.PollInterval,
	}
}

// CheckTermination performs a single poll against the metadata endpoint.
// Returns true if a termination notice is active, false otherwise.
func (c *Client) CheckTermination() (bool, error) {
	url := fmt.Sprintf("%s/latest/meta-data/spot/termination-time", c.baseURL)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return false, fmt.Errorf("metadata poll failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// 200 → termination notice is active
		return true, nil
	case http.StatusNotFound:
		// 404 → no termination notice, node is safe
		return false, nil
	default:
		// anything else is unexpected but non-fatal
		return false, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
}

// StartPolling polls on every PollInterval tick until either:
//   - a termination notice is detected → sends TerminationNotice on ch
//   - the context is cancelled         → returns cleanly
func (c *Client) StartPolling(ctx context.Context, ch chan<- TerminationNotice) {
	ticker := time.NewTicker(c.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// context cancelled, shut down cleanly
			return
		case <-ticker.C:
			noticed, err := c.CheckTermination()
			if err != nil {
				// log and continue — a single poll failure is not fatal
				fmt.Printf("[metadata] poll error (will retry): %v\n", err)
				continue
			}
			if noticed {
				ch <- TerminationNotice{DetectedAt: time.Now()}
				return
			}
		}
	}
}
