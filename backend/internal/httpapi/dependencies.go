package httpapi

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/nanachi1212/mystocktracer/backend/internal/appsettings"
	"github.com/nanachi1212/mystocktracer/backend/internal/hermes"
	"github.com/nanachi1212/mystocktracer/backend/internal/providers/taiwan"
	"github.com/nanachi1212/mystocktracer/backend/internal/providers/toalpha"
	"github.com/nanachi1212/mystocktracer/backend/internal/taiwanportfolio"
	"github.com/nanachi1212/mystocktracer/backend/internal/taiwanwatchlist"
)

func prepareRuntimeDependencies(config Config) (Config, error) {
	if config.Logger == nil {
		config.Logger = log.Default()
	}
	if config.TaiwanCorporateEvents == nil {
		config.TaiwanCorporateEvents = toalpha.NewClient(toalpha.Config{
			Enabled: config.ToAlphaMOPSEnabled, Endpoint: config.ToAlphaMOPSEndpoint,
			Timeout: config.ToAlphaMOPSTimeout, Retries: 1,
		})
	}
	fillTaiwanProviders(&config)
	startupError := openPersistence(&config)
	if config.HermesGateway != nil && (!config.StrictPersistence || startupError == nil) {
		syncRuntimeSettings(config.SettingsStore, config.HermesGateway)
	}
	return config, startupError
}

func fillTaiwanProviders(config *Config) {
	if hasEveryTaiwanProvider(*config) {
		return
	}
	client := taiwan.NewClient(taiwan.Config{
		CashflowCacheDir: config.TaiwanCashflowCacheDir,
		CorporateEvents:  config.TaiwanCorporateEvents,
	})
	if config.TaiwanDirectory == nil {
		config.TaiwanDirectory = client
	}
	if config.TaiwanMarket == nil {
		config.TaiwanMarket = client
	}
	if config.TaiwanChip == nil {
		config.TaiwanChip = client
	}
	if config.TaiwanFundamentals == nil {
		config.TaiwanFundamentals = client
	}
	if config.TaiwanSnapshot == nil {
		config.TaiwanSnapshot = client
	}
	if config.TaiwanBreadth == nil {
		config.TaiwanBreadth = client
	}
	if config.TaiwanScreener == nil {
		config.TaiwanScreener = client
	}
	if config.TaiwanScreenerInstitutional == nil {
		config.TaiwanScreenerInstitutional = client
	}
	if config.TaiwanScreenerMargin == nil {
		config.TaiwanScreenerMargin = client
	}
	if config.TaiwanScreenerRevenue == nil {
		config.TaiwanScreenerRevenue = client
	}
	if config.TaiwanScreenerValuation == nil {
		config.TaiwanScreenerValuation = client
	}
	if config.TaiwanScreenerDividends == nil {
		config.TaiwanScreenerDividends = client
	}
	if config.TaiwanScreenerFinancials == nil {
		config.TaiwanScreenerFinancials = client
	}
	if config.TaiwanScreenerBalance == nil {
		config.TaiwanScreenerBalance = client
	}
	if config.TaiwanScreenerCashflow == nil {
		config.TaiwanScreenerCashflow = client
	}
	if config.TaiwanEmotion == nil {
		config.TaiwanEmotion = client
	}
	if config.TaiwanIndustryRadar == nil {
		config.TaiwanIndustryRadar = client
	}
	if config.TaiwanIntelligence == nil {
		config.TaiwanIntelligence = client
	}
}

func hasEveryTaiwanProvider(config Config) bool {
	return config.TaiwanDirectory != nil && config.TaiwanMarket != nil &&
		config.TaiwanChip != nil && config.TaiwanFundamentals != nil &&
		config.TaiwanSnapshot != nil && config.TaiwanBreadth != nil &&
		config.TaiwanScreener != nil && config.TaiwanScreenerInstitutional != nil &&
		config.TaiwanScreenerMargin != nil && config.TaiwanScreenerRevenue != nil &&
		config.TaiwanScreenerValuation != nil && config.TaiwanScreenerDividends != nil &&
		config.TaiwanScreenerFinancials != nil && config.TaiwanScreenerBalance != nil &&
		config.TaiwanScreenerCashflow != nil && config.TaiwanEmotion != nil &&
		config.TaiwanIndustryRadar != nil && config.TaiwanIntelligence != nil
}

func openPersistence(config *Config) error {
	var startupErrors []error
	if config.WatchlistStore == nil {
		store, err := taiwanwatchlist.OpenStore(config.WatchlistDBPath)
		if err != nil && config.StrictPersistence {
			startupErrors = append(startupErrors, fmt.Errorf("open taiwan watchlist database: %w", err))
		}
		if err != nil {
			store, _ = taiwanwatchlist.OpenStore(":memory:")
		}
		config.WatchlistStore = store
	}
	if config.TaiwanPortfolioStore == nil {
		store, err := taiwanportfolio.OpenStore(config.TaiwanPortfolioDBPath)
		if err != nil && config.StrictPersistence {
			startupErrors = append(startupErrors, fmt.Errorf("open Taiwan portfolio database: %w", err))
		}
		if err != nil {
			store, _ = taiwanportfolio.OpenStore(":memory:")
		}
		config.TaiwanPortfolioStore = store
	}
	if config.SettingsStore == nil {
		store, err := appsettings.Open(config.SettingsPath)
		if err != nil && config.StrictPersistence {
			startupErrors = append(startupErrors, fmt.Errorf("open settings: %w", err))
		}
		if err != nil {
			store, _ = appsettings.Open("")
		}
		config.SettingsStore = store
	}
	return errors.Join(startupErrors...)
}

func syncRuntimeSettings(store *appsettings.Store, runtime AgentRuntime) {
	values := store.Snapshot()
	var migratedKey *string
	if secret := strings.TrimSpace(values.LLM.APIKey); secret != "" {
		migratedKey = &secret
		if updated, err := store.Update(func(next *appsettings.Values) error {
			next.LLM.APIKey = ""
			return nil
		}); err == nil {
			values = updated
		}
	}
	if profiles, ok := runtime.(hermes.ProfileGateway); ok {
		if migratedKey == nil {
			if key, err := runtime.ModelAPIKey(); err == nil && strings.TrimSpace(key) != "" {
				migratedKey = &key
			}
		}
		_ = profiles.SyncLLMProfile(values.LLM, values.ActiveLLMProfileID, migratedKey)
		return
	}
	_ = runtime.SyncLLM(values.LLM, migratedKey)
}
