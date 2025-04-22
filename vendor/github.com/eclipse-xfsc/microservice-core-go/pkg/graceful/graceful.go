package graceful

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Shutdown gracefully stops the given HTTP server on
// receiving a stop signal or context cancellation signal
// and waits for the active connections to be closed
// for {timeout} period of time.
//
// The {timeout} period is respected in both stop conditions.
func Shutdown(ctx context.Context, srv *http.Server, timeout time.Duration) error {
	done := make(chan error, 1)
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)

		// wait for a signal or context cancellation
		select {
		case <-c:
		case <-ctx.Done():
		}

		ctx := context.Background()
		var cancel context.CancelFunc
		if timeout > 0 {
			ctx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}

		done <- srv.Shutdown(ctx)
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}

	return <-done
}
