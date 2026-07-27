package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rufushsu9987/max-sentinel/internal/bus"
	"github.com/rufushsu9987/max-sentinel/internal/config"
	"github.com/rufushsu9987/max-sentinel/internal/domain"
	"github.com/rufushsu9987/max-sentinel/internal/maxws"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	events := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "max_sentinel_market_events_total", Help: "Normalized MAX events."}, []string{"market", "type"})
	dropped := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "max_sentinel_market_messages_dropped_total", Help: "Dropped MAX messages."}, []string{"reason"})
	publishLatency := prometheus.NewHistogram(prometheus.HistogramOpts{Name: "max_sentinel_market_publish_duration_seconds", Help: "Kafka publish latency.", Buckets: prometheus.DefBuckets})
	prometheus.MustRegister(events, dropped, publishLatency)

	publisher, err := bus.NewPublisher(cfg.KafkaBrokers, cfg.MarketEventsTopic, bus.Options{
		TLSEnabled:    cfg.KafkaTLSEnabled,
		SASLMechanism: cfg.KafkaSASLMechanism,
		SASLUsername:  cfg.KafkaSASLUsername,
		SASLPassword:  cfg.KafkaSASLPassword,
	})
	if err != nil {
		logger.Error("configure Kafka publisher", "error", err)
		os.Exit(1)
	}
	defer publisher.Close()

	var ready atomic.Bool
	client := &maxws.Client{
		URL:     cfg.MAXWebSocketURL,
		Markets: cfg.Markets,
		Depth:   cfg.BookDepth,
		Logger:  logger,
		OnRawDrop: func(reason string) {
			dropped.WithLabelValues(reason).Inc()
		},
		OnConnectionState: func(connected bool) {
			ready.Store(connected)
		},
		OnEvent: func(ctx context.Context, event domain.MarketEvent) error {
			started := time.Now()
			if err := publisher.Publish(ctx, event.Market, event); err != nil {
				return err
			}
			publishLatency.Observe(time.Since(started).Seconds())
			events.WithLabelValues(event.Market, string(event.Type)).Inc()
			return nil
		},
	}

	httpServer := &http.Server{Addr: cfg.MarketDataHTTPAddr, Handler: healthMux(&ready), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		logger.Info("marketdata metrics server started", "addr", cfg.MarketDataHTTPAddr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("marketdata metrics server failed", "error", err)
			stop()
		}
	}()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	if err := client.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("marketdata service stopped", "error", err)
		os.Exit(1)
	}
}

func healthMux(ready *atomic.Bool) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) {
		if ready == nil || !ready.Load() {
			http.Error(w, "MAX WebSocket is not connected", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})
	mux.Handle("GET /metrics", promhttp.Handler())
	return mux
}
