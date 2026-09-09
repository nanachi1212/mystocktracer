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
	// M7H/P5.5B.2 — cashflowMu guards cashflowSnapshot, cashflowFilling, and cashflowRetryAfter.
	cashflowMu          sync.Mutex
	cashflowSnapshot    *cashflowSnapshot
	cashflowCacheDir    string // empty means no persistent cache
	cashflowFilling     bool
	cashflowRetryAfter  time.Time
	cashflowFillDone    chan struct{}
	syncCashflow        bool
	bgCtx               context.Context
	bgCancel            context.CancelFunc
	closeOnce           sync.Once
	snapshotMu          sync.RWMutex
	dailyDays           map[string][]foundation.TaiwanDailySnapshot
	instDays            map[string][]foundation.InstitutionalFlow
	marginDays          map[string][]foundation.MarginTrading
	calendar            foundation.TaiwanTradingCalendar
	now                 func() time.Time
	twseSem             chan struct{}
	chipFlightMu        sync.Mutex
	chipFlights         map[string]*chipFlightCall
	dailyFlightMu       sync.Mutex
	dailyFlights        map[string]*dailyFlightCall
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
	CashflowCacheDir  string // optional directory for persistent cashflow snapshot cache
	Holidays          map[string]bool
	Now               func() time.Time
	SyncCashflow      bool // synchronous cashflow fill mode for tests; default false (production is always async background fill)
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
	bgCtx, bgCancel := context.WithCancel(context.Background())
	return &Client{
		httpClient:        httpClient,
		twseBaseURL:       first(config.TWSEBaseURL, defaultTWSEBaseURL),
		tpexBaseURL:       first(config.TPExBaseURL, defaultTPExBaseURL),
		finMindURL:        first(config.FinMindURL, defaultFinMindURL),
		twseReportBaseURL: first(config.TWSEReportBaseURL, "https://www.twse.com.tw"),
		tpexReportBaseURL: first(config.TPExReportBaseURL, "https://www.tpex.org.tw"),
		cashflowBaseURL:   first(config.CashflowBaseURL, defaultCashflowBaseURL),
		cashflowCacheDir:  config.CashflowCacheDir,
		syncCashflow:      config.SyncCashflow,
		bgCtx:             bgCtx,
		bgCancel:          bgCancel,
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

// Close cancels any running background worker and cleans up process-owned resources.
func (c *Client) Close() error {
	c.closeOnce.Do(func() {
		if c.bgCancel != nil {
			c.bgCancel()
		}
	})
	return nil
}

// AwaitCashflowFill blocks until any in-progress background cashflow fill finishes,
// or until ctx expires. If no fill is running, it returns immediately.
func (c *Client) AwaitCashflowFill(ctx context.Context) error {
	c.cashflowMu.Lock()
	if !c.cashflowFilling {
		c.cashflowMu.Unlock()
		return nil
	}
	done := c.cashflowFillDone
	c.cashflowMu.Unlock()
	if done == nil {
		return nil
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
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
