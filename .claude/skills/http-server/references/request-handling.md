# Request Handling

Parsing query parameters, validating requests, file uploads, streaming responses, and reverse proxying.

## Common Patterns

### Query Parameters

```go
func listOrders(store *order.Store) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        status := r.URL.Query().Get("status")
        limitStr := r.URL.Query().Get("limit")
        limit := 20 // default
        if limitStr != "" {
            var err error
            limit, err = strconv.Atoi(limitStr)
            if err != nil || limit < 1 || limit > 100 {
                respondError(w, http.StatusBadRequest, "invalid limit")
                return
            }
        }
        // ...
    }
}
```

### Request Validation

```go
func createOrder(store *order.Store) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        req, err := decode[CreateOrderRequest](r)
        if err != nil {
            respondError(w, http.StatusBadRequest, "invalid request body")
            return
        }

        if req.Total <= 0 {
            respondError(w, http.StatusBadRequest, "total must be positive")
            return
        }

        // ...
    }
}
```

## File Upload Handling

```go
func uploadHandler() http.HandlerFunc {
    const maxUploadSize = 10 << 20 // 10 MB

    return func(w http.ResponseWriter, r *http.Request) {
        r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)

        if err := r.ParseMultipartForm(maxUploadSize); err != nil {
            respondError(w, http.StatusBadRequest, "file too large")
            return
        }

        file, header, err := r.FormFile("file")
        if err != nil {
            respondError(w, http.StatusBadRequest, "missing file")
            return
        }
        defer file.Close()

        // Validate content type
        contentType := header.Header.Get("Content-Type")
        if contentType != "image/png" && contentType != "image/jpeg" {
            respondError(w, http.StatusBadRequest, "invalid file type")
            return
        }

        // Process file...
    }
}
```

## Streaming Response

```go
func streamHandler() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        flusher, ok := w.(http.Flusher)
        if !ok {
            http.Error(w, "streaming not supported", http.StatusInternalServerError)
            return
        }

        w.Header().Set("Content-Type", "text/event-stream")
        w.Header().Set("Cache-Control", "no-cache")
        w.Header().Set("Connection", "keep-alive")

        for i := 0; i < 10; i++ {
            select {
            case <-r.Context().Done():
                return
            default:
                fmt.Fprintf(w, "data: message %d\n\n", i)
                flusher.Flush()
                time.Sleep(time.Second)
            }
        }
    }
}
```

## Reverse Proxy (Go 1.26+)

```go
// DEPRECATED (Go 1.26): ReverseProxy.Director
// Use ReverseProxy.Rewrite instead (safer, receives ProxyRequest)
proxy := &httputil.ReverseProxy{
    Rewrite: func(r *httputil.ProxyRequest) {
        r.SetURL(target)
        r.SetXForwarded()
    },
}
```
