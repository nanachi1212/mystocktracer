package foundation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

const contractV11FixtureSHA256 = "a76aee8a566d838c582d2281b0cdc18b8941ea97e2f4780ea6693775fd97ee28"

type contractV11Revision struct {
	Identity    string  `json:"revision_identity"`
	Supersedes  *string `json:"supersedes_revision"`
	AvailableAt string  `json:"available_at"`
	Value       int     `json:"value"`
}

type contractV11Fixture struct {
	Version              string   `json:"contract_version"`
	Extends              string   `json:"extends"`
	AvailabilityPolicies []string `json:"availability_policies"`
	Boundary             struct {
		AvailableAt       string   `json:"available_at"`
		StrictUnavailable []string `json:"strict_unavailable"`
		FirstAvailable    string   `json:"first_available"`
	} `json:"availability_boundary"`
	Revisions []contractV11Revision `json:"revisions"`
	Queries   []struct {
		QueryAt          string `json:"query_at"`
		ExpectedRevision string `json:"expected_revision"`
		ExpectedValue    int    `json:"expected_value"`
	} `json:"revision_queries"`
	Insufficient struct {
		Policy      string  `json:"availability_policy"`
		AvailableAt *string `json:"available_at"`
		Available   bool    `json:"historically_available"`
	} `json:"insufficient_availability"`
	ExactEvidence map[string]any `json:"exact_evidence"`
	ShareCapital  struct {
		Issued            int  `json:"issued_shares"`
		Float             int  `json:"float_shares"`
		IssuedEqualsFloat bool `json:"issued_equals_float"`
		CapitalImplies    bool `json:"capital_implies_float"`
	} `json:"share_capital"`
}

func loadContractV11(t *testing.T) (contractV11Fixture, []byte) {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "docs", "taiwan_market_contract_v1_1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture contractV11Fixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture, raw
}

func parseContractTime(t *testing.T, raw string) time.Time {
	t.Helper()
	value, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestTaiwanMarketContractV11AvailabilityAndRevisions(t *testing.T) {
	fixture, _ := loadContractV11(t)
	if fixture.Version != "1.1.0" || fixture.Extends != "1.0.0" {
		t.Fatalf("version=%s extends=%s", fixture.Version, fixture.Extends)
	}
	availableAt := parseContractTime(t, fixture.Boundary.AvailableAt)
	for _, raw := range fixture.Boundary.StrictUnavailable {
		if parseContractTime(t, raw).After(availableAt) {
			t.Fatalf("strict unavailable boundary is after available_at: %s", raw)
		}
	}
	if !parseContractTime(t, fixture.Boundary.FirstAvailable).After(availableAt) {
		t.Fatal("first_available must be strictly after available_at")
	}
	for _, query := range fixture.Queries {
		queryAt := parseContractTime(t, query.QueryAt)
		var selected *contractV11Revision
		for i := range fixture.Revisions {
			revision := &fixture.Revisions[i]
			if !queryAt.After(parseContractTime(t, revision.AvailableAt)) {
				continue
			}
			if selected == nil || parseContractTime(t, revision.AvailableAt).After(parseContractTime(t, selected.AvailableAt)) {
				selected = revision
			}
		}
		if selected == nil || selected.Identity != query.ExpectedRevision || selected.Value != query.ExpectedValue {
			t.Fatalf("query %s selected=%+v", query.QueryAt, selected)
		}
	}
}

func TestTaiwanMarketContractV11EvidenceAndShareConcepts(t *testing.T) {
	fixture, raw := loadContractV11(t)
	if fixture.Insufficient.Policy != "insufficient" || fixture.Insufficient.AvailableAt != nil || fixture.Insufficient.Available {
		t.Fatalf("invalid insufficient contract: %+v", fixture.Insufficient)
	}
	if fixture.ExactEvidence["availability_policy"] != "exact_timestamp" || fixture.ExactEvidence["availability_evidence_identifier"] == "" {
		t.Fatalf("invalid exact evidence: %+v", fixture.ExactEvidence)
	}
	if fixture.ShareCapital.Issued == fixture.ShareCapital.Float || fixture.ShareCapital.IssuedEqualsFloat || fixture.ShareCapital.CapitalImplies {
		t.Fatalf("share concepts conflated: %+v", fixture.ShareCapital)
	}
	sum := sha256.Sum256(raw)
	if got := hex.EncodeToString(sum[:]); got != contractV11FixtureSHA256 {
		t.Fatalf("fixture SHA-256=%s", got)
	}
}
