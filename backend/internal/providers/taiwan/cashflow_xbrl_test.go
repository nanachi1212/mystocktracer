package taiwan

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"easy-stock/backend/internal/foundation"
)

// ==================================================
// Small local test helpers
// ==================================================

func int64Ptr(v int64) *int64       { return &v }
func float64Ptr(v float64) *float64 { return &v }

func assertInt64PtrEqual(t *testing.T, got, want *int64) {
	t.Helper()
	if (got == nil) != (want == nil) {
		t.Fatalf("got %v, want %v", ptrIntString(got), ptrIntString(want))
	}
	if got != nil && *got != *want {
		t.Fatalf("got %d, want %d", *got, *want)
	}
}

func assertFloat64PtrEqual(t *testing.T, got, want *float64) {
	t.Helper()
	if (got == nil) != (want == nil) {
		t.Fatalf("got %v, want %v", ptrFloatString(got), ptrFloatString(want))
	}
	if got != nil && *got != *want {
		t.Fatalf("got %v, want %v", *got, *want)
	}
}

func ptrIntString(v *int64) string {
	if v == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%d", *v)
}

func ptrFloatString(v *float64) string {
	if v == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%v", *v)
}

func cashflowTestIdentity(code, exchange string, isETF bool) foundation.SecurityIdentity {
	typ := foundation.SecurityTypeStock
	if isETF {
		typ = foundation.SecurityTypeETF
	}
	return foundation.SecurityIdentity{
		Canonical: code + "." + exchange, Code: code, Name: code, Exchange: exchange,
		Market: "TW", Type: typ, Currency: "TWD",
	}
}

// ==================================================
// iXBRL document fixture builder
// ==================================================

type ixbrlFixture struct {
	code          string
	year, quarter int

	ocfText, ocfScale, ocfSign, ocfFormat string
	ocfNil, omitOCF                       bool

	plText, plScale, plSign string
	plNil, omitProfitLoss   bool

	malformedXML bool
}

func quarterEndDate(year, quarter int) string {
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

func ixFactTag(name, contextRef, text, scale, sign, format string, isNil bool) string {
	if format == "" {
		format = "ixt:numdotdecimal"
	}
	attrs := fmt.Sprintf(`name="%s" contextRef="%s" unitRef="TWD" scale="%s" decimals="-3" format="%s"`, name, contextRef, scale, format)
	if sign != "" {
		attrs += fmt.Sprintf(` sign="%s"`, sign)
	}
	if isNil {
		attrs += ` xsi:nil="true"`
	}
	return fmt.Sprintf(`<ix:nonFraction %s>%s</ix:nonFraction>`, attrs, text)
}

// buildIXBRLDocument builds a minimal but structurally realistic iXBRL document: a base
// (non-dimensional) context for the current period, a prior-year comparative context, and a
// dimensional (equity-component) context carrying a decoy ProfitLoss fact of value 0 — proving the
// context-selection rule (not "first matching concept") is what the parser actually relies on.
func buildIXBRLDocument(f ixbrlFixture) string {
	if f.malformedXML {
		return `<?xml version="1.0" encoding="UTF-8"?><html xmlns="http://www.w3.org/1999/xhtml"><body><xbrli:context id="Base"><unclosed></body>`
	}
	startDate := fmt.Sprintf("%04d-01-01", f.year)
	endDate := quarterEndDate(f.year, f.quarter)
	priorStart := fmt.Sprintf("%04d-01-01", f.year-1)
	priorEnd := quarterEndDate(f.year-1, f.quarter)

	var facts []string
	if !f.omitOCF {
		facts = append(facts, ixFactTag(cashflowOCFConcept, "Base", f.ocfText, f.ocfScale, f.ocfSign, f.ocfFormat, f.ocfNil))
	}
	if !f.omitProfitLoss {
		facts = append(facts, ixFactTag(cashflowProfitLossConcept, "Base", f.plText, f.plScale, f.plSign, "", f.plNil))
	}
	decoy := ixFactTag(cashflowProfitLossConcept, "Dimensional", "0", "3", "", "", false)

	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:ifrs-full="http://xbrl.ifrs.org/taxonomy/2017-03-09/ifrs-full" xmlns:iso4217="http://www.xbrl.org/2003/iso4217" xmlns:xbrli="http://www.xbrl.org/2003/instance" xmlns:xbrldi="http://xbrl.org/2006/xbrldi" xmlns:ix="http://www.xbrl.org/2013/inlineXBRL" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
<body>
<xbrli:context id="Base"><xbrli:entity><xbrli:identifier scheme="http://www.twse.com.tw">%s</xbrli:identifier></xbrli:entity><xbrli:period><xbrli:startDate>%s</xbrli:startDate><xbrli:endDate>%s</xbrli:endDate></xbrli:period></xbrli:context>
<xbrli:context id="Prior"><xbrli:entity><xbrli:identifier scheme="http://www.twse.com.tw">%s</xbrli:identifier></xbrli:entity><xbrli:period><xbrli:startDate>%s</xbrli:startDate><xbrli:endDate>%s</xbrli:endDate></xbrli:period></xbrli:context>
<xbrli:context id="Dimensional"><xbrli:entity><xbrli:identifier scheme="http://www.twse.com.tw">%s</xbrli:identifier></xbrli:entity><xbrli:period><xbrli:startDate>%s</xbrli:startDate><xbrli:endDate>%s</xbrli:endDate></xbrli:period><xbrli:scenario><xbrldi:explicitMember dimension="ifrs-full:ComponentsOfEquityAxis">ifrs-full:RetainedEarningsMember</xbrldi:explicitMember></xbrli:scenario></xbrli:context>
%s
%s
</body>
</html>`, f.code, startDate, endDate, f.code, priorStart, priorEnd, f.code, startDate, endDate, strings.Join(facts, "\n"), decoy)
}

func writeZipFile(t *testing.T, path string, entries map[string]string) {
	t.Helper()
	if err := os.WriteFile(path, buildZipBytes(t, entries), 0o644); err != nil {
		t.Fatalf("write zip fixture %s: %v", path, err)
	}
}

func buildZipBytes(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range entries {
		entry, err := w.Create(name)
		if err != nil {
			t.Fatalf("create entry %s: %v", name, err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatalf("write entry %s: %v", name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	return buf.Bytes()
}

// ==================================================
// Discovery
// ==================================================

func TestFetchCashflowCandidates_NewestToOldestDeduped(t *testing.T) {
	html := `<a onclick="...fileName=tifrs-2026Q2.zip...">x</a>
<a onclick="...fileName=tifrs-2026Q1.zip...">x</a>
<a onclick="...fileName=tifrs-2025Q4.zip...">x</a>
<a onclick="...fileName=tifrs-2025Q4.zip...">dup</a>
<a onclick="...fileName=tifrs-2025Q3.zip...">x</a>
<a onclick="...fileName=tifrs-2025Q2.zip...">x</a>
<a onclick="...fileName=tifrs-2025Q1.zip...">x</a>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, html)
	}))
	defer server.Close()
	c := NewClient(Config{CashflowBaseURL: server.URL, HTTPClient: server.Client()})
	candidates, err := c.fetchCashflowCandidates(context.Background())
	if err != nil {
		t.Fatalf("fetchCashflowCandidates: %v", err)
	}
	want := []cashflowCandidate{{2026, 2}, {2026, 1}, {2025, 4}, {2025, 3}, {2025, 2}, {2025, 1}}
	if len(candidates) != len(want) {
		t.Fatalf("got %d candidates, want %d: %v", len(candidates), len(want), candidates)
	}
	for i, got := range candidates {
		if got != want[i] {
			t.Errorf("candidate[%d] = %+v, want %+v", i, got, want[i])
		}
	}
}

// ==================================================
// Archive/ZIP validation
// ==================================================

func TestFetchCashflowCandidateZIP_RejectsFakeHTML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "<html>系統忙碌，請重新查詢一遍</html>")
	}))
	defer server.Close()
	c := NewClient(Config{CashflowBaseURL: server.URL, HTTPClient: server.Client()})
	_, err := c.fetchCashflowCandidateZIP(context.Background(), cashflowCandidate{2026, 3})
	if err != errCashflowCandidateInvalid {
		t.Fatalf("expected errCashflowCandidateInvalid, got %v", err)
	}
}

func TestFetchCashflowCandidateZIP_AcceptsVariousZipMimeTypes(t *testing.T) {
	zipBytes := buildZipBytes(t, map[string]string{"tifrs-fr1-m1-ci-cr-1101-2026Q2.html": "ok"})
	for _, mime := range []string{"application/x-zip-compressed", "application/zip", "application/octet-stream", ""} {
		mime := mime
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if mime != "" {
				w.Header().Set("Content-Type", mime)
			}
			w.Write(zipBytes)
		}))
		c := NewClient(Config{CashflowBaseURL: server.URL, HTTPClient: server.Client()})
		path, err := c.fetchCashflowCandidateZIP(context.Background(), cashflowCandidate{2026, 2})
		server.Close()
		if err != nil {
			t.Fatalf("mime %q: unexpected error %v", mime, err)
		}
		os.Remove(path)
	}
}

// TestFetchCashflowCandidateZIP_NotBoundByGeneralClientTimeout is a regression test for a live-verified
// production bug: fetchCashflowCandidateZIP used to reuse c.httpClient directly, whose Timeout (15s by
// default — sized for the small JSON API calls used everywhere else in this package) is often too short
// for a real ~110-130MB quarterly archive, causing the newest genuinely complete candidate to be
// silently rejected as a timeout while an older, faster-to-download one was accepted instead. This test
// uses a deliberately short "general" client Timeout (1s) with a server response slower than that (but
// within the fix's own longer archive-specific timeout) — before the fix this fails with
// errCashflowCandidateInvalid; after the fix it must succeed.
func TestFetchCashflowCandidateZIP_NotBoundByGeneralClientTimeout(t *testing.T) {
	zipBytes := buildZipBytes(t, map[string]string{"tifrs-fr1-m1-ci-cr-1101-2026Q2.html": "ok"})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1200 * time.Millisecond)
		w.Header().Set("Content-Type", "application/zip")
		w.Write(zipBytes)
	}))
	defer server.Close()
	c := NewClient(Config{CashflowBaseURL: server.URL, HTTPClient: &http.Client{Timeout: 1 * time.Second}})
	path, err := c.fetchCashflowCandidateZIP(context.Background(), cashflowCandidate{2026, 2})
	if err != nil {
		t.Fatalf("archive download must not be bound by the general (short, JSON-sized) client Timeout: %v", err)
	}
	os.Remove(path)
}

func TestFetchCashflowCandidateZIP_RejectsCorruptZip(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		fmt.Fprint(w, "this is not a real zip file")
	}))
	defer server.Close()
	c := NewClient(Config{CashflowBaseURL: server.URL, HTTPClient: server.Client()})
	_, err := c.fetchCashflowCandidateZIP(context.Background(), cashflowCandidate{2026, 2})
	if err != errCashflowCandidateInvalid {
		t.Fatalf("expected errCashflowCandidateInvalid, got %v", err)
	}
}

// ==================================================
// Numeric parsing
// ==================================================

func TestParseIXBRLThousandTWD(t *testing.T) {
	cases := []struct {
		name string
		fact ixFact
		want *int64
	}{
		{"scale3 passthrough", ixFact{text: "1,234", scale: "3", format: "ixt:numdotdecimal"}, int64Ptr(1234)},
		{"negative via sign", ixFact{text: "1,234", scale: "3", sign: "-", format: "ixt:numdotdecimal"}, int64Ptr(-1234)},
		{"real zero is not nil", ixFact{text: "0", scale: "3", format: "ixt:numdotdecimal"}, int64Ptr(0)},
		{"xsi:nil is nil", ixFact{text: "1,234", scale: "3", nilFlag: "true", format: "ixt:numdotdecimal"}, nil},
		{"empty text is nil", ixFact{text: "", scale: "3", format: "ixt:numdotdecimal"}, nil},
		{"unsupported format is nil", ixFact{text: "1,234", scale: "3", format: "ixt:other"}, nil},
		{"malformed number is nil", ixFact{text: "abc", scale: "3", format: "ixt:numdotdecimal"}, nil},
		{"malformed scale is nil", ixFact{text: "1,234", scale: "x", format: "ixt:numdotdecimal"}, nil},
		{"scale 0 divides down", ixFact{text: "1234000", scale: "0", format: "ixt:numdotdecimal"}, int64Ptr(1234)},
		{"scale 6 multiplies up", ixFact{text: "1", scale: "6", format: "ixt:numdotdecimal"}, int64Ptr(1000)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseIXBRLThousandTWD(tc.fact)
			assertInt64PtrEqual(t, got, tc.want)
		})
	}
}

func TestScaleToThousandTWD_OverflowRejected(t *testing.T) {
	_, safe := scaleToThousandTWD(9_000_000_000_000_000_000, 5)
	if safe {
		t.Fatal("expected overflow to be rejected, got safe=true")
	}
	v, safe := scaleToThousandTWD(5, 0)
	if !safe || v != 5 {
		t.Fatalf("exponent 0 should pass through unchanged, got v=%d safe=%v", v, safe)
	}
}

// ==================================================
// cashflowRatio (negative/zero denominator policy — deliberately NOT the same as ratio())
// ==================================================

func TestCashflowRatio(t *testing.T) {
	cases := []struct {
		name             string
		numerator, denom *int64
		want             *float64
	}{
		{"positive/positive", int64Ptr(100), int64Ptr(50), float64Ptr(200)},
		{"positive/negative retains sign", int64Ptr(100), int64Ptr(-50), float64Ptr(-200)},
		{"negative numerator retains sign", int64Ptr(-100), int64Ptr(50), float64Ptr(-200)},
		{"zero denominator is nil", int64Ptr(100), int64Ptr(0), nil},
		{"nil numerator is nil", nil, int64Ptr(50), nil},
		{"nil denominator is nil", int64Ptr(100), nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := cashflowRatio(tc.numerator, tc.denom)
			assertFloat64PtrEqual(t, got, tc.want)
		})
	}
}

// ==================================================
// iXBRL document context selection / parsing
// ==================================================

func TestParseCashflowIssuerDocument_ContextSelection(t *testing.T) {
	doc := buildIXBRLDocument(ixbrlFixture{
		code: "2330", year: 2026, quarter: 2,
		ocfText: "1,234,567", ocfScale: "3",
		plText: "500,000", plScale: "3",
	})
	ocf, pl, err := parseCashflowIssuerDocument(strings.NewReader(doc), "2026-01-01", "2026-06-30")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	assertInt64PtrEqual(t, ocf, int64Ptr(1234567))
	// The decoy dimensional-context ProfitLoss fact (value 0) must NOT win.
	assertInt64PtrEqual(t, pl, int64Ptr(500000))
}

func TestParseCashflowIssuerDocument_NegativeSign(t *testing.T) {
	doc := buildIXBRLDocument(ixbrlFixture{
		code: "6488", year: 2026, quarter: 2,
		ocfText: "150,005", ocfScale: "3", ocfSign: "-",
		plText: "500,000", plScale: "3",
	})
	ocf, pl, err := parseCashflowIssuerDocument(strings.NewReader(doc), "2026-01-01", "2026-06-30")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	assertInt64PtrEqual(t, ocf, int64Ptr(-150005))
	assertInt64PtrEqual(t, pl, int64Ptr(500000))
}

func TestParseCashflowIssuerDocument_MissingConceptIsNilNotFailure(t *testing.T) {
	doc := buildIXBRLDocument(ixbrlFixture{
		code: "1101", year: 2026, quarter: 2,
		omitOCF: true,
		plText:  "500,000", plScale: "3",
	})
	ocf, pl, err := parseCashflowIssuerDocument(strings.NewReader(doc), "2026-01-01", "2026-06-30")
	if err != nil {
		t.Fatalf("missing concept must not be a parse error: %v", err)
	}
	if ocf != nil {
		t.Errorf("expected nil OCF, got %v", *ocf)
	}
	assertInt64PtrEqual(t, pl, int64Ptr(500000))
}

func TestParseCashflowIssuerDocument_MalformedXMLIsFailure(t *testing.T) {
	doc := buildIXBRLDocument(ixbrlFixture{malformedXML: true})
	_, _, err := parseCashflowIssuerDocument(strings.NewReader(doc), "2026-01-01", "2026-06-30")
	if err == nil {
		t.Fatal("expected a structural parse error for malformed XML")
	}
}

// ==================================================
// eligibleCashflowIdentities (ETF exclusion)
// ==================================================

func TestEligibleCashflowIdentities_ExcludesETF(t *testing.T) {
	identities := []foundation.SecurityIdentity{
		cashflowTestIdentity("2330", "TWSE", false),
		cashflowTestIdentity("0050", "TWSE", true),
		cashflowTestIdentity("6488", "TPEX", false),
	}
	allow := eligibleCashflowIdentities(identities)
	if len(allow["TWSE"]) != 1 || len(allow["TPEX"]) != 1 {
		t.Fatalf("expected 1 TWSE + 1 TPEX eligible identity (ETF excluded), got TWSE=%d TPEX=%d", len(allow["TWSE"]), len(allow["TPEX"]))
	}
	if _, ok := allow["TWSE"]["0050"]; ok {
		t.Fatal("ETF 0050 must be excluded from the eligible universe")
	}
}

// ==================================================
// parseCashflowArchive: CR/IR selection, coverage threshold, six categories
// ==================================================

func fixedCandidate() cashflowCandidate { return cashflowCandidate{year: 2026, quarter: 2} }

func testEligible3() (map[string]map[string]foundation.SecurityIdentity, int) {
	identities := []foundation.SecurityIdentity{
		cashflowTestIdentity("2330", "TWSE", false),
		cashflowTestIdentity("6488", "TPEX", false),
		cashflowTestIdentity("9999", "TWSE", false),
	}
	allow := eligibleCashflowIdentities(identities)
	return allow, len(allow["TWSE"]) + len(allow["TPEX"])
}

func TestParseCashflowArchive_CRPreferredOverIR(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/archive.zip"
	crDoc := buildIXBRLDocument(ixbrlFixture{code: "2330", year: 2026, quarter: 2, ocfText: "111,000", ocfScale: "3", plText: "50,000", plScale: "3"})
	irDoc := buildIXBRLDocument(ixbrlFixture{code: "2330", year: 2026, quarter: 2, ocfText: "999,999", ocfScale: "3", plText: "1", plScale: "3"})
	otherDoc := buildIXBRLDocument(ixbrlFixture{code: "6488", year: 2026, quarter: 2, ocfText: "222,000", ocfScale: "3", plText: "60,000", plScale: "3"})
	thirdDoc := buildIXBRLDocument(ixbrlFixture{code: "9999", year: 2026, quarter: 2, ocfText: "333,000", ocfScale: "3", plText: "70,000", plScale: "3"})
	writeZipFile(t, path, map[string]string{
		"tifrs-fr1-m1-ci-cr-2330-2026Q2.html": crDoc,
		"tifrs-fr1-m1-ci-ir-2330-2026Q2.html": irDoc,
		"tifrs-fr1-m1-ci-cr-6488-2026Q2.html": otherDoc,
		"tifrs-fr1-m1-ci-cr-9999-2026Q2.html": thirdDoc,
	})
	eligible, total := testEligible3()
	snapshot, ok := parseCashflowArchive(path, fixedCandidate(), eligible, total)
	if !ok {
		t.Fatal("expected archive to pass completeness threshold")
	}
	row := snapshot.Rows["2330.TWSE"]
	assertInt64PtrEqual(t, row.OperatingCashFlow, int64Ptr(111000)) // CR value, not IR
}

func TestParseCashflowArchive_IRFallbackWhenNoCR(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/archive.zip"
	irDoc := buildIXBRLDocument(ixbrlFixture{code: "2330", year: 2026, quarter: 2, ocfText: "111,000", ocfScale: "3", plText: "50,000", plScale: "3"})
	otherDoc := buildIXBRLDocument(ixbrlFixture{code: "6488", year: 2026, quarter: 2, ocfText: "222,000", ocfScale: "3", plText: "60,000", plScale: "3"})
	thirdDoc := buildIXBRLDocument(ixbrlFixture{code: "9999", year: 2026, quarter: 2, ocfText: "333,000", ocfScale: "3", plText: "70,000", plScale: "3"})
	writeZipFile(t, path, map[string]string{
		"tifrs-fr1-m1-ci-ir-2330-2026Q2.html": irDoc,
		"tifrs-fr1-m1-ci-cr-6488-2026Q2.html": otherDoc,
		"tifrs-fr1-m1-ci-cr-9999-2026Q2.html": thirdDoc,
	})
	eligible, total := testEligible3()
	snapshot, ok := parseCashflowArchive(path, fixedCandidate(), eligible, total)
	if !ok {
		t.Fatal("expected archive to pass completeness threshold")
	}
	row, present := snapshot.Rows["2330.TWSE"]
	if !present {
		t.Fatal("expected a row for 2330.TWSE via IR fallback")
	}
	assertInt64PtrEqual(t, row.OperatingCashFlow, int64Ptr(111000))
}

func TestParseCashflowArchive_UnmatchedIssuerExcluded(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/archive.zip"
	doc2330 := buildIXBRLDocument(ixbrlFixture{code: "2330", year: 2026, quarter: 2, ocfText: "111,000", ocfScale: "3", plText: "50,000", plScale: "3"})
	doc6488 := buildIXBRLDocument(ixbrlFixture{code: "6488", year: 2026, quarter: 2, ocfText: "222,000", ocfScale: "3", plText: "60,000", plScale: "3"})
	doc9999 := buildIXBRLDocument(ixbrlFixture{code: "9999", year: 2026, quarter: 2, ocfText: "333,000", ocfScale: "3", plText: "70,000", plScale: "3"})
	docUnknown := buildIXBRLDocument(ixbrlFixture{code: "0000", year: 2026, quarter: 2, ocfText: "1", ocfScale: "3", plText: "1", plScale: "3"})
	writeZipFile(t, path, map[string]string{
		"tifrs-fr1-m1-ci-cr-2330-2026Q2.html": doc2330,
		"tifrs-fr1-m1-ci-cr-6488-2026Q2.html": doc6488,
		"tifrs-fr1-m1-ci-cr-9999-2026Q2.html": doc9999,
		"tifrs-fr1-m1-ci-cr-0000-2026Q2.html": docUnknown,
	})
	eligible, total := testEligible3()
	snapshot, ok := parseCashflowArchive(path, fixedCandidate(), eligible, total)
	if !ok {
		t.Fatal("expected archive to pass completeness threshold")
	}
	if _, present := snapshot.Rows["0000.TWSE"]; present {
		t.Fatal("unmatched issuer code must never produce a row")
	}
	if len(snapshot.Rows) != 3 {
		t.Fatalf("expected exactly 3 rows (unmatched issuer excluded), got %d", len(snapshot.Rows))
	}
}

func TestParseCashflowArchive_CoverageThresholdRejectsStub(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/archive.zip"
	doc2330 := buildIXBRLDocument(ixbrlFixture{code: "2330", year: 2026, quarter: 2, ocfText: "111,000", ocfScale: "3", plText: "50,000", plScale: "3"})
	writeZipFile(t, path, map[string]string{
		"tifrs-fr1-m1-ci-cr-2330-2026Q2.html": doc2330, // only 1 of 3 eligible issuers present.
	})
	eligible, total := testEligible3()
	_, ok := parseCashflowArchive(path, fixedCandidate(), eligible, total)
	if ok {
		t.Fatal("a stub archive (1/3 eligible issuers) must fail the completeness threshold")
	}
}

func TestParseCashflowArchive_AllSixCategories(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/archive.zip"
	entries := map[string]string{}
	codes := map[string]string{"ci": "2330", "basi": "2891", "bd": "6015", "fh": "2882", "ins": "2823", "mim": "1409"}
	identities := []foundation.SecurityIdentity{}
	for category, code := range codes {
		entries[fmt.Sprintf("tifrs-fr1-m1-%s-cr-%s-2026Q2.html", category, code)] = buildIXBRLDocument(ixbrlFixture{
			code: code, year: 2026, quarter: 2, ocfText: "10,000", ocfScale: "3", plText: "5,000", plScale: "3",
		})
		identities = append(identities, cashflowTestIdentity(code, "TWSE", false))
	}
	writeZipFile(t, path, entries)
	allow := eligibleCashflowIdentities(identities)
	total := len(allow["TWSE"]) + len(allow["TPEX"])
	snapshot, ok := parseCashflowArchive(path, fixedCandidate(), allow, total)
	if !ok {
		t.Fatal("expected all-six-category archive to pass completeness threshold")
	}
	if len(snapshot.Rows) != 6 {
		t.Fatalf("expected 6 rows (one per category), got %d", len(snapshot.Rows))
	}
	for category, code := range codes {
		row, present := snapshot.Rows[code+".TWSE"]
		if !present {
			t.Fatalf("category %s: missing row", category)
		}
		if row.Category != category {
			t.Errorf("category %s: row.Category = %q", category, row.Category)
		}
		assertInt64PtrEqual(t, row.OperatingCashFlow, int64Ptr(10000))
	}
}

func TestParseCashflowArchive_DocumentFailureYieldsPartial(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/archive.zip"
	good1 := buildIXBRLDocument(ixbrlFixture{code: "2330", year: 2026, quarter: 2, ocfText: "111,000", ocfScale: "3", plText: "50,000", plScale: "3"})
	good2 := buildIXBRLDocument(ixbrlFixture{code: "9999", year: 2026, quarter: 2, ocfText: "333,000", ocfScale: "3", plText: "70,000", plScale: "3"})
	bad := buildIXBRLDocument(ixbrlFixture{code: "6488", year: 2026, quarter: 2, malformedXML: true})
	writeZipFile(t, path, map[string]string{
		"tifrs-fr1-m1-ci-cr-2330-2026Q2.html": good1,
		"tifrs-fr1-m1-ci-cr-9999-2026Q2.html": good2,
		"tifrs-fr1-m1-ci-cr-6488-2026Q2.html": bad,
	})
	eligible, total := testEligible3()
	snapshot, ok := parseCashflowArchive(path, fixedCandidate(), eligible, total)
	if !ok {
		t.Fatal("expected archive to pass completeness threshold despite one malformed document")
	}
	if snapshot.Status != "partial" {
		t.Fatalf("expected partial status, got %q", snapshot.Status)
	}
	if _, present := snapshot.Rows["2330.TWSE"]; !present {
		t.Fatal("successful issuers must survive a sibling's document failure")
	}
	if _, present := snapshot.Rows["6488.TPEX"]; present {
		t.Fatal("malformed issuer must not produce a row")
	}
}

// ==================================================
// End-to-end ScreenerCashflow: discovery + download + parse + cache + concurrency
// ==================================================

// cashflowTestServer builds one httptest.Server serving the TWSE/TPEx directory endpoints plus the
// MOPS discovery (t203sb02) and download (FileDownLoad) endpoints, so a single Client can be pointed at
// it via TWSEBaseURL/TPExBaseURL/CashflowBaseURL exactly like the real three official hosts.
func cashflowTestServer(t *testing.T, zipBytes []byte, downloadCalls, discoveryCalls *int64) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/opendata/t187ap03_L", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[{"公司代號":"2330","公司簡稱":"台積電","公司名稱":"台積電","產業別":"半導體業","上市日期":"1994/09/05"},{"公司代號":"9999","公司簡稱":"九九","公司名稱":"九九","產業別":"其他","上市日期":"2000/01/01"}]`)
	})
	mux.HandleFunc("/opendata/t187ap47_L", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `[]`) })
	mux.HandleFunc("/mopsfin_t187ap03_O", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[{"SecuritiesCompanyCode":"6488","CompanyAbbreviation":"環球晶","CompanyName":"環球晶","SecuritiesIndustryCode":"半導體業","DateOfListing":"2019/07/10"}]`)
	})
	mux.HandleFunc("/mops/web/t203sb02", func(w http.ResponseWriter, r *http.Request) {
		if discoveryCalls != nil {
			atomic.AddInt64(discoveryCalls, 1)
		}
		fmt.Fprint(w, `<a onclick="...fileName=tifrs-2026Q2.zip...">x</a>`)
	})
	mux.HandleFunc("/server-java/FileDownLoad", func(w http.ResponseWriter, r *http.Request) {
		if downloadCalls != nil {
			atomic.AddInt64(downloadCalls, 1)
		}
		w.Header().Set("Content-Type", "application/x-zip-compressed")
		w.Write(zipBytes)
	})
	return httptest.NewServer(mux)
}

func newCashflowTestClient(server *httptest.Server, now time.Time) *Client {
	return NewClient(Config{
		TWSEBaseURL: server.URL, TPExBaseURL: server.URL, CashflowBaseURL: server.URL,
		HTTPClient: server.Client(), Now: func() time.Time { return now },
	})
}

func threeIssuerZip(t *testing.T) []byte {
	doc2330 := buildIXBRLDocument(ixbrlFixture{code: "2330", year: 2026, quarter: 2, ocfText: "1,122,637,757", ocfScale: "3", plText: "758,226,085", plScale: "3"})
	doc6488 := buildIXBRLDocument(ixbrlFixture{code: "6488", year: 2026, quarter: 2, ocfText: "150,005", ocfScale: "3", ocfSign: "-", plText: "60,000", plScale: "3"})
	doc9999 := buildIXBRLDocument(ixbrlFixture{code: "9999", year: 2026, quarter: 2, ocfText: "333,000", ocfScale: "3", plText: "0", plScale: "3"})
	return buildZipBytes(t, map[string]string{
		"tifrs-fr1-m1-ci-cr-2330-2026Q2.html": doc2330,
		"tifrs-fr1-m1-ci-cr-6488-2026Q2.html": doc6488,
		"tifrs-fr1-m1-ci-cr-9999-2026Q2.html": doc9999,
	})
}

func TestScreenerCashflow_EndToEnd(t *testing.T) {
	zipBytes := threeIssuerZip(t)
	var downloads, discoveries int64
	server := cashflowTestServer(t, zipBytes, &downloads, &discoveries)
	defer server.Close()
	c := newCashflowTestClient(server, time.Now())

	rows, freshness, err := c.ScreenerCashflow(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("ScreenerCashflow: %v", err)
	}
	if freshness.Status != "available" {
		t.Fatalf("expected available, got %q", freshness.Status)
	}
	if freshness.AsOf == nil || *freshness.AsOf != "2026-Q2" {
		t.Fatalf("expected AsOf 2026-Q2, got %v", freshness.AsOf)
	}
	byCanonical := map[string]foundation.FinancialStatementPeriod{}
	for _, row := range rows {
		byCanonical[row.Canonical] = row
	}
	tsmc := byCanonical["2330.TWSE"]
	assertInt64PtrEqual(t, tsmc.OperatingCashFlow, int64Ptr(1122637757))
	assertFloat64PtrEqual(t, tsmc.CashFlowToNetIncome, float64Ptr(float64(1122637757)/float64(758226085)*100))

	gw := byCanonical["6488.TPEX"]
	assertInt64PtrEqual(t, gw.OperatingCashFlow, int64Ptr(-150005)) // negative OCF preserved

	zeroPL := byCanonical["9999.TWSE"]
	if zeroPL.CashFlowToNetIncome != nil {
		t.Fatalf("zero ProfitLoss must yield a nil ratio, got %v", *zeroPL.CashFlowToNetIncome)
	}

	if downloads != 1 || discoveries != 1 {
		t.Fatalf("expected exactly 1 discovery + 1 download on cold load, got discoveries=%d downloads=%d", discoveries, downloads)
	}

	// Warm cache: a second call within the TTL must trigger zero additional HTTP requests.
	if _, _, err := c.ScreenerCashflow(context.Background(), time.Now()); err != nil {
		t.Fatalf("warm ScreenerCashflow: %v", err)
	}
	if downloads != 1 || discoveries != 1 {
		t.Fatalf("expected warm cache hit (0 additional HTTP), got discoveries=%d downloads=%d", discoveries, downloads)
	}
}

func TestScreenerCashflow_UnavailableIsNegativelyCachedAndDeduplicated(t *testing.T) {
	var downloads, discoveries int64
	mux := http.NewServeMux()
	mux.HandleFunc("/opendata/t187ap03_L", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[{"公司代號":"2330","公司簡稱":"台積電","公司名稱":"台積電","產業別":"半導體業","上市日期":"1994/09/05"}]`)
	})
	mux.HandleFunc("/opendata/t187ap47_L", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `[]`) })
	mux.HandleFunc("/mopsfin_t187ap03_O", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `[]`) })
	mux.HandleFunc("/mops/web/t203sb02", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&discoveries, 1)
		fmt.Fprint(w, `<a onclick="...fileName=tifrs-2026Q2.zip...">x</a><a onclick="...fileName=tifrs-2026Q1.zip...">x</a><a onclick="...fileName=tifrs-2025Q4.zip...">x</a><a onclick="...fileName=tifrs-2025Q3.zip...">x</a>`)
	})
	mux.HandleFunc("/server-java/FileDownLoad", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&downloads, 1)
		w.Header().Set("Content-Type", "text/html") // every candidate is a fake-200 HTML error page.
		fmt.Fprint(w, "<html>maintenance</html>")
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	now := time.Now()
	c := newCashflowTestClient(server, now)

	// Concurrent cold callers during the outage — only one discovery/download attempt should occur.
	const concurrency = 8
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, freshness, err := c.ScreenerCashflow(context.Background(), now)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if freshness.Status != "unavailable" {
				t.Errorf("expected unavailable, got %q", freshness.Status)
			}
		}()
	}
	wg.Wait()

	if discoveries != 1 {
		t.Fatalf("expected exactly 1 discovery attempt across %d concurrent callers, got %d", concurrency, discoveries)
	}
	if downloads != cashflowCandidateWindow {
		t.Fatalf("expected exactly %d candidate probes (bounded window), got %d", cashflowCandidateWindow, downloads)
	}

	// Still within the 15-minute negative-cache TTL: zero additional HTTP.
	if _, _, err := c.ScreenerCashflow(context.Background(), now.Add(5*time.Minute)); err != nil {
		t.Fatalf("cached-unavailable call: %v", err)
	}
	if discoveries != 1 || downloads != int64(cashflowCandidateWindow) {
		t.Fatalf("expected no additional HTTP within negative-cache TTL, got discoveries=%d downloads=%d", discoveries, downloads)
	}

	// After the negative-cache TTL expires, one more bounded retry occurs.
	if _, _, err := c.ScreenerCashflow(context.Background(), now.Add(16*time.Minute)); err != nil {
		t.Fatalf("post-TTL retry: %v", err)
	}
	if discoveries != 2 {
		t.Fatalf("expected exactly 1 additional discovery after TTL expiry, got %d total", discoveries)
	}
}
