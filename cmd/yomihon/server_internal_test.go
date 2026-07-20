package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestServerBoundsRequestHeaders(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewUnstartedServer(handler)
	srv.Config = newHTTPServer(handler)
	srv.Start()
	t.Cleanup(srv.Close)

	if srv.Config.MaxHeaderBytes != maxRequestHeaderBytes || srv.Config.MaxHeaderBytes <= 0 {
		t.Fatalf("MaxHeaderBytes = %d, want %d", srv.Config.MaxHeaderBytes, maxRequestHeaderBytes)
	}
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL, http.NoBody)
	if err != nil {
		t.Fatalf("new ordinary request: %v", err)
	}
	resp, err := srv.Client().Do(request)
	if err != nil {
		t.Fatalf("ordinary request: %v", err)
	}
	if closeErr := resp.Body.Close(); closeErr != nil {
		t.Errorf("close ordinary response: %v", closeErr)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ordinary request status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	addr := strings.TrimPrefix(srv.URL, "http://")
	var dialer net.Dialer
	conn, err := dialer.DialContext(t.Context(), "tcp", addr)
	if err != nil {
		t.Fatalf("dial server: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := conn.Close(); closeErr != nil {
			t.Errorf("close oversized connection: %v", closeErr)
		}
	})
	header := strings.Repeat("a", maxRequestHeaderBytes+8<<10)
	if _, err = fmt.Fprintf(conn, "GET / HTTP/1.1\r\nHost: %s\r\nX-Fill: %s\r\n\r\n", addr, header); err != nil {
		t.Fatalf("write oversized request: %v", err)
	}
	request, err = http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL, http.NoBody)
	if err != nil {
		t.Fatalf("new response request: %v", err)
	}
	oversized, err := http.ReadResponse(bufio.NewReader(conn), request)
	if err != nil {
		t.Fatalf("read oversized response: %v", err)
	}
	if closeErr := oversized.Body.Close(); closeErr != nil {
		t.Errorf("close oversized response: %v", closeErr)
	}
	if oversized.StatusCode != http.StatusRequestHeaderFieldsTooLarge {
		t.Errorf("oversized request status = %d, want %d", oversized.StatusCode, http.StatusRequestHeaderFieldsTooLarge)
	}
}

func TestServeHTTPDrainsAcceptedHandlersAfterListenerFailure(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("listener failed")
	assertServeHTTPDrainsAcceptedHandler(t, t.Context(), wantErr, func(listener *failAfterOneListener) {
		listener.Fail()
	})
}

func TestServeHTTPDrainsAcceptedHandlersAfterCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	assertServeHTTPDrainsAcceptedHandler(t, ctx, nil, func(*failAfterOneListener) {
		cancel()
	})
}

func assertServeHTTPDrainsAcceptedHandler(
	t *testing.T,
	ctx context.Context,
	wantErr error,
	stop func(*failAfterOneListener),
) {
	t.Helper()

	serverConn, clientConn := net.Pipe()
	t.Cleanup(func() {
		if closeErr := clientConn.Close(); closeErr != nil && !errors.Is(closeErr, net.ErrClosed) {
			t.Errorf("close client connection: %v", closeErr)
		}
	})
	listenerErr := wantErr
	if listenerErr == nil {
		listenerErr = errors.New("listener closed during cancellation")
	}
	listener := &failAfterOneListener{
		conn: serverConn,
		err:  listenerErr,
		fail: make(chan struct{}),
	}
	handlerStarted := make(chan struct{})
	releaseHandler := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(handlerStarted)
		<-releaseHandler
		if _, err := io.WriteString(w, "ok"); err != nil {
			t.Errorf("write response: %v", err)
		}
	})
	srv := newHTTPServer(handler)
	shutdownStarted := make(chan struct{})
	srv.RegisterOnShutdown(func() { close(shutdownStarted) })
	serveResult := make(chan error, 1)
	go func() { serveResult <- serveHTTP(ctx, srv, listener) }()

	clientResult := make(chan error, 1)
	go func() {
		if _, err := io.WriteString(clientConn, "GET / HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n"); err != nil {
			clientResult <- fmt.Errorf("write request: %w", err)
			return
		}
		request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://localhost/", http.NoBody)
		if err != nil {
			clientResult <- fmt.Errorf("create response request: %w", err)
			return
		}
		response, err := http.ReadResponse(bufio.NewReader(clientConn), request)
		if err != nil {
			clientResult <- fmt.Errorf("read response: %w", err)
			return
		}
		_, readErr := io.Copy(io.Discard, response.Body)
		clientResult <- errors.Join(readErr, response.Body.Close())
	}()

	<-handlerStarted
	stop(listener)
	<-shutdownStarted
	select {
	case err := <-serveResult:
		t.Fatalf("serveHTTP() returned %v before accepted handler exited", err)
	default:
	}
	close(releaseHandler)
	if err := <-clientResult; err != nil {
		t.Fatalf("client request error = %v", err)
	}
	err := <-serveResult
	if errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("serveHTTP() reached its shutdown deadline before the handler was released: %v", err)
	}
	if wantErr == nil && err != nil {
		t.Fatalf("serveHTTP() after cancellation = %v, want nil after drain", err)
	}
	if wantErr != nil && !errors.Is(err, wantErr) {
		t.Fatalf("serveHTTP() = %v, want listener failure %v after drain", err, wantErr)
	}
}

type failAfterOneListener struct {
	mu        sync.Mutex
	conn      net.Conn
	err       error
	fail      chan struct{}
	closeOnce sync.Once
}

func (l *failAfterOneListener) Accept() (net.Conn, error) {
	l.mu.Lock()
	if l.conn != nil {
		conn := l.conn
		l.conn = nil
		l.mu.Unlock()
		return conn, nil
	}
	l.mu.Unlock()
	<-l.fail
	return nil, l.err
}

func (l *failAfterOneListener) Close() error {
	l.Fail()
	return nil
}

func (l *failAfterOneListener) Addr() net.Addr { return &net.TCPAddr{} }

func (l *failAfterOneListener) Fail() { l.closeOnce.Do(func() { close(l.fail) }) }
