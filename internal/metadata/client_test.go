package metadata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/d-madiou/spot-termination-handler/internal/config"
)

// helper: builds a Client pointed at a test server URL
func newTestClient(serverURL string) *Client {
	cfg := &config.Config{
		MetadataURL:  serverURL,
		PollInterval: 50 * time.Millisecond,
	}
	return NewClient(cfg)
}

// helper: spins up a mock metadata server that always returns the given status
func mockServer(statusCode int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
	}))
}

func TestCheckTermination_Safe(t *testing.T) {
	server := mockServer(http.StatusNotFound)
	defer server.Close()

	client := newTestClient(server.URL)
	noticed, err := client.CheckTermination()

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if noticed {
		t.Error("expected false (safe), got true")
	}
}

func TestCheckTermination_Noticed(t *testing.T) {
	server := mockServer(http.StatusOK)
	defer server.Close()

	client := newTestClient(server.URL)
	noticed, err := client.CheckTermination()

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if !noticed {
		t.Error("expected true (termination noticed), got false")
	}
}

func TestCheckTermination_UnexpectedStatus(t *testing.T) {
	server := mockServer(http.StatusInternalServerError)
	defer server.Close()

	client := newTestClient(server.URL)
	noticed, err := client.CheckTermination()

	if err == nil {
		t.Error("expected error for unexpected status code, got nil")
	}
	if noticed {
		t.Error("expected false on unexpected status, got true")
	}
}

func TestCheckTermination_NetworkError(t *testing.T) {
	// point at a port nothing is listening on
	client := newTestClient("http://localhost:19999")
	noticed, err := client.CheckTermination()

	if err == nil {
		t.Error("expected error for network failure, got nil")
	}
	if noticed {
		t.Error("expected false on network error, got true")
	}
}

func TestStartPolling_DetectsTermination(t *testing.T) {
	callCount := 0

	// returns 404 twice, then 200 on the third call
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount >= 3 {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	ch := make(chan TerminationNotice, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go client.StartPolling(ctx, ch)

	select {
	case notice := <-ch:
		if notice.DetectedAt.IsZero() {
			t.Error("expected DetectedAt to be set")
		}
	case <-ctx.Done():
		t.Error("expected termination notice before timeout")
	}
}

func TestStartPolling_ContextCancel(t *testing.T) {
	// server always returns 404 — termination never fires
	server := mockServer(http.StatusNotFound)
	defer server.Close()

	client := newTestClient(server.URL)
	ch := make(chan TerminationNotice, 1)
	ctx, cancel := context.WithCancel(context.Background())

	go client.StartPolling(ctx, ch)

	// cancel after a short time
	time.Sleep(150 * time.Millisecond)
	cancel()

	// give goroutine a moment to exit
	time.Sleep(100 * time.Millisecond)

	select {
	case <-ch:
		t.Error("expected no termination notice after context cancel")
	default:
		// correct — nothing on the channel
	}
}

func TestStartPolling_NetworkErrorContinues(t *testing.T) {
	callCount := 0

	// first call fails (connection refused style via 500), second returns 200
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	ch := make(chan TerminationNotice, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go client.StartPolling(ctx, ch)

	select {
	case <-ch:
		// correct — recovered from error and detected termination
	case <-ctx.Done():
		t.Error("expected termination notice after error recovery")
	}
}
