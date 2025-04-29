package util

import (
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"time"
)

// RetryConfig configures retry behavior
type RetryConfig struct {
	MaxAttempts int
	Label       string
}

// WithRetry attempts to execute a function with retries
func WithRetry[T any](config RetryConfig, fn func() (T, error)) (T, error) {
	var zero T

	if config.MaxAttempts == 0 {
		config.MaxAttempts = 3
	}

	if config.Label == "" {
		config.Label = "__unnamed__"
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil)).With("component", fmt.Sprintf("retry-%s", config.Label))

	var lastErr error
	for i := 1; i <= config.MaxAttempts; i++ {
		result, err := fn()
		if err == nil {
			return result, nil
		}

		lastErr = err

		if i < config.MaxAttempts {
			backoff := 10 * time.Second
			logger.Info(fmt.Sprintf("Attempt %d failed. Retry in %.0f seconds", i, backoff.Seconds()), "error", err)
			time.Sleep(backoff)
		}
	}

	return zero, fmt.Errorf("withRetry(label=%s): Retry exhausted: %w", config.Label, lastErr)
}

// Result represents a computation that can succeed or fail
type Result[T any] struct {
	Value   T
	Err     string
	Success bool
}

// Ok creates a successful result
func Ok[T any](value T) Result[T] {
	return Result[T]{
		Value:   value,
		Success: true,
	}
}

// Err creates a failed result
func Err[T any](err string) Result[T] {
	var zero T
	return Result[T]{
		Value:   zero,
		Err:     err,
		Success: false,
	}
}

// ReplaceImageMarkdown replaces markdown image syntax with just the alt text
func ReplaceImageMarkdown(content string) string {
	re := regexp.MustCompile(`!?\[([^\]]+)\]\([^)]+\)`)
	return re.ReplaceAllString(content, "$1")
}
