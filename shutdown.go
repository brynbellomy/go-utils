package utils

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"time"
)

type ContextCloser interface {
	Close(ctx context.Context) error
}

func KillGracefullyOnInterrupt(gracePeriod time.Duration, fn func(ctx context.Context) []ContextCloser) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	closers := fn(ctx)

	<-ctx.Done()
	stop()
	slog.Info("shutting down gracefully, press Ctrl+C again to force")

	// Perform application shutdown with the specified grace period
	timeoutCtx, cancel := context.WithTimeout(context.Background(), gracePeriod)
	defer cancel()

	for _, closer := range closers {
		go func() {
			defer cancel()
			if err := closer.Close(timeoutCtx); err != nil {
				slog.Error("could not close gracefully", "err", err)
			}
		}()
	}

	select {
	case <-timeoutCtx.Done():
		if timeoutCtx.Err() == context.DeadlineExceeded {
			slog.Error("timeout exceeded, forcing shutdown")
			os.Exit(-1)
		}
	}
}
