package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nanachi1212/mystocktracer/backend/internal/agent/hermesadapter"
	"github.com/nanachi1212/mystocktracer/backend/internal/httpapi"
	"github.com/nanachi1212/mystocktracer/backend/internal/runtimelog"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	cfg := loadRuntimeConfig()
	if cfg.logDirectory != "" {
		logger, closer, err := runtimelog.ConfigureStandard(cfg.logDirectory, "backend")
		if err != nil {
			log.Printf("runtime logging unavailable: %v", err)
		} else {
			defer closer.Close()
			logger.Printf("level=info event=runtime_start component=backend version=%q", cfg.version)
		}
	}

	agent := hermesadapter.New(hermesadapter.Config{
		RuntimeRoot: cfg.hermesRuntimeRoot,
		Home:        cfg.hermesHome,
		WorkDir:     cfg.hermesWorkDir,
		PythonPath:  cfg.hermesPython,
	})
	api := httpapi.NewServer(httpapi.Config{
		Token:                  cfg.token,
		WatchlistDBPath:        cfg.watchlistDBPath,
		TaiwanPortfolioDBPath:  cfg.portfolioDBPath,
		SettingsPath:           cfg.settingsPath,
		AgentRuntime:           agent,
		Logger:                 log.Default(),
		StrictPersistence:      true,
		TaiwanCashflowCacheDir: cfg.cashflowCacheDirectory,
		ToAlphaMOPSEnabled:     cfg.toAlphaMOPSEnabled,
		ToAlphaMOPSEndpoint:    cfg.toAlphaMOPSEndpoint,
	})
	if err := api.StartupError(); err != nil {
		_ = api.Close()
		return fmtError("persistent data startup failed", err)
	}
	defer func() {
		if err := api.Close(); err != nil {
			log.Printf("close persistent data: %v", err)
		}
	}()

	httpServer := &http.Server{
		Addr:              cfg.address,
		Handler:           api,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       90 * time.Second,
	}
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("HTTP graceful shutdown: %v", err)
		}
	}()

	log.Printf("mystocktracer data foundation listening on http://%s", cfg.address)
	err := httpServer.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	<-shutdownDone
	return nil
}

func fmtError(message string, err error) error {
	return &startupError{message: message, cause: err}
}

type startupError struct {
	message string
	cause   error
}

func (e *startupError) Error() string { return e.message + ": " + e.cause.Error() }
func (e *startupError) Unwrap() error { return e.cause }
