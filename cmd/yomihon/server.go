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

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 120 * time.Second

	// shutdownSlack is what the grace period gets on top of the longest read
	// deadline a connection can still be sitting on.
	shutdownSlack = 5 * time.Second
)

func newHTTPServer(handler http.Handler) *http.Server {
	return &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxRequestHeaderBytes,
	}
}

// shutdownGrace is how long graceful shutdown waits for requests already
// accepted. It is derived from the server's own read deadlines rather than
// written as a number beside them: a grace period equal to one of those
// deadlines lets a connection that merely opened, or stalled part-way through
// a body, trip the shutdown deadline at the same instant its own deadline
// fires, and an ordinary interrupt then reports a handler that overran when
// none did. Deriving it means a later edit to a read deadline carries.
func shutdownGrace(srv *http.Server) time.Duration {
	return max(srv.ReadHeaderTimeout, srv.ReadTimeout) + shutdownSlack
}

// serveHTTP runs srv until cancellation or a listener failure. Both paths
// attempt graceful shutdown within grace. The readingSite separately owns
// accepted request lifetimes, so its deferred close still waits for a handler
// if this bounded attempt reaches its deadline.
func serveHTTP(ctx context.Context, srv *http.Server, listener net.Listener, grace time.Duration) error {
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(listener) }()

	var serveErr error
	select {
	case serveErr = <-errCh:
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), grace)
	defer cancel()
	var result error
	if err := srv.Shutdown(shutdownCtx); err != nil {
		result = errors.Join(result, fmt.Errorf("shutdown: %w", err))
		// A handler still running this long after the grace period is blocked
		// writing to a socket the reader stopped draining, and it stays blocked
		// until the write deadline — half a minute, where the vault-side wait
		// that follows has no deadline at all. Closing the connections releases
		// it, so what that wait costs is the work still owed to the vault
		// rather than a client that walked away. Only a deadline earns this:
		// any other shutdown failure is reported and left alone.
		if errors.Is(err, context.DeadlineExceeded) {
			if closeErr := srv.Close(); closeErr != nil {
				result = errors.Join(result, fmt.Errorf("close: %w", closeErr))
			}
		}
	}
	if serveErr == nil {
		serveErr = <-errCh
	}
	if !errors.Is(serveErr, http.ErrServerClosed) {
		result = errors.Join(result, fmt.Errorf("serve: %w", serveErr))
	}
	return result
}
