// Package retrier small utilities for retrier.
package retrier

import (
	"context"
	"errors"
	"math/rand"
	"time"
)

// ErrNonRetriableError non retriable error to use and avoid retries.
var ErrNonRetriableError = errors.New("non retriable error")

type RetriableFunc func(ctx context.Context) error

type Retrier struct {
	MaxAttempts int
	BackOffTime time.Duration
}

func NewRetrier(maxAttempts int, backOffTime time.Duration) *Retrier {
	return &Retrier{
		MaxAttempts: maxAttempts,
		BackOffTime: backOffTime,
	}
}

func (r *Retrier) Do(ctx context.Context, retriableFunc RetriableFunc) error {
	retryCount := 0
	err := handleRetriableError(retriableFunc(ctx))
	if err == nil {
		return nil
	}

	retryCount++
	if retryCount > r.MaxAttempts {
		return err
	}

	jitter := time.Duration(rand.Int63n(int64(r.BackOffTime) / 2)) //nolint:gosec // We don't need a cryptographically secure random number here.
	ticker := time.NewTicker(r.BackOffTime + jitter)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			err = handleRetriableError(retriableFunc(ctx))
			if err == nil {
				return nil
			}
			retryCount++
			if retryCount > r.MaxAttempts {
				return err
			}
		}
	}
}

func handleRetriableError(err error) error {
	switch {
	case errors.Is(err, ErrNonRetriableError):
		return nil
	case err != nil:
		return err
	default:
		return nil
	}
}
