package taiwan

import (
	"archive/zip"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"easy-stock/backend/internal/foundation"
)

// ==================================================
// M7H — Screener bulk cash-flow reader (operating_cash_flow / cash_flow_to_net_income)
// ==================================================
//
// Source: MOPS' official quarterly XBRL bulk archive (https://mopsov.twse.com.tw/mops/web/t203sb02,
// download transport GET /server-java/FileDownLoad?...&fileName=tifrs-{YYYY}Q{N}.zip) — a genuinely
// different official source from every other Screener domain in this package (all of which read TWSE
// OpenAPI / TPEx bulk JSON). One archive covers TWSE + TPEx + all six statement categories in a single
// download (TRUE BULK), so this reader is completely independent of ScreenerFinancials/ScreenerBalance:
// different discovery mechanism, different cache lifecycle (a single combined cashflowSnapshot on
// Client, see client.go), different resource model (a temporary on-disk ZIP, never fully expanded).
//
// PIT note: no filing/acceptance timestamp exists anywhere in this archive (M7H.2/M7H.3) — historical
// PIT remains BLOCKED, same as every other Taiwan fundamentals domain.

const (
	cashflowCandidateWindow    = 4    // M7H.3 §5: covers one full year, strictly bounded.
	cashflowCoverageThreshold  = 0.80 // guard against stub/incomplete archives, not a claim that 20% missing data is acceptable.
	cashflowAvailableTTL       = 7 * 24 * time.Hour
	cashflowUnavailableTTL     = 15 * time.Minute
	cashflowMaxArchiveBytes    = 512 << 20        // generous bound on a single quarterly archive (observed ~110-130MB).
	cashflowArchiveHTTPTimeout = 90 * time.Second // a ~110-130MB archive routinely takes >15s; see fetchCashflowCandidateZIP.
	cashflowOCFConcept         = "ifrs-full:CashFlowsFromUsedInOperatingActivities"
	cashflowProfitLossConcept  = "ifrs-full:ProfitLoss"
)

// errCashflowCandidateInvalid marks a candidate archive as unusable (network/HTTP/Content-Type/ZIP-open
// failure, or a completeness check that did not pass) — the caller moves on to the next candidate; it
// is never treated as a fatal Go error.
var errCashflowCandidateInvalid = errors.New("cashflow candidate archive invalid")

var cashflowCandidatePattern = regexp.MustCompile(`fileName=tifrs-(\d{4})Q([1-4])\.zip`)
var cashflowIssuerFilePattern = regexp.MustCompile(`^tifrs-fr1-m\d+-([a-z]+)-(cr|ir)-([0-9A-Za-z]+)-(\d{4})Q([1-4])\.html$`)

// cashflowRow is the provider-internal normalized result of parsing one issuer's selected (CR
// preferred, else IR — see parseCashflowArchive) MOPS XBRL report. ProfitLoss never leaves this
// package — it exists solely to compute CashFlowToNetIncome before the public
// foundation.FinancialStatementPeriod row is built (see ScreenerCashflow).
type cashflowRow struct {
	Canonical         string `json:"canonical"`
	Category          string `json:"category"`
	FiscalYear        int    `json:"fiscal_year"`
	FiscalQuarter     int    `json:"fiscal_quarter"`
	OperatingCashFlow *int64 `json:"operating_cash_flow"`
	ProfitLoss        *int64 `json:"profit_loss"`
}

// cashflowSnapshot is the single combined cache entry for the M7H domain: the target period (Period)
// and its parsed data (Rows) are always mutually consistent because they are produced and stored
// together by one cashflowData() call — there is no separate "discovery" cache layer to fall out of
// sync with a separate "parsed data" cache layer.
type cashflowSnapshot struct {
	Period    string // "YYYY-QN"; "" when Status == "unavailable" (never fabricated).
	Rows      map[string]cashflowRow
	Status    string // "available" | "partial" | "unavailable"
	ExpiresAt time.Time
}

type cashflowCandidate struct {
	year, quarter int
}

// cashflowCacheSchemaVersion is incremented when the parsing logic changes in a way that invalidates
// previously cached results (e.g. new fields extracted, changed normalization). On load, a mismatch
// causes the cached file to be treated as a miss — the next network fetch will re-populate it.
const cashflowCacheSchemaVersion = 1

// cashflowCacheEnvelope is the JSON-serializable wrapper written to disk by saveCashflowCache.
type cashflowCacheEnvelope struct {
	SchemaVersion int                    `json:"schema_version"`
	Period        string                 `json:"period"`
	Status        string                 `json:"status"`
	ExpiresAt     time.Time              `json:"expires_at"`
	Rows          map[string]cashflowRow `json:"rows"`
}

// cashflowRow JSON tags — the struct already has exported fields, so json.Marshal works directly.
// We add tags via a shadow type to keep the original struct clean.

// loadCashflowCache attempts to read a previously persisted cashflow snapshot from disk. Returns nil
// (never an error) on any miss — corrupt file, schema mismatch, missing file, expired entry — so the
// caller falls through to the normal network path.
func (c *Client) loadCashflowCache(now time.Time) *cashflowSnapshot {
	if c.cashflowCacheDir == "" {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(c.cashflowCacheDir, "cashflow-snapshot.json"))
	if err != nil {
		return nil
	}
	var envelope cashflowCacheEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil
	}
	if envelope.SchemaVersion != cashflowCacheSchemaVersion {
		return nil
	}
	if !now.Before(envelope.ExpiresAt) {
		return nil
	}
	if envelope.Rows == nil {
		envelope.Rows = map[string]cashflowRow{}
	}
	return &cashflowSnapshot{
		Period:    envelope.Period,
		Status:    envelope.Status,
		ExpiresAt: envelope.ExpiresAt,
		Rows:      envelope.Rows,
	}
}

// saveCashflowCache persists a successful cashflow snapshot to disk using atomic temp+rename. Errors
// are silently ignored — a failed save simply means the next cold start will re-download, which is the
// existing behavior. Only "available" and "partial" snapshots are persisted; "unavailable" is not worth
// caching to disk (its 15-minute TTL is shorter than typical restart intervals).
func (c *Client) saveCashflowCache(snapshot *cashflowSnapshot) {
	if c.cashflowCacheDir == "" || snapshot == nil || snapshot.Status == "unavailable" {
		return
	}
	envelope := cashflowCacheEnvelope{
		SchemaVersion: cashflowCacheSchemaVersion,
		Period:        snapshot.Period,
		Status:        snapshot.Status,
		ExpiresAt:     snapshot.ExpiresAt,
		Rows:          snapshot.Rows,
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		return
	}
	if err := os.MkdirAll(c.cashflowCacheDir, 0o755); err != nil {
		return
	}
	target := filepath.Join(c.cashflowCacheDir, "cashflow-snapshot.json")
	temp, err := os.CreateTemp(c.cashflowCacheDir, "cashflow-snapshot-*.tmp")
	if err != nil {
		return
	}
	tempPath := temp.Name()
	if _, writeErr := temp.Write(data); writeErr != nil {
		temp.Close()
		os.Remove(tempPath)
		return
	}
	if closeErr := temp.Close(); closeErr != nil {
		os.Remove(tempPath)
		return
	}
	if renameErr := os.Rename(tempPath, target); renameErr != nil {
		os.Remove(tempPath)
	}
}

// ScreenerCashflow returns, for every TWSE/TPEx stock security present in the current target quarter's
// official MOPS XBRL bulk archive, the operating_cash_flow / cash_flow_to_net_income pair. Lazy-loaded
// by the httpapi layer only when a M7H filter/sort is actually requested (taiwanScreenerNeedsCashflow) —
// never called unconditionally, never per-security.
func (c *Client) ScreenerCashflow(ctx context.Context, now time.Time) ([]foundation.FinancialStatementPeriod, foundation.TaiwanFundamentalsDomainFreshness, error) {
	snapshot, err := c.cashflowData(ctx, now)
	if err != nil {
		return nil, foundation.TaiwanFundamentalsDomainFreshness{}, err
	}
	rows := make([]foundation.FinancialStatementPeriod, 0, len(snapshot.Rows))
	for _, row := range snapshot.Rows {
		rows = append(rows, foundation.FinancialStatementPeriod{
			Canonical:           row.Canonical,
			AccountingCategory:  row.Category,
			FiscalYear:          row.FiscalYear,
			FiscalQuarter:       row.FiscalQuarter,
			OperatingCashFlow:   row.OperatingCashFlow,
			CashFlowToNetIncome: cashflowRatio(row.OperatingCashFlow, row.ProfitLoss),
		})
	}
	if snapshot.Status == "unavailable" {
		return rows, foundation.TaiwanFundamentalsDomainFreshness{Status: "unavailable"}, nil
	}
	period := snapshot.Period
	return rows, foundation.TaiwanFundamentalsDomainFreshness{AsOf: &period, Status: snapshot.Status}, nil
}

// cashflowRatio computes operating_cash_flow / ProfitLoss * 100. Unlike the generic ratio() helper
// (fundamentals.go), which nulls any non-positive denominator — correct for total-assets/equity-style
// denominators that are structurally always positive — net income can legitimately be negative for a
// loss-making company, and a negative cash_flow_to_net_income ratio is a real, meaningful
// earnings-quality signal (M7H.3) that must not be silently hidden. Only an exact-zero denominator is
// unsafe (division by zero) and nulled. This intentionally does not change ratio()'s existing behavior
// for debt_ratio/debt_to_equity/current_ratio.
func cashflowRatio(numerator, denominator *int64) *float64 {
	if numerator == nil || denominator == nil || *denominator == 0 {
		return nil
	}
	v := float64(*numerator) / float64(*denominator) * 100
	return &v
}

// cashflowData returns the current cash-flow snapshot, reusing a fresh cached one (available, partial,
// or unavailable — all three are legitimate cacheable results, see cashflowUnavailableTTL) or
// performing exactly one bounded discovery+download+parse cycle when the cache is stale/empty.
// cashflowMu is held across the entire sequence (mirroring chipDay's existing tight-locking pattern in
// chip.go, not fundamentalsRows' looser check-then-fetch one), so concurrent cold callers never each
// trigger a duplicate 110-130MB download — they serialize and then all reuse the one result.
//
// Persistent cache: when CashflowCacheDir is configured, a successful network result is saved to disk
// as JSON, and on cold start the disk cache is checked before falling through to the network path.
func (c *Client) cashflowData(ctx context.Context, now time.Time) (*cashflowSnapshot, error) {
	c.cashflowMu.Lock()
	defer c.cashflowMu.Unlock()
	if c.cashflowSnapshot != nil && now.Before(c.cashflowSnapshot.ExpiresAt) {
		return c.cashflowSnapshot, nil
	}
	// Cold start: try disk cache before expensive network fetch.
	if c.cashflowSnapshot == nil {
		if cached := c.loadCashflowCache(now); cached != nil {
			c.cashflowSnapshot = cached
			return cached, nil
		}
	}
	snapshot, err := c.discoverAndParseCashflow(ctx, now)
	if err != nil {
		// Fatal/local failure (context cancellation, Directory() dependency failure, temp-file resource
		// failure) — never cache this as "official source unavailable"; propagate the real error.
		return nil, err
	}
	if snapshot.Status == "unavailable" {
		snapshot.ExpiresAt = now.Add(cashflowUnavailableTTL)
	} else {
		snapshot.ExpiresAt = now.Add(cashflowAvailableTTL)
	}
	c.cashflowSnapshot = snapshot
	c.saveCashflowCache(snapshot)
	return snapshot, nil
}

// discoverAndParseCashflow performs the bounded discovery+download+parse cycle: fetch t203sb02, take
// at most cashflowCandidateWindow candidates (newest-to-oldest), and for each attempt a download+
// validation+completeness check until one succeeds. Every "official source failure" case (network,
// fake-200 HTML, invalid ZIP, no candidate complete enough) returns a Status:"unavailable" snapshot
// with a nil error — only a genuine local/fatal error (context cancellation, Directory() failure,
// temp-file creation failure) is returned as a real Go error.
func (c *Client) discoverAndParseCashflow(ctx context.Context, now time.Time) (*cashflowSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	identities, err := c.Directory(ctx)
	if err != nil {
		// Same systemic-dependency failure every other ScreenerXxx reader in this package propagates
		// directly (ScreenerFinancials/ScreenerBalance do the same) — not specifically a cash-flow-
		// source problem, so it is not negatively cached under the M7H-specific TTL.
		return nil, err
	}
	eligible := eligibleCashflowIdentities(identities)
	eligibleTotal := len(eligible["TWSE"]) + len(eligible["TPEX"])
	if eligibleTotal == 0 {
		return &cashflowSnapshot{Status: "unavailable"}, nil
	}

	candidates, err := c.fetchCashflowCandidates(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return &cashflowSnapshot{Status: "unavailable"}, nil
	}
	if len(candidates) > cashflowCandidateWindow {
		candidates = candidates[:cashflowCandidateWindow]
	}

	for _, candidate := range candidates {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		path, err := c.fetchCashflowCandidateZIP(ctx, candidate)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if errors.Is(err, errCashflowCandidateInvalid) {
				continue
			}
			return nil, err // local resource failure (e.g. os.CreateTemp) — fatal, propagate.
		}
		snapshot, ok := parseCashflowArchive(path, candidate, eligible, eligibleTotal)
		os.Remove(path)
		if ok {
			return snapshot, nil
		}
	}
	return &cashflowSnapshot{Status: "unavailable"}, nil
}

// eligibleCashflowIdentities builds the "eligible Screener stock universe" used by the completeness
// check below — non-ETF stock identities only, consistent with the existing ETF-unsupported policy for
// every other fundamentals domain (Fundamentals()/ScreenerRevenue already mark ETF financial-statement
// data as not applicable).
func eligibleCashflowIdentities(identities []foundation.SecurityIdentity) map[string]map[string]foundation.SecurityIdentity {
	allow := map[string]map[string]foundation.SecurityIdentity{"TWSE": {}, "TPEX": {}}
	for _, identity := range identities {
		if identity.Type == foundation.SecurityTypeETF {
			continue
		}
		if exchange, ok := allow[identity.Exchange]; ok {
			exchange[identity.Code] = identity
		}
	}
	return allow
}

// fetchCashflowCandidates fetches t203sb02 and parses every fileName=tifrs-{YYYY}Q{N}.zip reference
// into a deduplicated, newest-to-oldest sorted candidate list. Never infers the latest quarter from the
// current date — only from what the official page itself lists.
func (c *Client) fetchCashflowCandidates(ctx context.Context) ([]cashflowCandidate, error) {
	u := c.cashflowBaseURL + "/mops/web/t203sb02"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("t203sb02 HTTP status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxDirectoryBytes))
	if err != nil {
		return nil, err
	}
	matches := cashflowCandidatePattern.FindAllStringSubmatch(string(body), -1)
	seen := map[cashflowCandidate]bool{}
	var candidates []cashflowCandidate
	for _, m := range matches {
		year, yearErr := strconv.Atoi(m[1])
		quarter, quarterErr := strconv.Atoi(m[2])
		if yearErr != nil || quarterErr != nil {
			continue
		}
		candidate := cashflowCandidate{year: year, quarter: quarter}
		if seen[candidate] {
			continue
		}
		seen[candidate] = true
		candidates = append(candidates, candidate)
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].year != candidates[j].year {
			return candidates[i].year > candidates[j].year
		}
		return candidates[i].quarter > candidates[j].quarter
	})
	return candidates, nil
}

// cashflowArchiveHTTPClient returns an HTTP client suitable for downloading a full quarterly cash-flow
// archive — a shallow copy of c.httpClient (preserving Transport, CheckRedirect, Jar, and any other
// configured http.Client-level behavior) with only Timeout raised to a value sized for a ~110-130MB
// payload instead of c.httpClient's small-JSON-call default. c.httpClient itself is never mutated.
func (c *Client) cashflowArchiveHTTPClient() *http.Client {
	client := *c.httpClient
	client.Timeout = cashflowArchiveHTTPTimeout
	return &client
}

// fetchCashflowCandidateZIP downloads one candidate archive to a temporary file and validates it:
// HTTP 200, a fast-path rejection of an obvious HTML/text error body (M7H.3 observed a real HTTP-200
// fake-ZIP case), and — authoritatively, regardless of the exact Content-Type value — a successful
// zip.OpenReader. The returned path is only non-empty on success; the caller is responsible for
// removing it after use.
func (c *Client) fetchCashflowCandidateZIP(ctx context.Context, candidate cashflowCandidate) (string, error) {
	fileName := fmt.Sprintf("tifrs-%04dQ%d.zip", candidate.year, candidate.quarter)
	u := fmt.Sprintf("%s/server-java/FileDownLoad?step=9&functionName=show_file2&fileName=%s&filePath=/ifrs/%04d/", c.cashflowBaseURL, fileName, candidate.year)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	// c.httpClient's Timeout (15s by default, see NewClient) is sized for the small JSON API calls used
	// everywhere else in this package — a real ~110-130MB quarterly archive routinely takes longer than
	// that to download, which silently rejected the newest genuinely complete candidate (observed live:
	// 2026-Q2 timed out under the 15s default while an older, smaller candidate happened to finish in
	// time). This request therefore uses its own client with a longer Timeout, reusing c.httpClient's
	// Transport so any custom transport configuration (proxies, TLS, etc.) still applies — c.httpClient
	// itself is left completely unchanged for every other endpoint.
	resp, err := c.cashflowArchiveHTTPClient().Do(req)
	if err != nil {
		return "", errCashflowCandidateInvalid
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", errCashflowCandidateInvalid
	}
	if ct := resp.Header.Get("Content-Type"); strings.Contains(ct, "text/html") || strings.Contains(ct, "text/plain") {
		return "", errCashflowCandidateInvalid
	}

	temp, err := os.CreateTemp("", "cashflow-*.zip")
	if err != nil {
		return "", err // local resource failure — fatal, propagate.
	}
	path := temp.Name()
	_, copyErr := io.Copy(temp, io.LimitReader(resp.Body, cashflowMaxArchiveBytes))
	closeErr := temp.Close()
	if copyErr != nil {
		os.Remove(path)
		return "", errCashflowCandidateInvalid
	}
	if closeErr != nil {
		os.Remove(path)
		return "", closeErr
	}
	reader, openErr := zip.OpenReader(path)
	if openErr != nil {
		os.Remove(path)
		return "", errCashflowCandidateInvalid
	}
	reader.Close()
	return path, nil
}

// cashflowIssuerSelection is the CR-preferred-else-IR selection for one issuer code, resolved purely
// from ZIP entry filenames (no content read yet) — see parseCashflowArchive.
type cashflowIssuerSelection struct {
	file     *zip.File
	category string
	isCR     bool
}

// parseCashflowArchive opens the validated candidate ZIP, resolves CR/IR duplicates deterministically,
// joins against the eligible identity universe, applies the completeness threshold, and — only once
// the candidate is confirmed complete enough — parses each selected issuer's iXBRL document. Returns
// ok=false (never an error) when this candidate does not pass the completeness check, so the caller
// moves on to the next (older) candidate.
func parseCashflowArchive(path string, candidate cashflowCandidate, eligible map[string]map[string]foundation.SecurityIdentity, eligibleTotal int) (*cashflowSnapshot, bool) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, false
	}
	defer reader.Close()

	bestByCode := map[string]cashflowIssuerSelection{}
	for _, f := range reader.File {
		m := cashflowIssuerFilePattern.FindStringSubmatch(f.Name)
		if m == nil {
			continue // does not match the recognized issuer-report filename shape; ignored, not counted.
		}
		category, reportType, code := m[1], m[2], m[3]
		isCR := reportType == "cr"
		if existing, ok := bestByCode[code]; ok && (existing.isCR || !isCR) {
			continue // CR already selected for this code, or a duplicate IR already seen — first wins.
		}
		bestByCode[code] = cashflowIssuerSelection{file: f, category: category, isCR: isCR}
	}

	joined := map[string]cashflowIssuerSelection{} // canonical -> selected report
	for code, sel := range bestByCode {
		identity, ok := eligible["TWSE"][code]
		if !ok {
			identity, ok = eligible["TPEX"][code]
		}
		if !ok {
			continue // not part of this project's eligible stock universe — never counted.
		}
		joined[identity.Canonical] = sel
	}
	if float64(len(joined)) < cashflowCoverageThreshold*float64(eligibleTotal) {
		return nil, false
	}

	startDate := fmt.Sprintf("%04d-01-01", candidate.year)
	endDate := cashflowQuarterEnd(candidate.year, candidate.quarter)

	rows := map[string]cashflowRow{}
	attempted, succeeded := 0, 0
	for canonical, sel := range joined {
		attempted++
		rc, openErr := sel.file.Open()
		if openErr != nil {
			continue // document-level failure; counted against attempted, not removed from it.
		}
		ocf, profitLoss, parseErr := parseCashflowIssuerDocument(rc, startDate, endDate)
		rc.Close()
		if parseErr != nil {
			continue // structurally malformed document — this issuer's fields stay absent, others unaffected.
		}
		succeeded++
		rows[canonical] = cashflowRow{
			Canonical: canonical, Category: sel.category,
			FiscalYear: candidate.year, FiscalQuarter: candidate.quarter,
			OperatingCashFlow: ocf, ProfitLoss: profitLoss,
		}
	}
	if succeeded == 0 {
		return nil, false // archive valid but zero usable issuer reports — treat as unavailable, try older.
	}
	status := "available"
	if succeeded < attempted {
		status = "partial"
	}
	period := fmt.Sprintf("%d-Q%d", candidate.year, candidate.quarter)
	return &cashflowSnapshot{Period: period, Rows: rows, Status: status}, true
}

func cashflowQuarterEnd(year, quarter int) string {
	switch quarter {
	case 1:
		return fmt.Sprintf("%04d-03-31", year)
	case 2:
		return fmt.Sprintf("%04d-06-30", year)
	case 3:
		return fmt.Sprintf("%04d-09-30", year)
	default:
		return fmt.Sprintf("%04d-12-31", year)
	}
}

// ixContext is the narrow subset of an <xbrli:context> this reader cares about: the exact duration
// (startDate/endDate) and whether it carries a <xbrli:scenario>/<xbrli:segment> dimensional qualifier
// (M7H.3 §6/§11/§21: a context with either present is a sub-component breakdown, e.g. an equity
// roll-forward member, never the plain consolidated total).
type ixContext struct {
	startDate, endDate string
	dimensional        bool
}

// ixFact is the narrow subset of an <ix:nonFraction> fact this reader cares about.
type ixFact struct {
	contextRef, unitRef, scale, sign, decimals, format, nilFlag, text string
}

// parseCashflowIssuerDocument is the narrow iXBRL parser: it streams the document token-by-token,
// collecting every <xbrli:context> definition and every <ix:nonFraction> fact for exactly the two
// concepts this domain needs, then resolves each concept to the single fact whose context is the
// non-dimensional base context matching the target fiscal period exactly (never "first matching
// concept" — M7H.3 §6/§11 proved that trap picks up equity roll-forward member values, including 0).
// A structurally malformed document (the XML itself cannot be tokenized) returns a non-nil error —
// that is the only case that counts as an issuer-level parse failure; a missing concept, an
// xsi:nil="true" fact, or an unsupported numeric format all resolve to a nil field on an otherwise
// successfully parsed document (never an error here).
func parseCashflowIssuerDocument(r io.Reader, startDate, endDate string) (ocf *int64, profitLoss *int64, err error) {
	decoder := xml.NewDecoder(r)
	decoder.Strict = false
	decoder.AutoClose = xml.HTMLAutoClose
	decoder.Entity = xml.HTMLEntity

	contexts := map[string]ixContext{}
	type conceptFact struct {
		concept string
		fact    ixFact
	}
	var facts []conceptFact

	for {
		tok, tokErr := decoder.Token()
		if tokErr == io.EOF {
			break
		}
		if tokErr != nil {
			return nil, nil, tokErr
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		switch start.Name.Local {
		case "context":
			var raw struct {
				ID     string `xml:"id,attr"`
				Period struct {
					StartDate string `xml:"startDate"`
					EndDate   string `xml:"endDate"`
				} `xml:"period"`
				Scenario *struct{} `xml:"scenario"`
				Segment  *struct{} `xml:"segment"`
			}
			if decodeErr := decoder.DecodeElement(&raw, &start); decodeErr != nil {
				continue // one malformed context is skipped, not fatal to the whole document.
			}
			contexts[raw.ID] = ixContext{
				startDate:   raw.Period.StartDate,
				endDate:     raw.Period.EndDate,
				dimensional: raw.Scenario != nil || raw.Segment != nil,
			}
		case "nonFraction":
			var raw struct {
				Name       string `xml:"name,attr"`
				ContextRef string `xml:"contextRef,attr"`
				UnitRef    string `xml:"unitRef,attr"`
				Scale      string `xml:"scale,attr"`
				Sign       string `xml:"sign,attr"`
				Decimals   string `xml:"decimals,attr"`
				Format     string `xml:"format,attr"`
				Nil        string `xml:"nil,attr"`
				Text       string `xml:",chardata"`
			}
			if decodeErr := decoder.DecodeElement(&raw, &start); decodeErr != nil {
				continue
			}
			if raw.Name != cashflowOCFConcept && raw.Name != cashflowProfitLossConcept {
				continue // ignore every other concept — this is a narrow parser, not a general engine.
			}
			facts = append(facts, conceptFact{concept: raw.Name, fact: ixFact{
				contextRef: raw.ContextRef, unitRef: raw.UnitRef, scale: raw.Scale,
				sign: raw.Sign, decimals: raw.Decimals, format: raw.Format,
				nilFlag: raw.Nil, text: raw.Text,
			}})
		}
	}

	for _, entry := range facts {
		ctx, ok := contexts[entry.fact.contextRef]
		if !ok || ctx.dimensional || ctx.startDate != startDate || ctx.endDate != endDate {
			continue
		}
		value := parseIXBRLThousandTWD(entry.fact)
		switch entry.concept {
		case cashflowOCFConcept:
			if ocf == nil {
				ocf = value
			}
		case cashflowProfitLossConcept:
			if profitLoss == nil {
				profitLoss = value
			}
		}
	}
	return ocf, profitLoss, nil
}

// parseIXBRLThousandTWD converts one <ix:nonFraction> fact directly into this project's public
// thousand-TWD monetary convention. Verified MOPS facts always carry scale="3" (M7H.2/M7H.3), in which
// case the displayed integer already IS the thousand-TWD magnitude — no multiply-then-divide detour
// through an absolute-TWD intermediate. Any other integer scale is handled via an explicit,
// overflow-checked power-of-ten adjustment (scaleToThousandTWD); an unrepresentable result, an
// unsupported numeric format, or xsi:nil="true" all resolve to nil (field-level, never a document
// failure, never a silently wrapped/Inf/NaN value).
func parseIXBRLThousandTWD(fact ixFact) *int64 {
	if strings.EqualFold(fact.nilFlag, "true") {
		return nil
	}
	text := strings.TrimSpace(fact.text)
	if text == "" {
		return nil
	}
	if fact.format != "" && fact.format != "ixt:numdotdecimal" {
		return nil
	}
	cleaned := strings.ReplaceAll(text, ",", "")
	parsed, err := strconv.ParseInt(cleaned, 10, 64)
	if err != nil {
		return nil
	}
	scale := 3
	if fact.scale != "" {
		s, scaleErr := strconv.Atoi(fact.scale)
		if scaleErr != nil {
			return nil
		}
		scale = s
	}
	normalized, safe := scaleToThousandTWD(parsed, scale-3)
	if !safe {
		return nil
	}
	if strings.EqualFold(fact.sign, "-") {
		normalized = -normalized
	}
	return &normalized
}

// scaleToThousandTWD applies 10^exponent to value with explicit overflow checks — it never silently
// wraps. exponent == 0 (the verified scale="3" case) is a pure pass-through.
func scaleToThousandTWD(value int64, exponent int) (int64, bool) {
	if exponent == 0 {
		return value, true
	}
	if exponent > 0 {
		for i := 0; i < exponent; i++ {
			if value > 0 && value > math.MaxInt64/10 {
				return 0, false
			}
			if value < 0 && value < math.MinInt64/10 {
				return 0, false
			}
			value *= 10
		}
		return value, true
	}
	for i := 0; i < -exponent; i++ {
		value /= 10
	}
	return value, true
}
