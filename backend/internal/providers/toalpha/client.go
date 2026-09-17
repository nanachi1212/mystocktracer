package toalpha

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"easy-stock/backend/internal/foundation"
)

const (
	DefaultMOPSEndpoint = "https://toalpha.tw/mcp/mops"
	defaultTimeout      = 8 * time.Second
	defaultCacheTTL     = 10 * time.Minute
	defaultRetries      = 1
	maxResponseBytes    = 4 << 20
	maxCacheEntries     = 256
	protocolVersion     = "2025-03-26"
)

type Config struct {
	Enabled    bool
	Endpoint   string
	HTTPClient *http.Client
	Timeout    time.Duration
	Retries    int
	CacheTTL   time.Duration
	Now        func() time.Time
}

type Client struct {
	enabled    bool
	endpoint   string
	httpClient *http.Client
	timeout    time.Duration
	retries    int
	cacheTTL   time.Duration
	now        func() time.Time

	cacheMu sync.Mutex
	cache   map[string]cachedFeed
}

type cachedFeed struct {
	expires time.Time
	feed    foundation.TaiwanCorporateEventFeed
}

func NewClient(config Config) *Client {
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	cacheTTL := config.CacheTTL
	if cacheTTL <= 0 {
		cacheTTL = defaultCacheTTL
	}
	retries := config.Retries
	if retries == 0 {
		retries = defaultRetries
	} else if retries < 0 {
		retries = 0
	}
	if retries > 1 {
		retries = 1
	}
	client := config.HTTPClient
	if client == nil {
		client = &http.Client{}
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	endpoint := strings.TrimSpace(config.Endpoint)
	if endpoint == "" {
		endpoint = DefaultMOPSEndpoint
	}
	return &Client{
		enabled: config.Enabled, endpoint: endpoint, httpClient: client,
		timeout: timeout, retries: retries, cacheTTL: cacheTTL, now: now,
		cache: map[string]cachedFeed{},
	}
}

// CorporateEvents returns bounded recent MOPS announcements. Disabled and
// unavailable states are represented in the feed instead of failing callers,
// so this optional integration can never remove canonical Taiwan evidence.
func (c *Client) CorporateEvents(ctx context.Context, canonical string, days, limit int) foundation.TaiwanCorporateEventFeed {
	if c == nil || !c.enabled {
		return unavailableFeed(foundation.TaiwanCorporateEventsNotQueried, "ToAlpha MOPS integration is disabled")
	}
	parsedEndpoint, endpointErr := url.ParseRequestURI(c.endpoint)
	if endpointErr != nil || (parsedEndpoint.Scheme != "https" && parsedEndpoint.Scheme != "http") || parsedEndpoint.Host == "" {
		return unavailableFeed(foundation.TaiwanCorporateEventsUnavailable, "ToAlpha MOPS endpoint is invalid")
	}
	code := stockCode(canonical)
	if code == "" {
		return unavailableFeed(foundation.TaiwanCorporateEventsUnavailable, "canonical Taiwan stock symbol is required")
	}
	if days <= 0 || days > 365 {
		days = 90
	}
	if limit <= 0 || limit > 30 {
		limit = 12
	}
	cacheKey := strings.Join([]string{strings.ToUpper(strings.TrimSpace(canonical)), strconv.Itoa(days), strconv.Itoa(limit)}, ":")
	cachedValue, cacheFresh, cacheFound := c.cached(cacheKey)
	if cacheFresh {
		return cachedValue
	}

	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	var raw materialNewsResponse
	var err error
	for attempt := 0; attempt <= c.retries; attempt++ {
		raw, err = c.fetchMaterialNews(callCtx, code, days, limit)
		if err == nil || !retryable(err) || callCtx.Err() != nil {
			break
		}
	}
	if err != nil {
		reason := "ToAlpha MOPS is unavailable"
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(callCtx.Err(), context.DeadlineExceeded) {
			reason = "ToAlpha MOPS request timed out"
		} else if errors.Is(err, context.Canceled) || errors.Is(callCtx.Err(), context.Canceled) {
			reason = "ToAlpha MOPS request was canceled"
		}
		if cacheFound {
			cachedValue.Status, cachedValue.Stale, cachedValue.Reason = foundation.TaiwanCorporateEventsStale, true, reason+"; showing expired cached events"
			for index := range cachedValue.Events {
				cachedValue.Events[index].Status = foundation.TaiwanCorporateEventsStale
				cachedValue.Events[index].Stale = true
				cachedValue.Events[index].Reason = cachedValue.Reason
			}
			return cachedValue
		}
		return unavailableFeed(foundation.TaiwanCorporateEventsUnavailable, reason)
	}
	feed := normalizeMaterialNews(canonical, raw, c.now().UTC())
	if feed.Status == foundation.TaiwanCorporateEventsPartial {
		return cloneFeed(feed)
	}
	c.cacheMu.Lock()
	if len(c.cache) >= maxCacheEntries {
		oldestKey := ""
		var oldestExpiry time.Time
		for key, entry := range c.cache {
			if oldestKey == "" || entry.expires.Before(oldestExpiry) {
				oldestKey, oldestExpiry = key, entry.expires
			}
		}
		delete(c.cache, oldestKey)
	}
	c.cache[cacheKey] = cachedFeed{expires: c.now().Add(c.cacheTTL), feed: feed}
	c.cacheMu.Unlock()
	return cloneFeed(feed)
}

func (c *Client) cached(key string) (foundation.TaiwanCorporateEventFeed, bool, bool) {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	entry, ok := c.cache[key]
	if !ok {
		return foundation.TaiwanCorporateEventFeed{}, false, false
	}
	return cloneFeed(entry.feed), c.now().Before(entry.expires), true
}

func cloneFeed(feed foundation.TaiwanCorporateEventFeed) foundation.TaiwanCorporateEventFeed {
	feed.Events = append([]foundation.TaiwanCorporateEvent(nil), feed.Events...)
	return feed
}

func unavailableFeed(status, reason string) foundation.TaiwanCorporateEventFeed {
	return foundation.TaiwanCorporateEventFeed{
		Status: status, Provider: foundation.TaiwanCorporateEventProviderToAlpha,
		Source: foundation.TaiwanCorporateEventSourceMOPS, Reason: reason,
		Events: []foundation.TaiwanCorporateEvent{},
	}
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type initializeResult struct {
	ProtocolVersion string          `json:"protocolVersion"`
	Capabilities    json.RawMessage `json:"capabilities"`
	ServerInfo      json.RawMessage `json:"serverInfo"`
	Instructions    string          `json:"instructions,omitempty"`
}

type toolCallResult struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	IsError           bool            `json:"isError"`
	StructuredContent json.RawMessage `json:"structuredContent,omitempty"`
	Meta              json.RawMessage `json:"_meta,omitempty"`
}

type httpStatusError struct{ status int }

func (e httpStatusError) Error() string { return fmt.Sprintf("HTTP status %d", e.status) }

func retryable(err error) bool {
	var statusErr httpStatusError
	if errors.As(err, &statusErr) {
		return statusErr.status == http.StatusTooManyRequests || statusErr.status >= 500
	}
	var networkError net.Error
	return errors.As(err, &networkError) && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
}

func (c *Client) fetchMaterialNews(ctx context.Context, code string, days, limit int) (materialNewsResponse, error) {
	initResponse, sessionID, err := c.post(ctx, "", rpcRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]string{"name": "mystocktracer", "version": "1"},
		},
	})
	if err != nil {
		return materialNewsResponse{}, fmt.Errorf("initialize ToAlpha MOPS: %w", err)
	}
	if sessionID == "" {
		return materialNewsResponse{}, errors.New("initialize ToAlpha MOPS: missing MCP session id")
	}
	var initialized initializeResult
	if err := decodeStrictJSON(initResponse.Result, &initialized); err != nil || initialized.ProtocolVersion != protocolVersion {
		return materialNewsResponse{}, errors.New("initialize ToAlpha MOPS: malformed response")
	}
	if _, _, err := c.post(ctx, sessionID, rpcRequest{JSONRPC: "2.0", Method: "notifications/initialized"}); err != nil {
		return materialNewsResponse{}, fmt.Errorf("notify ToAlpha MOPS initialized: %w", err)
	}
	callResponse, _, err := c.post(ctx, sessionID, rpcRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: map[string]any{"name": "material_news", "arguments": map[string]any{"stock_id": code, "days": days, "limit": limit}},
	})
	if err != nil {
		return materialNewsResponse{}, fmt.Errorf("call ToAlpha material_news: %w", err)
	}
	var result toolCallResult
	if err := decodeStrictJSON(callResponse.Result, &result); err != nil {
		return materialNewsResponse{}, fmt.Errorf("decode ToAlpha tool result: %w", err)
	}
	if len(result.Content) != 1 || result.Content[0].Type != "text" {
		return materialNewsResponse{}, errors.New("decode ToAlpha tool result: unknown content")
	}
	if result.IsError {
		return materialNewsResponse{}, fmt.Errorf("ToAlpha material_news error: %s", boundedError(result.Content[0].Text))
	}
	var raw materialNewsResponse
	decoder := json.NewDecoder(strings.NewReader(result.Content[0].Text))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&raw); err != nil {
		return materialNewsResponse{}, fmt.Errorf("decode ToAlpha material_news payload: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return materialNewsResponse{}, errors.New("decode ToAlpha material_news payload: trailing content")
	}
	if raw.Rows == nil || raw.Count < 0 || raw.Count < len(raw.Rows) {
		return materialNewsResponse{}, errors.New("decode ToAlpha material_news payload: missing rows")
	}
	if raw.Stock.ID != "" && strings.TrimSpace(raw.Stock.ID) != code {
		return materialNewsResponse{}, errors.New("decode ToAlpha material_news payload: stock id mismatch")
	}
	for _, row := range raw.Rows {
		if row.ID != "" && strings.TrimSpace(row.ID) != code {
			return materialNewsResponse{}, errors.New("decode ToAlpha material_news payload: row stock id mismatch")
		}
	}
	return raw, nil
}

func (c *Client) post(ctx context.Context, sessionID string, payload rpcRequest) (rpcResponse, string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return rpcResponse{}, "", err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return rpcResponse{}, "", err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	if sessionID != "" {
		request.Header.Set("Mcp-Session-Id", sessionID)
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return rpcResponse{}, "", err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return rpcResponse{}, "", httpStatusError{status: response.StatusCode}
	}
	returnedSessionID := strings.TrimSpace(response.Header.Get("Mcp-Session-Id"))
	if payload.ID == 0 {
		data, readErr := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
		if readErr != nil {
			return rpcResponse{}, returnedSessionID, readErr
		}
		if len(data) > maxResponseBytes {
			return rpcResponse{}, returnedSessionID, errors.New("MCP response exceeds size limit")
		}
		return rpcResponse{}, returnedSessionID, nil
	}
	rpc, err := decodeRPCResponse(response.Body, payload.ID)
	if err != nil {
		return rpcResponse{}, "", err
	}
	if rpc.Error != nil {
		return rpcResponse{}, "", fmt.Errorf("JSON-RPC %d: %s", rpc.Error.Code, boundedError(rpc.Error.Message))
	}
	if len(rpc.Result) == 0 {
		return rpcResponse{}, "", errors.New("JSON-RPC response missing result")
	}
	return rpc, returnedSessionID, nil
}

func decodeRPCResponse(reader io.Reader, expectedID int) (rpcResponse, error) {
	limited := io.LimitReader(reader, maxResponseBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return rpcResponse{}, err
	}
	if len(data) > maxResponseBytes {
		return rpcResponse{}, errors.New("MCP response exceeds size limit")
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return rpcResponse{}, errors.New("empty MCP response")
	}
	if trimmed[0] == '{' {
		var response rpcResponse
		if err := json.Unmarshal(trimmed, &response); err != nil {
			return rpcResponse{}, err
		}
		if response.JSONRPC != "2.0" || response.ID != expectedID {
			return rpcResponse{}, errors.New("unexpected JSON-RPC response id")
		}
		return response, nil
	}
	scanner := bufio.NewScanner(bytes.NewReader(trimmed))
	scanner.Buffer(make([]byte, 64<<10), maxResponseBytes)
	var dataLines []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
		if strings.TrimSpace(line) == "" && len(dataLines) > 0 {
			var response rpcResponse
			if err := json.Unmarshal([]byte(strings.Join(dataLines, "\n")), &response); err == nil && response.JSONRPC == "2.0" && response.ID == expectedID {
				return response, nil
			}
			dataLines = nil
		}
	}
	if err := scanner.Err(); err != nil {
		return rpcResponse{}, err
	}
	if len(dataLines) > 0 {
		var response rpcResponse
		if err := json.Unmarshal([]byte(strings.Join(dataLines, "\n")), &response); err == nil && response.JSONRPC == "2.0" && response.ID == expectedID {
			return response, nil
		}
	}
	return rpcResponse{}, errors.New("matching JSON-RPC event not found")
}

func stockCode(canonical string) string {
	canonical = strings.ToUpper(strings.TrimSpace(canonical))
	dot := strings.LastIndex(canonical, ".")
	if dot <= 0 || (canonical[dot+1:] != "TWSE" && canonical[dot+1:] != "TPEX") {
		return ""
	}
	return canonical[:dot]
}

func boundedError(value string) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) > 300 {
		runes = runes[:300]
	}
	return string(runes)
}

func decodeStrictJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("trailing content")
	}
	return nil
}
