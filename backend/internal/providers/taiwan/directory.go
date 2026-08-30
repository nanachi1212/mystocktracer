package taiwan

import (
	"context"
	"fmt"
	"strings"
	"time"

	"easy-stock/backend/internal/foundation"
)

type twseCompany struct {
	Code     string `json:"公司代號"`
	Name     string `json:"公司簡稱"`
	FullName string `json:"公司名稱"`
	Industry string `json:"產業別"`
	ListedAt string `json:"上市日期"`
}

type twseFund struct {
	Code            string `json:"基金代號"`
	Name            string `json:"基金簡稱"`
	FullName        string `json:"基金中文名稱"`
	FundType        string `json:"基金類型"`
	ListedAt        string `json:"上市日期"`
	ContainsForeign string `json:"是否包含國外成分股"`
}

type tpexCompany struct {
	Code     string `json:"SecuritiesCompanyCode"`
	Name     string `json:"CompanyAbbreviation"`
	FullName string `json:"CompanyName"`
	Industry string `json:"SecuritiesIndustryCode"`
	ListedAt string `json:"DateOfListing"`
}

func (c *Client) twseStocks(ctx context.Context) ([]foundation.SecurityIdentity, error) {
	url := c.twseBaseURL + "/opendata/t187ap03_L"
	var rows []twseCompany
	if err := c.getJSON(ctx, url, &rows); err != nil {
		return nil, err
	}
	return companyIdentities(rows, "TWSE", "twse", url), nil
}

func (c *Client) twseETFs(ctx context.Context) ([]foundation.SecurityIdentity, error) {
	url := c.twseBaseURL + "/opendata/t187ap47_L"
	var rows []twseFund
	if err := c.getJSON(ctx, url, &rows); err != nil {
		return nil, err
	}
	now := time.Now()
	items := make([]foundation.SecurityIdentity, 0, len(rows))
	for _, row := range rows {
		code, name := strings.TrimSpace(row.Code), strings.TrimSpace(row.Name)
		if code == "" || name == "" || !taiwanSecurityCode(code) {
			continue
		}
		metadata := officialTWSEFundMetadata(row)
		identity := foundation.SecurityIdentity{
			Canonical: code + ".TWSE", Code: code, Name: name, FullName: strings.TrimSpace(row.FullName),
			Market: "TW", Exchange: "TWSE", Type: foundation.SecurityTypeETF,
			Currency: "TWD", Timezone: "Asia/Taipei", ListedAt: strings.TrimSpace(row.ListedAt),
			Industry: strings.TrimSpace(row.FundType), Provider: "twse", SourceURL: url, RetrievedAt: now, TaiwanMetadata: metadata,
		}
		profile := foundation.TaiwanRuleProfile(identity)
		identity.RuleProfile = &profile
		items = append(items, identity)
	}
	return items, nil
}

func officialTWSEFundMetadata(row twseFund) *foundation.TaiwanSecurityMetadata {
	fundType := strings.TrimSpace(row.FundType)
	metadata := &foundation.TaiwanSecurityMetadata{OfficialFundType: fundType}
	switch strings.TrimSpace(row.ContainsForeign) {
	case "是":
		metadata.ComponentScope = "foreign"
	case "否":
		metadata.ComponentScope = "domestic"
	}
	switch fundType {
	case "國內成分證券指數股票型基金", "國外成分證券指數股票型基金":
		metadata.Strategy = "ordinary"
	case "槓桿/反向指數股票型基金":
		metadata.Strategy = "leveraged_or_inverse"
	case "債券指數股票型基金":
		metadata.Strategy = "passive_bond"
	}
	return metadata
}

func (c *Client) tpexStocks(ctx context.Context) ([]foundation.SecurityIdentity, error) {
	url := c.tpexBaseURL + "/mopsfin_t187ap03_O"
	var rows []tpexCompany
	if err := c.getJSON(ctx, url, &rows); err != nil {
		return nil, err
	}
	now := time.Now()
	items := make([]foundation.SecurityIdentity, 0, len(rows))
	for _, row := range rows {
		code, name := strings.TrimSpace(row.Code), strings.TrimSpace(row.Name)
		if code == "" || name == "" || !taiwanSecurityCode(code) {
			continue
		}
		identity := foundation.SecurityIdentity{
			Canonical: code + ".TPEX", Code: code, Name: name, FullName: strings.TrimSpace(row.FullName),
			Market: "TW", Exchange: "TPEX", Type: foundation.SecurityTypeStock,
			Currency: "TWD", Timezone: "Asia/Taipei", ListedAt: strings.TrimSpace(row.ListedAt),
			Industry: strings.TrimSpace(row.Industry), Provider: "tpex", SourceURL: url, RetrievedAt: now,
		}
		profile := foundation.TaiwanRuleProfile(identity)
		identity.RuleProfile = &profile
		items = append(items, identity)
	}
	return items, nil
}

func companyIdentities(rows []twseCompany, exchange, provider, sourceURL string) []foundation.SecurityIdentity {
	now := time.Now()
	items := make([]foundation.SecurityIdentity, 0, len(rows))
	for _, row := range rows {
		code, name := strings.TrimSpace(row.Code), strings.TrimSpace(row.Name)
		if code == "" || name == "" || !taiwanSecurityCode(code) {
			continue
		}
		identity := foundation.SecurityIdentity{
			Canonical: fmt.Sprintf("%s.%s", code, exchange), Code: code, Name: name, FullName: strings.TrimSpace(row.FullName),
			Market: "TW", Exchange: exchange, Type: foundation.SecurityTypeStock,
			Currency: "TWD", Timezone: "Asia/Taipei", ListedAt: strings.TrimSpace(row.ListedAt),
			Industry: strings.TrimSpace(row.Industry), Provider: provider, SourceURL: sourceURL, RetrievedAt: now,
		}
		profile := foundation.TaiwanRuleProfile(identity)
		identity.RuleProfile = &profile
		items = append(items, identity)
	}
	return items
}

func taiwanSecurityCode(code string) bool {
	if len(code) < 4 || len(code) > 6 {
		return false
	}
	for _, character := range code {
		if (character < '0' || character > '9') && (character < 'A' || character > 'Z') {
			return false
		}
	}
	return true
}
