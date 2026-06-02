package config

import (
	"os"
	"testing"
	"time"
)

// helper: set env vars for a test, restore originals after
func setEnv(t *testing.T, vars map[string]string) {
	t.Helper()
	for key, val := range vars {
		original, exists := os.LookupEnv(key)
		os.Setenv(key, val)
		t.Cleanup(func() {
			if exists {
				os.Setenv(key, original)
			} else {
				os.Unsetenv(key)
			}
		})
	}
}

// ── Load() Tests ─────────────────────────────────────────────────────────────

func TestLoad_Defaults(t *testing.T) {
	// Ensure none of these are set in the environment
	for _, key := range []string{
		"METADATA_URL", "NODE_NAME", "POLL_INTERVAL_SECONDS",
		"DRAIN_TIMEOUT_SECONDS", "GRACE_PERIOD_SECONDS",
		"LOG_LEVEL", "WEBHOOK_URL",
	} {
		os.Unsetenv(key)
	}

	cfg := Load()

	if cfg.MetadataURL != "http://169.254.169.254" {
		t.Errorf("expected default MetadataURL, got %s", cfg.MetadataURL)
	}
	if cfg.NodeName != "" {
		t.Errorf("expected empty NodeName by default, got %s", cfg.NodeName)
	}
	if cfg.PollInterval != 5*time.Second {
		t.Errorf("expected 5s PollInterval, got %s", cfg.PollInterval)
	}
	if cfg.DrainTimeout != 120*time.Second {
		t.Errorf("expected 120s DrainTimeout, got %s", cfg.DrainTimeout)
	}
	if cfg.GracePeriod != 90*time.Second {
		t.Errorf("expected 90s GracePeriod, got %s", cfg.GracePeriod)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("expected default LogLevel 'info', got %s", cfg.LogLevel)
	}
	if cfg.WebhookURL != "" {
		t.Errorf("expected empty WebhookURL by default, got %s", cfg.WebhookURL)
	}
}

func TestLoad_Overrides(t *testing.T) {
	setEnv(t, map[string]string{
		"METADATA_URL":          "http://localhost:8080",
		"NODE_NAME":             "worker-node-1",
		"POLL_INTERVAL_SECONDS": "10",
		"DRAIN_TIMEOUT_SECONDS": "180",
		"GRACE_PERIOD_SECONDS":  "60",
		"LOG_LEVEL":             "debug",
		"WEBHOOK_URL":           "https://hooks.slack.com/test",
	})

	cfg := Load()

	if cfg.MetadataURL != "http://localhost:8080" {
		t.Errorf("expected overridden MetadataURL, got %s", cfg.MetadataURL)
	}
	if cfg.NodeName != "worker-node-1" {
		t.Errorf("expected overridden NodeName, got %s", cfg.NodeName)
	}
	if cfg.PollInterval != 10*time.Second {
		t.Errorf("expected 10s PollInterval, got %s", cfg.PollInterval)
	}
	if cfg.DrainTimeout != 180*time.Second {
		t.Errorf("expected 180s DrainTimeout, got %s", cfg.DrainTimeout)
	}
	if cfg.GracePeriod != 60*time.Second {
		t.Errorf("expected 60s GracePeriod, got %s", cfg.GracePeriod)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected overridden LogLevel, got %s", cfg.LogLevel)
	}
	if cfg.WebhookURL != "https://hooks.slack.com/test" {
		t.Errorf("expected overridden WebhookURL, got %s", cfg.WebhookURL)
	}
}

func TestLoad_InvalidDuration_FallsBackToDefault(t *testing.T) {
	setEnv(t, map[string]string{
		"POLL_INTERVAL_SECONDS": "not-a-number",
		"DRAIN_TIMEOUT_SECONDS": "-5",
	})

	cfg := Load()

	if cfg.PollInterval != 5*time.Second {
		t.Errorf("expected fallback to 5s PollInterval, got %s", cfg.PollInterval)
	}
	if cfg.DrainTimeout != 120*time.Second {
		t.Errorf("expected fallback to 120s DrainTimeout, got %s", cfg.DrainTimeout)
	}
}

// ── Validate() Tests ──────────────────────────────────────────────────────────

func TestValidate_NoNodeName(t *testing.T) {
	cfg := Load()
	cfg.NodeName = ""

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for empty NodeName, got nil")
	}
}

func TestValidate_PollIntervalTooShort(t *testing.T) {
	cfg := Load()
	cfg.NodeName = "worker-node-1"
	cfg.PollInterval = 500 * time.Millisecond

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for PollInterval < 1s, got nil")
	}
}

func TestValidate_DrainTimeoutTooShort(t *testing.T) {
	cfg := Load()
	cfg.NodeName = "worker-node-1"
	cfg.DrainTimeout = 5 * time.Second

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for DrainTimeout < 10s, got nil")
	}
}

func TestValidate_GracePeriodExceedsDrainTimeout(t *testing.T) {
	cfg := Load()
	cfg.NodeName = "worker-node-1"
	cfg.DrainTimeout = 60 * time.Second
	cfg.GracePeriod = 90 * time.Second

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error when GracePeriod >= DrainTimeout, got nil")
	}
}

func TestValidate_Valid(t *testing.T) {
	cfg := &Config{
		MetadataURL:  "http://169.254.169.254",
		NodeName:     "worker-node-1",
		PollInterval: 5 * time.Second,
		DrainTimeout: 120 * time.Second,
		GracePeriod:  90 * time.Second,
		LogLevel:     "info",
		WebhookURL:   "",
	}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("expected no error for valid config, got: %v", err)
	}
}
