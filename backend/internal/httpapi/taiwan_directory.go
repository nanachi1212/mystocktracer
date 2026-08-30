package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"easy-stock/backend/internal/foundation"
)

type taiwanDirectoryData struct {
	Securities []foundation.SecurityIdentity `json:"securities"`
	Total      int                           `json:"total"`
	Query      string                        `json:"query,omitempty"`
	Sources    []string                      `json:"sources"`
	UpdatedAt  time.Time                     `json:"updated_at"`
	ExpiresAt  time.Time                     `json:"expires_at"`
	Stale      bool                          `json:"stale"`
}

type taiwanDirectoryCache struct {
	mu        sync.Mutex
	ttl       time.Duration
	items     []foundation.SecurityIdentity
	expiresAt time.Time
	updatedAt time.Time
}

func newTaiwanDirectoryCache(ttl time.Duration) *taiwanDirectoryCache {
	return &taiwanDirectoryCache{ttl: ttl}
}

func (c *taiwanDirectoryCache) load(ctx context.Context, provider TaiwanDirectoryProvider) (items []foundation.SecurityIdentity, updatedAt, expiresAt time.Time, stale bool, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.items) > 0 && time.Now().Before(c.expiresAt) {
		return append([]foundation.SecurityIdentity(nil), c.items...), c.updatedAt, c.expiresAt, false, nil
	}
	items, err = provider.Directory(ctx)
	if err != nil {
		if len(c.items) > 0 {
			return append([]foundation.SecurityIdentity(nil), c.items...), c.updatedAt, c.expiresAt, true, nil
		}
		return nil, time.Time{}, time.Time{}, false, err
	}
	items, err = canonicalTaiwanDirectory(items)
	if err != nil {
		return nil, time.Time{}, time.Time{}, false, err
	}
	c.items = append([]foundation.SecurityIdentity(nil), items...)
	c.updatedAt = time.Now()
	c.expiresAt = c.updatedAt.Add(c.ttl)
	return append([]foundation.SecurityIdentity(nil), c.items...), c.updatedAt, c.expiresAt, false, nil
}

func canonicalTaiwanDirectory(items []foundation.SecurityIdentity) ([]foundation.SecurityIdentity, error) {
	seen := make(map[string]struct{}, len(items))
	out := make([]foundation.SecurityIdentity, 0, len(items))
	for _, item := range items {
		item.Canonical = strings.ToUpper(strings.TrimSpace(item.Canonical))
		item.Code, item.Name = strings.ToUpper(strings.TrimSpace(item.Code)), strings.TrimSpace(item.Name)
		if item.Canonical == "" || item.Code == "" || item.Name == "" || item.Market != "TW" || item.Currency != "TWD" {
			return nil, fmt.Errorf("invalid Taiwan directory identity %q", item.Canonical)
		}
		if _, exists := seen[item.Canonical]; exists {
			continue
		}
		seen[item.Canonical] = struct{}{}
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("Taiwan directory returned no securities")
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Code == out[j].Code {
			return out[i].Exchange < out[j].Exchange
		}
		return out[i].Code < out[j].Code
	})
	return out, nil
}

func filterTaiwanDirectory(items []foundation.SecurityIdentity, query string) []foundation.SecurityIdentity {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return items
	}
	exact := make([]foundation.SecurityIdentity, 0, 1)
	out := make([]foundation.SecurityIdentity, 0, 20)
	for _, item := range items {
		fields := []string{item.Code, item.Canonical, item.Name, item.FullName}
		for _, field := range fields {
			if strings.ToLower(strings.TrimSpace(field)) == query {
				exact = append(exact, item)
				break
			}
		}
		haystack := strings.ToLower(strings.Join(fields, " "))
		if strings.Contains(haystack, query) {
			out = append(out, item)
			if len(out) == 50 {
				break
			}
		}
	}
	if len(exact) > 0 {
		return exact
	}
	return out
}

func (s *Server) taiwanDirectoryHandler(w http.ResponseWriter, r *http.Request) {
	if s.taiwanDirectory == nil {
		writeError(w, http.StatusServiceUnavailable, "Taiwan directory provider is unavailable")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	items, updatedAt, expiresAt, stale, err := s.taiwanDirectories.load(ctx, s.taiwanDirectory)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("query"))
	filtered := filterTaiwanDirectory(items, query)
	writeJSON(w, http.StatusOK, map[string]any{"data": taiwanDirectoryData{
		Securities: filtered, Total: len(filtered), Query: query,
		Sources: []string{"twse", "tpex"}, UpdatedAt: updatedAt, ExpiresAt: expiresAt, Stale: stale,
	}})
}
