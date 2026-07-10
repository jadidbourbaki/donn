// Command donn runs the anonymous poll service. It reads the listen port from
// PORT and defaults to 8080. Polls live in memory and are seeded at startup, so
// the service is never empty.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jadidbourbaki/donn/internal/api"
	"github.com/jadidbourbaki/donn/internal/survey"
)

func main() {
	if err := run(); err != nil {
		slog.Error("donn exited", "error", err)
		os.Exit(1)
	}
}

func run() error {
	store := survey.NewStore()
	if err := store.Seed(); err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              ":" + port(),
		Handler:           api.NewServer(store),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		slog.Info("donn listening", "addr", srv.Addr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

func port() string {
	if p := os.Getenv("PORT"); p != "" {
		return p
	}
	return "8080"
}
