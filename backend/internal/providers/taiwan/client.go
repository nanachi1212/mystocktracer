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
	defaultTWSEBaseURL     = "https://openapi.twse.com.tw/v1"
	defaultTPExBaseURL     = "https://www.tpex.org.tw/openapi/v1"
	defaultFinMindURL      = "https://api.finmindtrade.com/api/v4/data"
	defaultCashflowBaseURL = "https://mopsov.twse.com.tw"
	maxDirectoryBytes      = 16 << 20
)

type Client struct {
	httpClient        *http.Client
	twseBaseURL       string
	tpexBaseURL       string
	finMindURL        string
	twseReportBaseURL string
	tpexReportBaseURL string
	cashflowBaseURL   string
	chipMu            sync.Mutex
	chipDays          map[string]chipSnapshot
	fundMu            sync.Mutex
	fundRows          map[string]fundSnapshot
	// M7H — cashflowMu guards the single current cashflowSnapshot (see cashflow_xbrl.go): the whole
	// check-cache/discover/download/parse/store sequence runs under this lock (mirroring chipDay's
	// existing tight-locking pattern, not fundamentalsRows' looser check-then-fetch one), so concurrent
	// cold Screener requests never each trigger their own 110-130MB archive download.
	cashflowMu       sync.Mutex
	cashflowSnapshot *cashflowSnapshot
	snapshotMu       sync.RWMutex
	dailyDays         map[string][]foundation.TaiwanDailySnapshot
	instDays          map[string][]foundation.InstitutionalFlow
	marginDays        map[string][]foundation.MarginTrading
	calendar          foundation.TaiwanTradingCalendar
	now               func() time.Time
	twseSem           chan struct{}
	chipFlightMu      sync.Mutex
	chipFlights       map[string]*chipFlightCall
	dailyFlightMu     sync.Mutex
	dailyFlights      map[string]*dailyFlightCall
}

type chipFlightCall struct {
	wg      sync.WaitGroup
	payload monthlyResponse
	err     error
}

type dailyFlightCall struct {
	wg      sync.WaitGroup
	section foundation.TaiwanRefreshSection
}

type Config struct {
	HTTPClient        *http.Client
	TWSEBaseURL       string
	TPExBaseURL       string
	FinMindURL        string
	TWSEReportBaseURL string
	TPExReportBaseURL string
	CashflowBaseURL   string
	Holidays          map[string]bool
	Now               func() time.Time
}

func NewClient(config Config) *Client {
	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	return &Client{
		httpClient:        httpClient,
		twseBaseURL:       first(config.TWSEBaseURL, defaultTWSEBaseURL),
		tpexBaseURL:       first(config.TPExBaseURL, defaultTPExBaseURL),
		finMindURL:        first(config.FinMindURL, defaultFinMindURL),
		twseReportBaseURL: first(config.TWSEReportBaseURL, "https://www.twse.com.tw"),
		tpexReportBaseURL: first(config.TPExReportBaseURL, "https://www.tpex.org.tw"),
		cashflowBaseURL:   first(config.CashflowBaseURL, defaultCashflowBaseURL),
		chipDays:          map[string]chipSnapshot{},
		fundRows:          map[string]fundSnapshot{},
		dailyDays:         map[string][]foundation.TaiwanDailySnapshot{},
		instDays:          map[string][]foundation.InstitutionalFlow{},
		marginDays:        map[string][]foundation.MarginTrading{},
		calendar:          foundation.TaiwanTradingCalendar{Holidays: config.Holidays},
		now:               now,
		twseSem:           make(chan struct{}, 4),
		chipFlights:       map[string]*chipFlightCall{},
		dailyFlights:      map[string]*dailyFlightCall{},
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
