// Package server builds the HTTP handler for the CareCircle server.
// Later tasks mount the MCP endpoint on this handler.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/rohitshukla001/AmazonDeveloperHackathon/internal/version"
)

// ShutdownTimeout is the maximum time that Serve waits for open requests
// to complete after the context is cancelled.
const ShutdownTimeout = 10 * time.Second

// Health is the response body of GET /healthz.
type Health struct {
	Status  string `json:"status"`
	Name    string `json:"name"`
	Version string `json:"version"`
}

// NewHandler returns the root HTTP handler.
func NewHandler(logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealth(logger))
	return mux
}

func handleHealth(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		err := json.NewEncoder(w).Encode(Health{
			Status:  "ok",
			Name:    version.Name,
			Version: version.Version,
		})
		if err != nil {
			logger.Warn("write health response", "err", err)
		}
	}
}

// Serve runs an HTTP server with handler on ln until ctx is cancelled.
// It then shuts the server down gracefully. Serve returns nil after a
// clean shutdown.
func Serve(ctx context.Context, ln net.Listener, handler http.Handler, logger *slog.Logger) error {
	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelWarn),
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()
	logger.Info("server listening", "addr", ln.Addr().String(), "version", version.Version)

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	logger.Info("server shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
