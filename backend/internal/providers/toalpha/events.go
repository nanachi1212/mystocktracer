package toalpha

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/nanachi1212/mystocktracer/backend/internal/foundation"
	"golang.org/x/text/unicode/norm"
)

type materialNewsResponse struct {
	Stock struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Market   string `json:"market"`
		Industry string `json:"industry"`
	} `json:"stock"`
	Count         int    `json:"count"`
	WindowDays    int    `json:"window_days"`
	Keyword       string `json:"keyword"`
	ImportantOnly bool   `json:"important_only"`
	Category      string `json:"category"`
	Rows          []struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Date      string `json:"date"`
		Time      string `json:"time"`
		Seq       int    `json:"seq"`
		Subject   string `json:"subject"`
		Category  string `json:"category"`
		Rule      string `json:"rule"`
		Important *bool  `json:"important"`
		FactDate  string `json:"fact_date"`
		Excerpt   string `json:"excerpt"`
		FullText  bool   `json:"full_text"`
	} `json:"rows"`
	Updated string `json:"updated"`
	Note    string `json:"note"`
	Source  string `json:"source"`
	Link    string `json:"link"`
}

func normalizeMaterialNews(canonical string, raw materialNewsResponse, retrievedAt time.Time) foundation.TaiwanCorporateEventFeed {
	feed := foundation.TaiwanCorporateEventFeed{
		Status: foundation.TaiwanCorporateEventsAvailable, Provider: foundation.TaiwanCorporateEventProviderToAlpha,
		Source: firstNonEmpty(raw.Source, foundation.TaiwanCorporateEventSourceMOPS), SourceURL: safeExternalURL(raw.Link),
		RetrievedAt: timePointer(retrievedAt), Events: make([]foundation.TaiwanCorporateEvent, 0, len(raw.Rows)),
	}
	if updated, err := time.Parse(time.RFC3339Nano, strings.Replace(strings.TrimSpace(raw.Updated), " ", "T", 1)); err == nil {
		feed.AsOf = updated.Format(time.RFC3339)
	} else {
		feed.AsOf = strings.TrimSpace(raw.Updated)
	}
	location, _ := time.LoadLocation("Asia/Taipei")
	for _, row := range raw.Rows {
		partialReasons := make([]string, 0, 3)
		publishedAt, publishedOK := parsePublishedAt(row.Date, row.Time, location)
		if !publishedOK {
			partialReasons = append(partialReasons, "announcement timestamp is missing or invalid")
		}
		title := boundedText(row.Subject, 500)
		if title == "" {
			partialReasons = append(partialReasons, "announcement title is missing")
		}
		if !row.FullText {
			partialReasons = append(partialReasons, "full announcement text is still being backfilled")
		}
		var eventDate *time.Time
		if value, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(row.FactDate), location); err == nil {
			eventDate = timePointer(value)
		}
		event := foundation.TaiwanCorporateEvent{
			ID:     stableEventID(raw.Stock.Market, firstNonEmpty(row.ID, raw.Stock.ID), row.Date, row.Time, row.Seq, canonical, foundation.TaiwanCorporateEventSourceMOPS, title),
			Symbol: canonical, Title: title, Category: boundedText(row.Category, 120), Detail: boundedText(row.Excerpt, 1200),
			PublishedAt: publishedAt, EventDate: eventDate,
			Provider: foundation.TaiwanCorporateEventProviderToAlpha, Source: foundation.TaiwanCorporateEventSourceMOPS,
			SourceURL: safeExternalURL(raw.Link), RetrievedAt: retrievedAt, Status: foundation.TaiwanCorporateEventsAvailable,
			ClassificationSource: "third_party_enrichment", Important: row.Important,
		}
		if len(partialReasons) > 0 {
			event.Status, event.Partial, event.Reason = foundation.TaiwanCorporateEventsPartial, true, strings.Join(partialReasons, "; ")
			feed.Partial = true
		}
		feed.Events = append(feed.Events, event)
	}
	sort.SliceStable(feed.Events, func(i, j int) bool {
		left, right := feed.Events[i].PublishedAt, feed.Events[j].PublishedAt
		if left == nil || right == nil {
			return left != nil
		}
		if left.Equal(*right) {
			return feed.Events[i].ID < feed.Events[j].ID
		}
		return left.After(*right)
	})
	feed.Events = dedupeEvents(feed.Events)
	if len(feed.Events) == 0 && raw.Count == 0 {
		feed.Status = foundation.TaiwanCorporateEventsNoEvents
	} else if feed.Partial || raw.Count > len(raw.Rows) {
		feed.Status = foundation.TaiwanCorporateEventsPartial
		feed.Partial = true
		if raw.Count > len(raw.Rows) {
			feed.Reason = "provider reported more events than the bounded payload contained"
		}
	}
	return feed
}

func dedupeEvents(events []foundation.TaiwanCorporateEvent) []foundation.TaiwanCorporateEvent {
	seen := make(map[string]struct{}, len(events))
	result := make([]foundation.TaiwanCorporateEvent, 0, len(events))
	for _, event := range events {
		if event.ID == "" {
			continue
		}
		if _, ok := seen[event.ID]; ok {
			continue
		}
		seen[event.ID] = struct{}{}
		result = append(result, event)
	}
	return result
}

func parsePublishedAt(date, clock string, location *time.Location) (*time.Time, bool) {
	value, err := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(date)+" "+strings.TrimSpace(clock), location)
	if err != nil {
		return nil, false
	}
	return timePointer(value), true
}

func stableEventID(market, issuer, date, clock string, seq int, canonical, source, title string) string {
	market, issuer, date = normalizeIdentity(market), normalizeIdentity(issuer), normalizeIdentity(date)
	if market != "" && issuer != "" && date != "" && seq > 0 {
		return fmt.Sprintf("mops:%s:%s:%s:%d", market, issuer, date, seq)
	}
	location, _ := time.LoadLocation("Asia/Taipei")
	published := normalizeIdentity(date) + " " + normalizeIdentity(clock)
	if parsed, ok := parsePublishedAt(date, clock, location); ok {
		published = parsed.Format(time.RFC3339)
	}
	input := strings.Join([]string{strings.ToUpper(strings.TrimSpace(canonical)), published, normalizeIdentity(source), normalizeTitle(title)}, "|")
	sum := sha256.Sum256([]byte(input))
	return "toalpha:sha256:" + hex.EncodeToString(sum[:16])
}

func normalizeTitle(value string) string {
	return normalizeIdentity(html.UnescapeString(value))
}

func normalizeIdentity(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(norm.NFC.String(strings.TrimSpace(value)))), " ")
}

func boundedText(value string, count int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) > count {
		runes = runes[:count]
	}
	return string(runes)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func safeExternalURL(value string) string {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
		return ""
	}
	return parsed.String()
}

func timePointer(value time.Time) *time.Time { return &value }
