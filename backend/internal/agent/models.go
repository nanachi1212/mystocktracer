package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const MaxModelListResponseBytes = 2 << 20

type ModelOption struct {
	ID          string `json:"id"`
	OwnedBy     string `json:"owned_by,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
}
type ModelDiscoveryRequest struct{ Provider, BaseURL, APIKey string }
type ModelDiscoveryResult struct {
	Models    []ModelOption
	SourceURL string
}

func DiscoverModels(ctx context.Context, input ModelDiscoveryRequest) (ModelDiscoveryResult, error) {
	provider := strings.ToLower(strings.TrimSpace(input.Provider))
	if !SupportedModelProvider(provider) {
		return ModelDiscoveryResult{}, errors.New("unsupported llm provider")
	}
	endpoint, err := ModelsURL(provider, input.BaseURL)
	if err != nil {
		return ModelDiscoveryResult{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return ModelDiscoveryResult{}, errors.New("cannot create model discovery request")
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "mystocktracer/model-discovery")
	if input.APIKey != "" {
		if provider == "anthropic" {
			request.Header.Set("x-api-key", input.APIKey)
			request.Header.Set("anthropic-version", "2023-06-01")
		} else {
			request.Header.Set("Authorization", "Bearer "+input.APIKey)
		}
	}
	client := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(request)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return ModelDiscoveryResult{}, errors.New("model discovery timed out")
		}
		return ModelDiscoveryResult{}, errors.New("cannot reach model discovery endpoint")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return ModelDiscoveryResult{}, fmt.Errorf("model discovery returned HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, MaxModelListResponseBytes+1))
	if err != nil {
		return ModelDiscoveryResult{}, errors.New("cannot read model discovery response")
	}
	if len(body) > MaxModelListResponseBytes {
		return ModelDiscoveryResult{}, errors.New("model discovery response too large")
	}
	models, err := decodeModels(body)
	if err != nil {
		return ModelDiscoveryResult{}, err
	}
	return ModelDiscoveryResult{Models: models, SourceURL: endpoint}, nil
}
func SupportedModelProvider(value string) bool {
	switch value {
	case "openai", "deepseek", "qwen", "moonshot", "minimax", "zhipu", "siliconflow", "anthropic", "custom":
		return true
	}
	return false
}
func ModelsURL(provider, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > 512 || strings.ContainsAny(raw, "\r\n") {
		return "", errors.New("invalid llm base_url")
	}
	u, e := url.Parse(raw)
	if e != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("llm base_url must be an http or https URL without credentials, query, or fragment")
	}
	p := strings.TrimRight(u.Path, "/")
	if provider == "anthropic" && p == "" {
		p = "/v1"
	}
	if !strings.HasSuffix(p, "/models") {
		p += "/models"
	}
	if p == "models" {
		p = "/models"
	}
	u.Path = p
	u.RawPath = ""
	return u.String(), nil
}
func decodeModels(body []byte) ([]ModelOption, error) {
	var payload struct {
		Data []ModelOption `json:"data"`
	}
	if json.Unmarshal(body, &payload) != nil {
		return nil, errors.New("model discovery response is not valid JSON")
	}
	seen := map[string]bool{}
	out := make([]ModelOption, 0, len(payload.Data))
	for _, m := range payload.Data {
		m.ID = strings.TrimSpace(m.ID)
		if m.ID == "" || len(m.ID) > 256 || seen[m.ID] {
			continue
		}
		seen[m.ID] = true
		m.OwnedBy = strings.TrimSpace(m.OwnedBy)
		m.DisplayName = strings.TrimSpace(m.DisplayName)
		out = append(out, m)
	}
	if len(out) == 0 {
		return nil, errors.New("model discovery returned no usable models")
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].ID) < strings.ToLower(out[j].ID) })
	return out, nil
}
