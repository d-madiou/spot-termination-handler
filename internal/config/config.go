package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all runtime configuration for the handler
type Config struct {
	MetadataURL  string
	NodeName     string
	PollInterval time.Duration
	DrainTimeout time.Duration
	GracePeriod  time.Duration
	LogLevel     string
	WebhookURL   string
}

// Load reads configuration from environment variables with sensible defaults
func Load() *Config {
	return &Config{
		MetadataURL:  getEnv("METADATA_URL", "http://169.254.169.254"),
		NodeName:     getEnv("NODE_NAME", ""),
		PollInterval: getDurationEnv("POLL_INTERVAL_SECONDS", 5*time.Second),
		DrainTimeout: getDurationEnv("DRAIN_TIMEOUT_SECONDS", 120*time.Second),
		GracePeriod:  getDurationEnv("GRACE_PERIOD_SECONDS", 90*time.Second),
		LogLevel:     getEnv("LOG_LEVEL", "info"),
		WebhookURL:   getEnv("WEBHOOK_URL", ""),
	}
}

// Validate checks that all required fields are present and values are sane
func (c *Config) Validate() error {
	if c.NodeName == "" {
		return fmt.Errorf("NODE_NAME is required but not set")
	}
	if c.PollInterval < 1*time.Second {
		return fmt.Errorf("POLL_INTERVAL_SECONDS must be at least 1 second")
	}
	if c.DrainTimeout < 10*time.Second {
		return fmt.Errorf("DRAIN_TIMEOUT_SECONDS must be at least 10 seconds")
	}
	if c.GracePeriod >= c.DrainTimeout {
		return fmt.Errorf("GRACE_PERIOD_SECONDS must be less than DRAIN_TIMEOUT_SECONDS")
	}
	return nil
}

// getEnv returns the value of an env var or a fallback default
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// getDurationEnv parses an env var as seconds into a time.Duration
// Falls back to defaultVal if the var is missing or unparseable
func getDurationEnv(key string, defaultVal time.Duration) time.Duration {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	seconds, err := strconv.Atoi(val)
	if err != nil || seconds <= 0 {
		return defaultVal
	}
	return time.Duration(seconds) * time.Second
}
