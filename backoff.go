package utils

import (
	"context"
	"math"
	"math/rand"
	"time"

	"github.com/brynbellomy/go-utils/errors"
)

var ErrAllRetryAttemptsFailed = errors.New("all retry attempts failed")

func ExponentialBackoff(
	ctx context.Context,
	attempts int,
	baseDelay time.Duration,
	maxDelay time.Duration,
	fn func(context.Context) error,
) error {
	var err error
	for i := range attempts {
		err = fn(ctx)
		if err == nil {
			return nil
		}

		// Calculate delay with jitter
		exp := math.Pow(2, float64(i))
		jitter := time.Duration(rand.Int63n(int64(baseDelay)))
		delay := time.Duration(exp) * baseDelay
		if delay > maxDelay {
			delay = maxDelay
		}
		delay += jitter

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}

	return errors.WithCause(ErrAllRetryAttemptsFailed, err)
}
