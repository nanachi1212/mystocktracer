package httpapi

import (
	"bufio"
	"crypto/subtle"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

var requestSequence atomic.Uint64

type observedResponse struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (w *observedResponse) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *observedResponse) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	count, err := w.ResponseWriter.Write(body)
	w.bytes += int64(count)
	return count, err
}

func (w *observedResponse) Flush() {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *observedResponse) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errorsNew("response writer does not support hijacking")
	}
	if w.status == 0 {
		w.status = http.StatusSwitchingProtocols
	}
	return hijacker.Hijack()
}

func (w *observedResponse) ReadFrom(reader io.Reader) (int64, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	if optimized, ok := w.ResponseWriter.(io.ReaderFrom); ok {
		count, err := optimized.ReadFrom(reader)
		w.bytes += count
		return count, err
	}
	count, err := io.Copy(w.ResponseWriter, reader)
	w.bytes += count
	return count, err
}

func (w *observedResponse) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	requestID := requestID(r)
	response := &observedResponse{ResponseWriter: w}
	response.Header().Set("X-Request-ID", requestID)
	defer s.logHTTP(r, response, requestID, started)

	if applyCORS(response, r) {
		response.WriteHeader(http.StatusNoContent)
		return
	}
	if !s.authorized(r) {
		writeError(response, http.StatusUnauthorized, "unauthorized")
		return
	}
	s.router.ServeHTTP(response, r)
}

func applyCORS(w http.ResponseWriter, r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if isAllowedOrigin(origin) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
	}
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-A-Stock-Token, X-Request-ID")
	w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	return r.Method == http.MethodOptions
}

func isAllowedOrigin(origin string) bool {
	if origin == "" || origin == "null" {
		return origin == "null"
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func (s *Server) authorized(r *http.Request) bool {
	if s.token == "" || r.URL.Path == "/api/health" || r.Method == http.MethodOptions {
		return true
	}
	candidates := []string{
		r.URL.Query().Get("token"),
		r.Header.Get("X-A-Stock-Token"),
	}
	authorization := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(authorization, "Bearer ") {
		candidates = append(candidates, strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer ")))
	}
	for _, candidate := range candidates {
		if constantTimeEqual(candidate, s.token) {
			return true
		}
	}
	return false
}

func constantTimeEqual(candidate, expected string) bool {
	if len(candidate) != len(expected) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(candidate), []byte(expected)) == 1
}

func requestID(r *http.Request) string {
	value := strings.TrimSpace(r.Header.Get("X-Request-ID"))
	if validRequestID(value) {
		return value
	}
	return fmt.Sprintf("%x-%x", time.Now().UnixMilli(), requestSequence.Add(1))
}

func validRequestID(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, char := range value {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' ||
			char >= '0' && char <= '9' || strings.ContainsRune("._-", char) {
			continue
		}
		return false
	}
	return true
}

func requestFeature(path string) string {
	switch {
	case path == "/api/health":
		return "health"
	case strings.HasPrefix(path, "/api/v1/tw/"):
		return "taiwan"
	case strings.HasPrefix(path, "/api/v1/settings"):
		return "settings"
	case strings.HasPrefix(path, "/api/v1/ai"):
		return "ai-chat"
	default:
		return "http"
	}
}

func (s *Server) logHTTP(r *http.Request, response *observedResponse, id string, started time.Time) {
	if s == nil || s.logger == nil {
		return
	}
	status := response.status
	if status == 0 {
		status = http.StatusOK
	}
	level := "info"
	if status >= http.StatusInternalServerError {
		level = "error"
	} else if status >= http.StatusBadRequest {
		level = "warn"
	}
	s.logger.Printf(
		"level=%s event=http_request feature=%q request_id=%q method=%q path=%q status=%d duration_ms=%d response_bytes=%d",
		level, requestFeature(r.URL.Path), id, r.Method, r.URL.Path, status,
		time.Since(started).Milliseconds(), response.bytes,
	)
}

func errorsNew(message string) error { return &middlewareError{message: message} }

type middlewareError struct{ message string }

func (e *middlewareError) Error() string { return e.message }
