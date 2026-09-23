package httpapi

import (
	"context"
	"encoding/json"
	"errors"
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
	var trailing json.RawMessage
	switch err := decoder.Decode(&trailing); {
	case errors.Is(err, io.EOF):
		return nil
	case err != nil:
		return err
	default:
		return errors.New("unexpected data after JSON value")
	}
}

func contextWithTimeout(request *http.Request, duration time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(request.Context(), duration)
}

func marketLimitQuery(request *http.Request, fallback, maximum int) (int, error) {
	input := strings.TrimSpace(request.URL.Query().Get("limit"))
	if input == "" {
		return fallback, nil
	}
	limit, err := strconv.Atoi(input)
	if err == nil && limit >= 1 && limit <= maximum {
		return limit, nil
	}
	return 0, fmt.Errorf("limit must be between 1 and %d", maximum)
}

func writeJSON(writer http.ResponseWriter, status int, payload any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(payload)
}

func writeError(writer http.ResponseWriter, status int, message string) {
	if status >= http.StatusInternalServerError {
		log.Printf("level=error event=http_error status=%d message=%q", status, runtimelog.Redact(message))
	}
	writeJSON(writer, status, map[string]string{"error": message})
}

func writeInternalError(writer http.ResponseWriter, event, message string, cause error) {
	writeLoggedFailure(writer, http.StatusInternalServerError, event, message, cause)
}

func writeUpstreamError(writer http.ResponseWriter, event, message string, cause error) {
	writeLoggedFailure(writer, http.StatusBadGateway, event, message, cause)
}

func writeLoggedFailure(writer http.ResponseWriter, status int, event, message string, cause error) {
	detail := ""
	if cause != nil {
		detail = runtimelog.Redact(cause.Error())
	}
	log.Printf("level=error event=%s detail=%q", event, detail)
	writeJSON(writer, status, map[string]string{"error": message})
}

func firstNonEmpty(values ...string) string {
	for _, candidate := range values {
		if strings.TrimSpace(candidate) != "" {
			return candidate
		}
	}
	return ""
}
