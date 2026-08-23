package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Fanju6/sing-box-observability/src/server/internal/api"
	"github.com/Fanju6/sing-box-observability/src/server/internal/auth"
	"github.com/Fanju6/sing-box-observability/src/server/internal/buildinfo"
	"github.com/Fanju6/sing-box-observability/src/server/internal/collector"
	"github.com/Fanju6/sing-box-observability/src/server/internal/config"
	"github.com/Fanju6/sing-box-observability/src/server/internal/events"
	"github.com/Fanju6/sing-box-observability/src/server/internal/source"
	"github.com/Fanju6/sing-box-observability/src/server/internal/storage"
	"github.com/Fanju6/sing-box-observability/src/server/internal/webui"
)

func main() {
	configPath := flag.String("config", "server.yaml", "path to YAML configuration")
	showVersion := flag.Bool("version", false, "print version and build metadata")
	flag.Parse()
	if *showVersion {
		fmt.Println(buildinfo.String())
		return
	}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("configuration rejected", "error", err)
		os.Exit(2)
	}
	client, err := source.NewClient(cfg.Singbox.BaseURL, cfg.Singbox.Token)
	if err != nil {
		logger.Error("upstream configuration rejected", "error", err)
		os.Exit(2)
	}
	store, err := storage.Open(cfg.Storage.Path)
	if err != nil {
		logger.Error("storage unavailable", "error", err)
		os.Exit(2)
	}
	defer store.Close()
	hub := events.NewHub()
	collectorService := collector.New(client, store, hub, cfg, logger)
	consoleAuth := auth.New(cfg.Console.AuthToken, cfg.Console.SessionTTL, cfg.Server.SecureCookie)
	consoleAuth.SetTrustedProxies(cfg.Server.TrustedProxies)
	apiHandler := api.New(collectorService, store, consoleAuth, cfg, logger).Handler()
	handler := webui.Handler(apiHandler)
	if !consoleAuth.Enabled() {
		handler = auth.LoopbackHostOnly(handler)
	}
	server := &http.Server{Addr: cfg.Server.Listen, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 0, IdleTimeout: 60 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	collectorService.Run(ctx)
	go func() {
		logger.Info("observability console listening", "listen", cfg.Server.Listen, "source", cfg.Singbox.Name)
	}()
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("HTTP server stopped", "error", err)
			stop()
		}
	}()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	collectorService.Stop()
	_ = server.Shutdown(shutdownCtx)
}
