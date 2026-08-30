package taiwan

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"easy-stock/backend/internal/foundation"
)

const (
	defaultTWSEBaseURL = "https://openapi.twse.com.tw/v1"
	defaultTPExBaseURL = "https://www.tpex.org.tw/openapi/v1"
	maxDirectoryBytes  = 16 << 20
)

type Client struct {
	httpClient  *http.Client
	twseBaseURL string
	tpexBaseURL string
	chipMu      sync.Mutex
	chipDays    map[string]chipSnapshot
}

type Config struct {
	HTTPClient  *http.Client
	TWSEBaseURL string
	TPExBaseURL string
}

func NewClient(config Config) *Client {
	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &Client{
		httpClient:  httpClient,
		twseBaseURL: first(config.TWSEBaseURL, defaultTWSEBaseURL),
		tpexBaseURL: first(config.TPExBaseURL, defaultTPExBaseURL),
		chipDays:    map[string]chipSnapshot{},
	}
}

func (c *Client) Directory(ctx context.Context) ([]foundation.SecurityIdentity, error) {
	twseStocks, err := c.twseStocks(ctx)
	if err != nil {
		return nil, fmt.Errorf("TWSE company directory: %w", err)
	}
	twseETFs, err := c.twseETFs(ctx)
	if err != nil {
		return nil, fmt.Errorf("TWSE fund directory: %w", err)
	}
	tpexStocks, err := c.tpexStocks(ctx)
	if err != nil {
		return nil, fmt.Errorf("TPEx company directory: %w", err)
	}
	items := append(twseStocks, twseETFs...)
	items = append(items, tpexStocks...)
	if len(items) == 0 {
		return nil, fmt.Errorf("official Taiwan directory returned no securities")
	}
	return items, nil
}

func (c *Client) getJSON(ctx context.Context, sourceURL string, target any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP status %d", response.StatusCode)
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxDirectoryBytes))
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}
	return nil
}

func first(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimRight(value, "/")
	}
	return fallback
}
