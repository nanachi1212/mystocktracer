package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/nanachi1212/mystocktracer/backend/internal/runtimelog"
)

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err == io.EOF {
		return nil
	} else if err != nil {
		return err
	}
	return io.ErrUnexpectedEOF
}

func contextWithTimeout(r *http.Request, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), timeout)
}

func marketLimitQuery(r *http.Request, fallback, maximum int) (int, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("limit"))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 || value > maximum {
		return 0, fmt.Errorf("limit must be between 1 and %d", maximum)
	}
	return value, nil
}

func writeError(w http.ResponseWriter, status int, message string) {
	if status >= http.StatusInternalServerError {
		log.Printf("level=error event=http_error status=%d message=%q", status, runtimelog.Redact(message))
	}
	writeJSON(w, status, map[string]string{"error": message})
}

func writeInternalError(w http.ResponseWriter, event, message string, err error) {
	detail := ""
	if err != nil {
		detail = runtimelog.Redact(err.Error())
	}
	log.Printf("level=error event=%s detail=%q", event, detail)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": message})
}

func writeUpstreamError(w http.ResponseWriter, event, message string, err error) {
	detail := ""
	if err != nil {
		detail = runtimelog.Redact(err.Error())
	}
	log.Printf("level=error event=%s detail=%q", event, detail)
	writeJSON(w, http.StatusBadGateway, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
