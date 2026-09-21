package httpapi

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/nanachi1212/mystocktracer/backend/internal/appsettings"
)

var supportedLLMProviders = map[string]struct{}{
	"": {}, "openai": {}, "deepseek": {}, "qwen": {}, "moonshot": {},
	"minimax": {}, "zhipu": {}, "siliconflow": {}, "anthropic": {}, "custom": {},
}

var supportedAPIModes = map[string]struct{}{
	"": {}, "chat_completions": {}, "responses": {}, "codex_responses": {}, "anthropic_messages": {},
}

func validateSettingsUpdate(request settingsUpdateRequest) error {
	if request.LLMProfiles != nil {
		profiles := *request.LLMProfiles
		if len(profiles) == 0 || len(profiles) > 20 {
			return fmt.Errorf("模型設定數量必須介於 1 到 20")
		}
		seen := make(map[string]struct{}, len(profiles))
		for _, profile := range profiles {
			id := strings.TrimSpace(profile.ID)
			if id == "" || len(id) > 100 {
				return fmt.Errorf("模型設定 ID 無效")
			}
			if _, exists := seen[id]; exists {
				return fmt.Errorf("模型設定 ID 不得重複")
			}
			seen[id] = struct{}{}
			if strings.TrimSpace(profile.Name) == "" || len([]rune(profile.Name)) > 80 {
				return fmt.Errorf("模型設定名稱必須介於 1 到 80 個字元")
			}
			if err := validateLLMFields(&profile.Provider, &profile.BaseURL, &profile.Model, &profile.APIMode, nil, profile.APIKey); err != nil {
				return err
			}
		}
		if request.ActiveLLMProfileID != nil {
			if _, exists := seen[strings.TrimSpace(*request.ActiveLLMProfileID)]; !exists {
				return fmt.Errorf("指定的模型設定不存在")
			}
		}
	}
	if err := validateLLMFields(
		request.LLM.Provider, request.LLM.BaseURL, request.LLM.Model, request.LLM.APIMode,
		request.LLM.ResponseTimeoutSeconds, request.LLM.APIKey,
	); err != nil {
		return err
	}
	for _, key := range request.ClearSecrets {
		if key != "llm_api_key" {
			return fmt.Errorf("不支援的 secret 清除項目: %s", key)
		}
	}
	return nil
}

func validateLLMFields(provider, baseURL, model, apiMode *string, timeout *int, apiKey *string) error {
	if provider != nil {
		if _, ok := supportedLLMProviders[strings.TrimSpace(*provider)]; !ok {
			return fmt.Errorf("不支援的模型供應商: %s", strings.TrimSpace(*provider))
		}
	}
	if baseURL != nil {
		value := strings.TrimSpace(*baseURL)
		if len(value) > 512 {
			return fmt.Errorf("模型 base_url 過長")
		}
		if value != "" {
			parsed, err := url.Parse(value)
			if err != nil || parsed.Host == "" || parsed.Scheme != "http" && parsed.Scheme != "https" {
				return fmt.Errorf("模型 base_url 必須是 HTTP 或 HTTPS 網址")
			}
		}
	}
	if model != nil && len(strings.TrimSpace(*model)) > 160 {
		return fmt.Errorf("模型名稱過長")
	}
	if apiMode != nil {
		if _, ok := supportedAPIModes[strings.TrimSpace(*apiMode)]; !ok {
			return fmt.Errorf("不支援的模型 API 模式: %s", strings.TrimSpace(*apiMode))
		}
	}
	if timeout != nil && (*timeout < appsettings.MinLLMResponseTimeoutSeconds || *timeout > appsettings.MaxLLMResponseTimeoutSeconds) {
		return fmt.Errorf("模型回應等待時間必須介於 %d 到 %d 秒", appsettings.MinLLMResponseTimeoutSeconds, appsettings.MaxLLMResponseTimeoutSeconds)
	}
	if apiKey != nil && len(*apiKey) > 16<<10 {
		return fmt.Errorf("secret 長度超過限制")
	}
	return nil
}
