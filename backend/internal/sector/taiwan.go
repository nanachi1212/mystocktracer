package sector

import (
	"sort"
	"strings"

	"easy-stock/backend/internal/foundation"
)

const TaiwanIndustryRadarVersion = "taiwan_industry_radar_v1"

type TaiwanIndustryDataQuality struct {
	SampleSize        int      `json:"sample_size"`
	DirectionCoverage *float64 `json:"direction_coverage"`
	AmountCoverage    *float64 `json:"amount_coverage"`
}

type TaiwanIndustry struct {
	IndustryID           string                    `json:"industry_id"`
	IndustryCode         string                    `json:"industry_code"`
	IndustryName         string                    `json:"industry_name"`
	Exchange             string                    `json:"exchange"`
	TaxonomySource       string                    `json:"taxonomy_source"`
	TaxonomySourceURL    string                    `json:"taxonomy_source_url"`
	ConstituentCount     int                       `json:"constituent_count"`
	TradedCount          int                       `json:"traded_count"`
	Advancers            int                       `json:"advancers"`
	Decliners            int                       `json:"decliners"`
	Unchanged            int                       `json:"unchanged"`
	NoTrade              int                       `json:"no_trade"`
	Unknown              int                       `json:"unknown"`
	AdvanceDeclineDiff   int                       `json:"advance_decline_diff"`
	AdvanceRatio         *float64                  `json:"advance_ratio"`
	TotalAmountTWD       float64                   `json:"total_amount_twd"`
	AdvancingAmountTWD   float64                   `json:"advancing_amount_twd"`
	DecliningAmountTWD   float64                   `json:"declining_amount_twd"`
	UnchangedAmountTWD   float64                   `json:"unchanged_amount_twd"`
	MissingAmountCount   int                       `json:"missing_amount_count"`
	AdvancingAmountRatio *float64                  `json:"advancing_amount_ratio"`
	RelativeBreadth      *float64                  `json:"relative_breadth"`
	RelativeCapital      *float64                  `json:"relative_capital"`
	BenchmarkScope       string                    `json:"benchmark_scope"`
	DataQuality          TaiwanIndustryDataQuality `json:"data_quality"`
}

type TaiwanIndustryScope struct {
	Scope                   string           `json:"scope"`
	ModelVersion            string           `json:"model_version"`
	AsOf                    *string          `json:"as_of"`
	TargetLatestTradingDate string           `json:"target_latest_trading_date"`
	Status                  string           `json:"status"`
	Freshness               string           `json:"freshness"`
	SnapshotStatus          string           `json:"snapshot_status"`
	TaxonomyStatus          string           `json:"taxonomy_status"`
	ClassificationBasis     string           `json:"classification_basis"`
	EligibleUniverseCount   int              `json:"eligible_universe_count"`
	ClassifiedCount         int              `json:"classified_count"`
	UnclassifiedCount       int              `json:"unclassified_count"`
	IndustryCoverage        *float64         `json:"industry_coverage"`
	RankingPolicy           string           `json:"ranking_policy"`
	IncludedExchanges       []string         `json:"included_exchanges"`
	MissingExchanges        []string         `json:"missing_exchanges"`
	Industries              []TaiwanIndustry `json:"industries"`
}

type TaiwanIndustryRadar struct {
	TWSE     TaiwanIndustryScope `json:"twse"`
	TPEX     TaiwanIndustryScope `json:"tpex"`
	Combined TaiwanIndustryScope `json:"combined"`
}

type taiwanIndustryDefinition struct{ name, source, url string }

const (
	twseTaxonomyURL = "https://twse-regulation.twse.com.tw/tw/law/DOC01_print.aspx?FLCODE=FL007104&FLNO=2"
	tpexTaxonomyURL = "https://www.tpex.org.tw/storage/eb_data/11209/1120500741-1.pdf"
)

var commonTaiwanIndustries = map[string]string{
	"02": "食品工業", "03": "塑膠工業", "04": "紡織纖維", "05": "電機機械", "06": "電器電纜",
	"08": "玻璃陶瓷", "10": "鋼鐵工業", "11": "橡膠工業", "14": "建材營造", "15": "航運業",
	"16": "觀光餐旅", "18": "貿易百貨", "20": "其他", "21": "化學工業", "22": "生技醫療業",
	"23": "油電燃氣業", "24": "半導體業", "25": "電腦及週邊設備業", "26": "光電業", "27": "通信網路業",
	"28": "電子零組件業", "29": "電子通路業", "30": "資訊服務業", "31": "其他電子業",
	"35": "綠能環保", "36": "數位雲端", "37": "運動休閒", "38": "居家生活",
}

func officialTaiwanIndustry(exchange, code string) (taiwanIndustryDefinition, bool) {
	code = strings.TrimSpace(code)
	name, ok := commonTaiwanIndustries[code]
	if exchange == "TWSE" {
		specific := map[string]string{"01": "水泥工業", "09": "造紙工業", "12": "汽車工業", "17": "金融保險", "19": "綜合"}
		if value, found := specific[code]; found {
			name, ok = value, true
		}
		return taiwanIndustryDefinition{name, "TWSE official industry classification", twseTaxonomyURL}, ok
	}
	if exchange == "TPEX" {
		specific := map[string]string{"17": "金融業", "32": "文化創意業", "33": "農業科技", "34": "電子商務", "80": "管理股票"}
		if value, found := specific[code]; found {
			name, ok = value, true
		}
		return taiwanIndustryDefinition{name, "TPEx official industry classification", tpexTaxonomyURL}, ok
	}
	return taiwanIndustryDefinition{}, false
}

func CalculateTaiwanIndustryRadar(identities []foundation.SecurityIdentity, rows []foundation.TaiwanDailySnapshot, breadth foundation.TaiwanMarketBreadth) TaiwanIndustryRadar {
	twse := calculateTaiwanIndustryScope("TWSE", identities, rows, breadth.TWSE)
	tpex := calculateTaiwanIndustryScope("TPEX", identities, rows, breadth.TPEX)
	combinedIdentities := make([]foundation.SecurityIdentity, 0, len(identities))
	for _, identity := range identities {
		if containsString(breadth.Combined.IncludedExchanges, identity.Exchange) {
			combinedIdentities = append(combinedIdentities, identity)
		}
	}
	combined := calculateTaiwanIndustryScope("COMBINED", combinedIdentities, rows, breadth.Combined)
	return TaiwanIndustryRadar{TWSE: twse, TPEX: tpex, Combined: combined}
}

func calculateTaiwanIndustryScope(scope string, identities []foundation.SecurityIdentity, rows []foundation.TaiwanDailySnapshot, market foundation.TaiwanBreadthScope) TaiwanIndustryScope {
	result := TaiwanIndustryScope{Scope: scope, ModelVersion: TaiwanIndustryRadarVersion, AsOf: market.AsOf,
		TargetLatestTradingDate: market.TargetLatestTradingDate, Status: market.Status, Freshness: market.Freshness,
		SnapshotStatus: market.Status, ClassificationBasis: "current_reference", RankingPolicy: "relative_breadth_desc_then_industry_id",
		IncludedExchanges: append([]string(nil), market.IncludedExchanges...), MissingExchanges: append([]string(nil), market.MissingExchanges...)}
	rowByID := make(map[string]foundation.TaiwanDailySnapshot, len(rows))
	for _, row := range rows {
		rowByID[row.Canonical] = row
	}
	groups := map[string]*TaiwanIndustry{}
	for _, identity := range identities {
		if identity.Type != foundation.SecurityTypeStock || (scope != "COMBINED" && identity.Exchange != scope) {
			continue
		}
		result.EligibleUniverseCount++
		definition, ok := officialTaiwanIndustry(identity.Exchange, identity.Industry)
		if !ok {
			result.UnclassifiedCount++
			continue
		}
		result.ClassifiedCount++
		id := identity.Exchange + ":" + strings.TrimSpace(identity.Industry)
		item := groups[id]
		if item == nil {
			item = &TaiwanIndustry{IndustryID: id, IndustryCode: strings.TrimSpace(identity.Industry), IndustryName: definition.name,
				Exchange: identity.Exchange, TaxonomySource: definition.source, TaxonomySourceURL: definition.url}
			groups[id] = item
		}
		item.ConstituentCount++
		row, found := rowByID[identity.Canonical]
		addTaiwanIndustryObservation(item, row, found && market.Status != "unavailable")
	}
	if result.EligibleUniverseCount > 0 {
		ratio := float64(result.ClassifiedCount) / float64(result.EligibleUniverseCount)
		result.IndustryCoverage = &ratio
	}
	switch {
	case result.ClassifiedCount == 0:
		result.TaxonomyStatus = "unavailable"
	case result.UnclassifiedCount > 0:
		result.TaxonomyStatus = "partial"
	default:
		result.TaxonomyStatus = "current"
	}
	if result.Status != "unavailable" && result.TaxonomyStatus == "unavailable" {
		result.Status = "unavailable"
	}
	if result.Status != "unavailable" && result.TaxonomyStatus == "partial" {
		result.Status = "partial"
	}
	for _, item := range groups {
		finalizeTaiwanIndustry(item, market)
		result.Industries = append(result.Industries, *item)
	}
	sort.Slice(result.Industries, func(i, j int) bool {
		a, b := result.Industries[i], result.Industries[j]
		if a.RelativeBreadth == nil {
			return b.RelativeBreadth == nil && a.IndustryID < b.IndustryID
		}
		if b.RelativeBreadth == nil {
			return true
		}
		if *a.RelativeBreadth == *b.RelativeBreadth {
			return a.IndustryID < b.IndustryID
		}
		return *a.RelativeBreadth > *b.RelativeBreadth
	})
	return result
}

func addTaiwanIndustryObservation(item *TaiwanIndustry, row foundation.TaiwanDailySnapshot, available bool) {
	if !available {
		item.Unknown++
		item.MissingAmountCount++
		return
	}
	if row.Amount == nil {
		item.MissingAmountCount++
	} else {
		item.TotalAmountTWD += *row.Amount
	}
	bucket := "unknown"
	if row.NoTrade || (row.Volume != nil && *row.Volume == 0) {
		bucket = "no_trade"
	} else if row.Change != nil {
		item.TradedCount++
		if *row.Change > 0 {
			bucket = "advance"
		} else if *row.Change < 0 {
			bucket = "decline"
		} else {
			bucket = "unchanged"
		}
	} else if row.Volume != nil && *row.Volume > 0 {
		item.TradedCount++
	}
	switch bucket {
	case "advance":
		item.Advancers++
		if row.Amount != nil {
			item.AdvancingAmountTWD += *row.Amount
		}
	case "decline":
		item.Decliners++
		if row.Amount != nil {
			item.DecliningAmountTWD += *row.Amount
		}
	case "unchanged":
		item.Unchanged++
		if row.Amount != nil {
			item.UnchangedAmountTWD += *row.Amount
		}
	case "no_trade":
		item.NoTrade++
	default:
		item.Unknown++
	}
}

func finalizeTaiwanIndustry(item *TaiwanIndustry, market foundation.TaiwanBreadthScope) {
	item.BenchmarkScope = market.Scope
	item.AdvanceDeclineDiff = item.Advancers - item.Decliners
	if denominator := item.Advancers + item.Decliners; denominator > 0 {
		ratio := float64(item.Advancers) / float64(denominator)
		item.AdvanceRatio = &ratio
	}
	if denominator := item.AdvancingAmountTWD + item.DecliningAmountTWD; denominator > 0 {
		ratio := item.AdvancingAmountTWD / denominator
		item.AdvancingAmountRatio = &ratio
	}
	if item.AdvanceRatio != nil && market.AdvanceRatio != nil {
		value := *item.AdvanceRatio - *market.AdvanceRatio
		item.RelativeBreadth = &value
	}
	if item.AdvancingAmountRatio != nil && market.AdvancingAmountRatio != nil {
		value := *item.AdvancingAmountRatio - *market.AdvancingAmountRatio
		item.RelativeCapital = &value
	}
	item.DataQuality.SampleSize = item.ConstituentCount
	if item.ConstituentCount > 0 {
		value := float64(item.Advancers+item.Decliners+item.Unchanged+item.NoTrade) / float64(item.ConstituentCount)
		item.DataQuality.DirectionCoverage = &value
		value = float64(item.ConstituentCount-item.MissingAmountCount) / float64(item.ConstituentCount)
		item.DataQuality.AmountCoverage = &value
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
