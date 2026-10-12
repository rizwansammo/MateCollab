package httpx

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// NewServer returns an http.Server with conservative timeouts.
func NewServer(addr string, h http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    64 << 10,
	}
}

// Serve runs the servers until ctx is cancelled, then shuts them down gracefully. It returns the
// first listener error, if any.
func Serve(ctx context.Context, logger *slog.Logger, servers ...*http.Server) error {
	errc := make(chan error, len(servers))
	for _, srv := range servers {
		ln, err := net.Listen("tcp", srv.Addr)
		if err != nil {
			return err
		}
		logger.Info("listening", slog.String("addr", ln.Addr().String()))
		go func(srv *http.Server, ln net.Listener) {
			if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errc <- err
			}
		}(srv, ln)
	}

	var runErr error
	select {
	case <-ctx.Done():
	case runErr = <-errc:
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancel()
	for _, srv := range servers {
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Warn("graceful shutdown incomplete", slog.String("addr", srv.Addr), slog.Any("error", err))
		}
	}
	return runErr
}
