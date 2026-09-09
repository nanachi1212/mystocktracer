package taiwan

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"easy-stock/backend/internal/foundation"
)

// ==================================================
// P5.5B.2: Cashflow Backend-Owned Background Cache Fill Tests (A ~ J)
// ==================================================

// TestCashflowBackground_A_CacheHit verifies that when cache is already warm (in-memory),
// ScreenerCashflow returns immediately without starting any background HTTP requests.
func TestCashflowBackground_A_CacheHit(t *testing.T) {
	var downloadCalls, discoveryCalls int64
	server := cashflowTestServer(t, threeIssuerZip(t), &downloadCalls, &discoveryCalls)
	defer server.Close()

	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	c := NewClient(Config{
		TWSEBaseURL:     server.URL,
		TPExBaseURL:     server.URL,
		CashflowBaseURL: server.URL,
		HTTPClient:      server.Client(),
		Now:             func() time.Time { return now },
		SyncCashflow:    false, // async mode
	})

	// Pre-populate in-memory cache
	c.cashflowSnapshot = &cashflowSnapshot{
		Period:    "2026-Q2",
		Status:    "available",
		ExpiresAt: now.Add(cashflowAvailableTTL),
		Rows: map[string]cashflowRow{
			"2330.TWSE": {
				Canonical:         "2330.TWSE",
				Category:          "ci",
				FiscalYear:        2026,
				FiscalQuarter:     2,
				OperatingCashFlow: int64Ptr(1122637757),
				ProfitLoss:        int64Ptr(758226085),
			},
		},
	}

	start := time.Now()
	rows, freshness, err := c.ScreenerCashflow(context.Background(), now)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if freshness.Status != "available" {
		t.Fatalf("expected available, got %s", freshness.Status)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if elapsed > 100*time.Millisecond {
		t.Fatalf("cache hit took too long: %v", elapsed)
	}
	if downloadCalls != 0 || discoveryCalls != 0 {
		t.Fatalf("expected 0 HTTP calls on cache hit, got discoveries=%d downloads=%d", discoveryCalls, downloadCalls)
	}
}

// TestCashflowBackground_B_CacheMissFastReturnAndTrigger verifies that on a cold cache miss,
// the caller returns immediately (< 100ms) with unavailable status, and a background fill is triggered.
func TestCashflowBackground_B_CacheMissFastReturnAndTrigger(t *testing.T) {
	var downloadCalls, discoveryCalls int64
	server := cashflowTestServer(t, threeIssuerZip(t), &downloadCalls, &discoveryCalls)
	defer server.Close()

	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	c := NewClient(Config{
		TWSEBaseURL:     server.URL,
		TPExBaseURL:     server.URL,
		CashflowBaseURL: server.URL,
		HTTPClient:      server.Client(),
		Now:             func() time.Time { return now },
		SyncCashflow:    false,
	})

	start := time.Now()
	rows, freshness, err := c.ScreenerCashflow(context.Background(), now)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error on cold miss: %v", err)
	}
	if elapsed > 100*time.Millisecond {
		t.Fatalf("cold miss took too long: %v (must not block on download)", elapsed)
	}
	if freshness.Status != "unavailable" {
		t.Fatalf("expected unavailable status on cold miss, got %s", freshness.Status)
	}
	if len(rows) != 0 {
		t.Fatalf("expected 0 rows on cold miss, got %d", len(rows))
	}

	// Wait for the background fill to finish
	if err := c.AwaitCashflowFill(context.Background()); err != nil {
		t.Fatalf("AwaitCashflowFill: %v", err)
	}

	if atomic.LoadInt64(&downloadCalls) != 1 || atomic.LoadInt64(&discoveryCalls) != 1 {
		t.Fatalf("expected background fill to complete 1 discovery + 1 download, got discoveries=%d downloads=%d",
			atomic.LoadInt64(&discoveryCalls), atomic.LoadInt64(&downloadCalls))
	}
}

// TestCashflowBackground_C_20ConcurrentCallersSingleDownload verifies that 20 concurrent
// callers hitting a cold cache all return quickly, and trigger exactly 1 archive download.
func TestCashflowBackground_C_20ConcurrentCallersSingleDownload(t *testing.T) {
	var downloadCalls, discoveryCalls int64
	zipBytes := threeIssuerZip(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/opendata/t187ap03_L", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[{"公司代號":"2330","公司簡稱":"台積電","公司名稱":"台積電","產業別":"半導體業","上市日期":"1994/09/05"}]`)
	})
	mux.HandleFunc("/opendata/t187ap47_L", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `[]`) })
	mux.HandleFunc("/mopsfin_t187ap03_O", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `[]`) })
	mux.HandleFunc("/mops/web/t203sb02", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&discoveryCalls, 1)
		fmt.Fprint(w, `<a onclick="...fileName=tifrs-2026Q2.zip...">x</a>`)
	})
	mux.HandleFunc("/server-java/FileDownLoad", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&downloadCalls, 1)
		time.Sleep(50 * time.Millisecond) // simulate download latency
		w.Header().Set("Content-Type", "application/x-zip-compressed")
		w.Write(zipBytes)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	c := NewClient(Config{
		TWSEBaseURL:     server.URL,
		TPExBaseURL:     server.URL,
		CashflowBaseURL: server.URL,
		HTTPClient:      server.Client(),
		Now:             func() time.Time { return now },
		SyncCashflow:    false,
	})

	const concurrency = 20
	var wg sync.WaitGroup
	var slowCallers int64

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			start := time.Now()
			_, freshness, err := c.ScreenerCashflow(context.Background(), now)
			if err != nil {
				t.Errorf("caller error: %v", err)
			}
			if freshness.Status != "unavailable" {
				t.Errorf("expected unavailable status, got %s", freshness.Status)
			}
			if time.Since(start) > 40*time.Millisecond {
				atomic.AddInt64(&slowCallers, 1)
			}
		}()
	}
	wg.Wait()

	if slowCallers > 0 {
		t.Errorf("%d callers were blocked by background download", slowCallers)
	}

	// Wait for background worker to complete
	if err := c.AwaitCashflowFill(context.Background()); err != nil {
		t.Fatalf("AwaitCashflowFill: %v", err)
	}

	if atomic.LoadInt64(&downloadCalls) != 1 {
		t.Fatalf("expected exactly 1 download call across 20 concurrent callers, got %d", atomic.LoadInt64(&downloadCalls))
	}
}

// TestCashflowBackground_D_CallerCancelDoesNotAbortBackground verifies that canceling the
// caller request context does NOT cancel the background cache fill.
func TestCashflowBackground_D_CallerCancelDoesNotAbortBackground(t *testing.T) {
	var downloadCalls, discoveryCalls int64
	zipBytes := threeIssuerZip(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/opendata/t187ap03_L", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[{"公司代號":"2330","公司簡稱":"台積電","公司名稱":"台積電","產業別":"半導體業","上市日期":"1994/09/05"}]`)
	})
	mux.HandleFunc("/opendata/t187ap47_L", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `[]`) })
	mux.HandleFunc("/mopsfin_t187ap03_O", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `[]`) })
	mux.HandleFunc("/mops/web/t203sb02", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&discoveryCalls, 1)
		fmt.Fprint(w, `<a onclick="...fileName=tifrs-2026Q2.zip...">x</a>`)
	})
	mux.HandleFunc("/server-java/FileDownLoad", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&downloadCalls, 1)
		time.Sleep(60 * time.Millisecond) // slow download
		w.Header().Set("Content-Type", "application/x-zip-compressed")
		w.Write(zipBytes)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	c := NewClient(Config{
		TWSEBaseURL:     server.URL,
		TPExBaseURL:     server.URL,
		CashflowBaseURL: server.URL,
		HTTPClient:      server.Client(),
		Now:             func() time.Time { return now },
		SyncCashflow:    false,
	})

	// Caller with an already-canceled context
	callerCtx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately!

	_, _, _ = c.ScreenerCashflow(callerCtx, now)

	// Background worker should continue to completion despite caller cancellation
	fillCtx, fillCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer fillCancel()
	if err := c.AwaitCashflowFill(fillCtx); err != nil {
		t.Fatalf("background fill failed to complete after caller cancellation: %v", err)
	}

	// Check that cache is now populated and fresh
	c.cashflowMu.Lock()
	snap := c.cashflowSnapshot
	c.cashflowMu.Unlock()

	if snap == nil || snap.Status != "available" {
		t.Fatalf("expected background fill to complete and populate available snapshot, got %+v", snap)
	}
}

// TestCashflowBackground_E_ProcessLifecycleCancelCleansTemp verifies that when the process/Client
// is closed, the background fill is canceled, temp files are cleaned up, and invalid cache is not saved.
func TestCashflowBackground_E_ProcessLifecycleCancelCleansTemp(t *testing.T) {
	startedDownload := make(chan struct{})

	mux := http.NewServeMux()
	mux.HandleFunc("/opendata/t187ap03_L", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[{"公司代號":"2330","公司簡稱":"台積電","公司名稱":"台積電","產業別":"半導體業","上市日期":"1994/09/05"}]`)
	})
	mux.HandleFunc("/opendata/t187ap47_L", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `[]`) })
	mux.HandleFunc("/mopsfin_t187ap03_O", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `[]`) })
	mux.HandleFunc("/mops/web/t203sb02", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<a onclick="...fileName=tifrs-2026Q2.zip...">x</a>`)
	})
	mux.HandleFunc("/server-java/FileDownLoad", func(w http.ResponseWriter, r *http.Request) {
		close(startedDownload)
		// Block until canceled by client
		<-r.Context().Done()
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	cacheDir := t.TempDir()
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	c := NewClient(Config{
		TWSEBaseURL:      server.URL,
		TPExBaseURL:      server.URL,
		CashflowBaseURL:  server.URL,
		HTTPClient:       server.Client(),
		CashflowCacheDir: cacheDir,
		Now:              func() time.Time { return now },
		SyncCashflow:     false,
	})

	// Trigger background fill
	_, _, _ = c.ScreenerCashflow(context.Background(), now)

	// Wait until download has started on server
	<-startedDownload

	// Shutdown process / client
	c.Close()

	// Wait for background worker to terminate
	_ = c.AwaitCashflowFill(context.Background())

	// Confirm no snapshot was saved to disk
	cacheFile := filepath.Join(cacheDir, "cashflow-snapshot.json")
	if _, err := os.Stat(cacheFile); err == nil {
		t.Fatalf("expected cache file NOT to be created on cancel, but it exists")
	}

	// Confirm memory snapshot was not set to available
	c.cashflowMu.Lock()
	snap := c.cashflowSnapshot
	c.cashflowMu.Unlock()
	if snap != nil && snap.Status == "available" {
		t.Fatalf("expected memory snapshot not to be marked available on cancel")
	}
}

// TestCashflowBackground_F_SuccessfulFillPublishesMemoryAndDisk verifies that after a successful
// fill, the snapshot is published to both memory and disk, and subsequent requests hit the cache.
func TestCashflowBackground_F_SuccessfulFillPublishesMemoryAndDisk(t *testing.T) {
	var downloadCalls, discoveryCalls int64
	server := cashflowTestServer(t, threeIssuerZip(t), &downloadCalls, &discoveryCalls)
	defer server.Close()

	cacheDir := t.TempDir()
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	c := NewClient(Config{
		TWSEBaseURL:      server.URL,
		TPExBaseURL:      server.URL,
		CashflowBaseURL:  server.URL,
		HTTPClient:       server.Client(),
		CashflowCacheDir: cacheDir,
		Now:              func() time.Time { return now },
		SyncCashflow:     false,
	})

	// 1. Initial miss: returns unavailable
	_, freshness1, err := c.ScreenerCashflow(context.Background(), now)
	if err != nil {
		t.Fatalf("initial call: %v", err)
	}
	if freshness1.Status != "unavailable" {
		t.Fatalf("expected unavailable, got %s", freshness1.Status)
	}

	// 2. Wait for fill to finish
	if err := c.AwaitCashflowFill(context.Background()); err != nil {
		t.Fatalf("AwaitCashflowFill: %v", err)
	}

	// 3. Verify disk file exists and is valid
	cacheFile := filepath.Join(cacheDir, "cashflow-snapshot.json")
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		t.Fatalf("expected persistent cache file on disk: %v", err)
	}
	var env cashflowCacheEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("invalid json in disk cache: %v", err)
	}
	if env.Status != "available" || env.Period != "2026-Q2" || len(env.Rows) != 3 {
		t.Fatalf("disk cache contents unexpected: status=%s period=%s rows=%d", env.Status, env.Period, len(env.Rows))
	}

	// 4. Second request in same process: hits memory cache, 0 new HTTP requests
	rows2, freshness2, err := c.ScreenerCashflow(context.Background(), now)
	if err != nil {
		t.Fatalf("warm call: %v", err)
	}
	if freshness2.Status != "available" || len(rows2) != 3 {
		t.Fatalf("expected available with 3 rows, got status=%s rows=%d", freshness2.Status, len(rows2))
	}
	if downloadCalls != 1 || discoveryCalls != 1 {
		t.Fatalf("expected no additional HTTP calls on warm request, got discoveries=%d downloads=%d", discoveryCalls, downloadCalls)
	}
}

// TestCashflowBackground_G_FailedFillEnforcesCooldown verifies that when fill fails, no cache is
// saved and a 15-minute cooldown is enforced preventing repeated downloads.
func TestCashflowBackground_G_FailedFillEnforcesCooldown(t *testing.T) {
	var downloadCalls, discoveryCalls int64
	mux := http.NewServeMux()
	mux.HandleFunc("/opendata/t187ap03_L", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[{"公司代號":"2330","公司簡稱":"台積電","公司名稱":"台積電","產業別":"半導體業","上市日期":"1994/09/05"}]`)
	})
	mux.HandleFunc("/opendata/t187ap47_L", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `[]`) })
	mux.HandleFunc("/mopsfin_t187ap03_O", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `[]`) })
	mux.HandleFunc("/mops/web/t203sb02", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&discoveryCalls, 1)
		fmt.Fprint(w, `<a onclick="...fileName=tifrs-2026Q2.zip...">x</a>`)
	})
	mux.HandleFunc("/server-java/FileDownLoad", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&downloadCalls, 1)
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "server error")
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	cacheDir := t.TempDir()
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	c := NewClient(Config{
		TWSEBaseURL:      server.URL,
		TPExBaseURL:      server.URL,
		CashflowBaseURL:  server.URL,
		HTTPClient:       server.Client(),
		CashflowCacheDir: cacheDir,
		Now:              func() time.Time { return now },
		SyncCashflow:     false,
	})

	// 1. Initial trigger: fails in background
	_, _, _ = c.ScreenerCashflow(context.Background(), now)
	if err := c.AwaitCashflowFill(context.Background()); err != nil {
		t.Fatalf("AwaitCashflowFill: %v", err)
	}

	// Confirm no disk file written
	cacheFile := filepath.Join(cacheDir, "cashflow-snapshot.json")
	if _, err := os.Stat(cacheFile); err == nil {
		t.Fatalf("expected no cache file written on failure")
	}

	downloadsAfterFirst := atomic.LoadInt64(&downloadCalls)
	if downloadsAfterFirst == 0 {
		t.Fatalf("expected at least 1 attempt to download")
	}

	// 2. Caller within cooldown (e.g. 5 mins later): should not trigger new download
	_, freshness, err := c.ScreenerCashflow(context.Background(), now.Add(5*time.Minute))
	if err != nil {
		t.Fatalf("cooldown call: %v", err)
	}
	if freshness.Status != "unavailable" {
		t.Fatalf("expected unavailable during cooldown, got %s", freshness.Status)
	}
	if atomic.LoadInt64(&downloadCalls) != downloadsAfterFirst {
		t.Fatalf("expected 0 new downloads during cooldown, got %d", atomic.LoadInt64(&downloadCalls))
	}
}

// TestCashflowBackground_H_CorruptDiskTriggersRebuild verifies that a corrupt cache file or schema
// mismatch triggers a background rebuild rather than returning corrupt data.
func TestCashflowBackground_H_CorruptDiskTriggersRebuild(t *testing.T) {
	var downloadCalls, discoveryCalls int64
	server := cashflowTestServer(t, threeIssuerZip(t), &downloadCalls, &discoveryCalls)
	defer server.Close()

	cacheDir := t.TempDir()
	corruptPath := filepath.Join(cacheDir, "cashflow-snapshot.json")
	if err := os.WriteFile(corruptPath, []byte("NOT_VALID_JSON{{{"), 0o644); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	c := NewClient(Config{
		TWSEBaseURL:      server.URL,
		TPExBaseURL:      server.URL,
		CashflowBaseURL:  server.URL,
		HTTPClient:       server.Client(),
		CashflowCacheDir: cacheDir,
		Now:              func() time.Time { return now },
		SyncCashflow:     false,
	})

	// 1. Corrupt file is treated as miss: returns unavailable immediately
	rows, freshness, err := c.ScreenerCashflow(context.Background(), now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if freshness.Status != "unavailable" || len(rows) != 0 {
		t.Fatalf("corrupt cache must not return valid data: status=%s rows=%d", freshness.Status, len(rows))
	}

	// 2. Wait for rebuild
	if err := c.AwaitCashflowFill(context.Background()); err != nil {
		t.Fatalf("AwaitCashflowFill: %v", err)
	}

	// 3. Verify disk file was overwritten with valid data
	data, err := os.ReadFile(corruptPath)
	if err != nil {
		t.Fatalf("read rebuilt cache: %v", err)
	}
	var env cashflowCacheEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("rebuilt cache is not valid JSON: %v", err)
	}
	if env.SchemaVersion != cashflowCacheSchemaVersion || env.Status != "available" {
		t.Fatalf("rebuilt cache invalid: version=%d status=%s", env.SchemaVersion, env.Status)
	}
}

// TestCashflowBackground_I_ScreenerAndIntelligenceShareFill verifies that Screener and Intelligence
// (statement) share the exact same background worker and do not trigger duplicate downloads.
func TestCashflowBackground_I_ScreenerAndIntelligenceShareFill(t *testing.T) {
	var downloadCalls, discoveryCalls int64
	zipBytes := threeIssuerZip(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/opendata/t187ap03_L", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[{"公司代號":"2330","公司簡稱":"台積電","公司名稱":"台積電","產業別":"半導體業","上市日期":"1994/09/05"}]`)
	})
	mux.HandleFunc("/opendata/t187ap47_L", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `[]`) })
	mux.HandleFunc("/mopsfin_t187ap03_O", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `[]`) })
	mux.HandleFunc("/opendata/t187ap05_L", func(w http.ResponseWriter, r *http.Request) {
		// TWSE income row
		fmt.Fprint(w, `[{"出表日期":"1150905","年度":"115","季別":"2","公司代號":"2330","營業收入":"1000000","本期稅前淨利（淨損）":"300000","本期稅後淨利（淨損）":"250000","淨利（淨損）歸屬於母公司業主":"248000","基本每股盈餘（元）":"9.55"}]`)
	})
	mux.HandleFunc("/opendata/t187ap06_L", func(w http.ResponseWriter, r *http.Request) {
		// TWSE balance row
		fmt.Fprint(w, `[{"出表日期":"1150905","年度":"115","季別":"1","公司代號":"2330","資產總計":"8000000","負債總計":"2500000","權益總計":"5500000","流動資產":"3000000","流動負債":"1200000","每股參考淨值":"28.5"}]`)
	})
	mux.HandleFunc("/mops/web/t203sb02", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&discoveryCalls, 1)
		fmt.Fprint(w, `<a onclick="...fileName=tifrs-2026Q2.zip...">x</a>`)
	})
	mux.HandleFunc("/server-java/FileDownLoad", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&downloadCalls, 1)
		time.Sleep(50 * time.Millisecond) // slow download
		w.Header().Set("Content-Type", "application/x-zip-compressed")
		w.Write(zipBytes)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	c := NewClient(Config{
		TWSEBaseURL:       server.URL,
		TPExBaseURL:       server.URL,
		CashflowBaseURL:   server.URL,
		TWSEReportBaseURL: server.URL,
		TPExReportBaseURL: server.URL,
		HTTPClient:        server.Client(),
		Now:               func() time.Time { return now },
		SyncCashflow:      false,
	})

	var wg sync.WaitGroup
	wg.Add(2)

	// Caller 1: Screener
	go func() {
		defer wg.Done()
		_, _, _ = c.ScreenerCashflow(context.Background(), now)
	}()

	// Caller 2: Intelligence (statement)
	go func() {
		defer wg.Done()
		s := foundation.SecurityIdentity{Canonical: "2330.TWSE", Code: "2330", Exchange: "TWSE"}
		_, _ = c.statement(context.Background(), s)
	}()

	wg.Wait()

	// Wait for background worker
	if err := c.AwaitCashflowFill(context.Background()); err != nil {
		t.Fatalf("AwaitCashflowFill: %v", err)
	}

	if atomic.LoadInt64(&downloadCalls) != 1 {
		t.Fatalf("expected exactly 1 download shared across Screener and Intelligence, got %d", atomic.LoadInt64(&downloadCalls))
	}
}

// TestCashflowBackground_J_DataSemanticsDuringPending verifies that while background fill is
// pending, statement returns non-cashflow fundamentals normally, leaving cashflow fields as nil.
func TestCashflowBackground_J_DataSemanticsDuringPending(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/opendata/t187ap03_L", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[{"公司代號":"2330","公司簡稱":"台積電","公司名稱":"台積電","產業別":"半導體業","上市日期":"1994/09/05"}]`)
	})
	mux.HandleFunc("/opendata/t187ap47_L", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `[]`) })
	mux.HandleFunc("/mopsfin_t187ap03_O", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `[]`) })
	mux.HandleFunc("/opendata/t187ap06_L_ci", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[{"出表日期":"1150905","年度":"115","季別":"2","公司代號":"2330","營業收入":"1000000","本期稅前淨利（淨損）":"300000","本期稅後淨利（淨損）":"250000","淨利（淨損）歸屬於母公司業主":"248000","基本每股盈餘（元）":"9.55"}]`)
	})
	mux.HandleFunc("/opendata/t187ap07_L_ci", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[{"出表日期":"1150905","年度":"115","季別":"1","公司代號":"2330","資產總計":"8000000","負債總計":"2500000","權益總計":"5500000","流動資產":"3000000","流動負債":"1200000","每股參考淨值":"28.5"}]`)
	})
	mux.HandleFunc("/mops/web/t203sb02", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond) // delay discovery
		fmt.Fprint(w, `<a onclick="...fileName=tifrs-2026Q2.zip...">x</a>`)
	})
	mux.HandleFunc("/server-java/FileDownLoad", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/x-zip-compressed")
		w.Write(threeIssuerZip(t))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	c := NewClient(Config{
		TWSEBaseURL:       server.URL,
		TPExBaseURL:       server.URL,
		CashflowBaseURL:   server.URL,
		TWSEReportBaseURL: server.URL,
		TPExReportBaseURL: server.URL,
		HTTPClient:        server.Client(),
		Now:               func() time.Time { return now },
		SyncCashflow:      false,
	})

	s := foundation.SecurityIdentity{Canonical: "2330.TWSE", Code: "2330", Exchange: "TWSE"}
	item, err := c.statement(context.Background(), s)
	if err != nil {
		t.Fatalf("statement error: %v", err)
	}

	// Non-cashflow fundamentals must be fully populated
	if item.Revenue == nil || *item.Revenue != 1000000000 { // 1M thousand-TWD = 1B TWD
		t.Fatalf("expected Revenue=1000000000, got %v", item.Revenue)
	}
	if item.TotalAssets == nil || *item.TotalAssets != 8000000000 {
		t.Fatalf("expected TotalAssets=8000000000, got %v", item.TotalAssets)
	}

	// Cashflow fields must be cleanly nil (never fabricated or zeroed)
	if item.OperatingCashFlow != nil {
		t.Fatalf("expected nil OperatingCashFlow while pending, got %v", *item.OperatingCashFlow)
	}
	if item.CashFlowToNetIncome != nil {
		t.Fatalf("expected nil CashFlowToNetIncome while pending, got %v", *item.CashFlowToNetIncome)
	}
	if item.CashflowStatus != "" {
		t.Fatalf("expected empty CashflowStatus while pending, got %s", item.CashflowStatus)
	}

	// Clean up background worker
	_ = c.AwaitCashflowFill(context.Background())
}
