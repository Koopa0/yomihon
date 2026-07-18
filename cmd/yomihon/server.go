package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

const maxRequestHeaderBytes = 64 << 10

const shutdownTimeout = 5 * time.Second

func newHTTPServer(handler http.Handler) *http.Server {
	return &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    maxRequestHeaderBytes,
	}
}

// serveHTTP runs srv until cancellation or a listener failure. Both paths
// attempt bounded graceful shutdown. The readingSite separately owns accepted
// request lifetimes, so its deferred close still waits for a handler if this
// bounded attempt reaches its deadline.
func serveHTTP(ctx context.Context, srv *http.Server, listener net.Listener) error {
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(listener) }()

	var serveErr error
	select {
	case serveErr = <-errCh:
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()
	var result error
	if err := srv.Shutdown(shutdownCtx); err != nil {
		result = errors.Join(result, fmt.Errorf("shutdown: %w", err))
	}
	if serveErr == nil {
		serveErr = <-errCh
	}
	if !errors.Is(serveErr, http.ErrServerClosed) {
		result = errors.Join(result, fmt.Errorf("serve: %w", serveErr))
	}
	return result
}
