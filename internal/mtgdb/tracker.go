package mtgdb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

// ProgressTracker provides velocity and completion estimates for stream-based operations.
type ProgressTracker struct {
	Total     float64
	StartTime time.Time
}

// NewTracker initializes a tracker with the total expected units.
func NewTracker(total float64) *ProgressTracker {
	return &ProgressTracker{
		Total:     total,
		StartTime: time.Now(),
	}
}

// GetETA calculates the progress ratio and a human-readable remaining time string.
func (t *ProgressTracker) GetETA(current float64) (float64, string) {
	if current <= 0 || t.Total <= 0 {
		return 0.0, "ETA: --m --s"
	}

	progress := current / t.Total
	elapsed := time.Since(t.StartTime).Seconds()

	if elapsed <= 0.001 {
		elapsed = 0.001
	}

	rate := current / elapsed
	remaining := t.Total - current
	etaSeconds := remaining / rate

	eta := time.Duration(etaSeconds) * time.Second
	minutes := int(eta.Minutes())
	seconds := int(eta.Seconds()) % 60

	return progress, fmt.Sprintf("ETA: %02dm %02ds", minutes, seconds)
}

// CreateCancelContext returns a context that cancels when the provided channel is closed.
// It uses a single monitor goroutine that cleans up after itself to prevent leaks.
func CreateCancelContext(cancelChan <-chan struct{}) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		select {
		case <-cancelChan:
			log.Debug().Msg("external cancellation signal received")
			cancel()
		case <-ctx.Done():
			// Context was cancelled elsewhere or finished normally
		}
	}()

	return ctx, cancel
}

// IsRootCancellation determines if an error was caused by a context timeout or user abort.
func IsRootCancellation(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}