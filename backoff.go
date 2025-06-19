package utils

import (
	"math"
	"math/rand"
	"time"

	"github.com/pkg/errors"
)

var ErrAllRetryAttemptsFailed = errors.New("all retry attempts failed")

func ExponentialBackoff(
	attempts int,
	baseDelay time.Duration,
	maxDelay time.Duration,
	fn func() error,
) error {
	for i := 0; i < attempts; i++ {
		err := fn()
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
		time.Sleep(delay)
	}

	return ErrAllRetryAttemptsFailed
}
