package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rufushsu9987/max-sentinel/internal/api"
	"github.com/rufushsu9987/max-sentinel/internal/bus"
	"github.com/rufushsu9987/max-sentinel/internal/config"
	"github.com/rufushsu9987/max-sentinel/internal/domain"
	"github.com/rufushsu9987/max-sentinel/internal/runtimeid"
	"github.com/segmentio/kafka-go"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	state := api.NewState()
	server := api.NewServer(state, logger, cfg.PaperMaxNotionalTWD, cfg.StaleAfter)
	httpServer := &http.Server{
		Addr:              cfg.GatewayHTTPAddr,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      0, // SSE connections are intentionally long-lived.
		IdleTimeout:       60 * time.Second,
	}

	if cfg.Environment == "demo" {
		go runDemo(ctx, state, cfg.Markets)
	} else {
		groupID := runtimeid.ConsumerGroup(cfg.GatewayGroupID, runtimeid.Instance())
		consumer, err := bus.NewConsumer(cfg.KafkaBrokers, cfg.RiskSnapshotsTopic, groupID, logger, bus.Options{
			TLSEnabled:    cfg.KafkaTLSEnabled,
			SASLMechanism: cfg.KafkaSASLMechanism,
			SASLUsername:  cfg.KafkaSASLUsername,
			SASLPassword:  cfg.KafkaSASLPassword,
		})
		if err != nil {
			logger.Error("configure Kafka consumer", "error", err)
			os.Exit(1)
		}
		defer consumer.Close()
		logger.Info("gateway Kafka consumer configured", "group_id", groupID)
		go func() {
			err := consumer.Run(ctx, func(_ context.Context, message kafka.Message) error {
				var snapshot domain.RiskSnapshot
				if err := json.Unmarshal(message.Value, &snapshot); err != nil {
					return bus.Permanent(err)
				}
				state.Update(snapshot)
				return nil
			})
			if err != nil && !errors.Is(err, context.Canceled) {
				logger.Error("gateway consumer stopped", "error", err)
				stop()
			}
		}()
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	logger.Info("gateway started", "addr", cfg.GatewayHTTPAddr, "environment", cfg.Environment)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("gateway failed", "error", err)
		os.Exit(1)
	}
}

func runDemo(ctx context.Context, state *api.State, markets []string) {
	if len(markets) == 0 {
		markets = []string{"btctwd", "btcusd", "ethtwd"}
	}
	started := time.Now().UTC()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	base := map[string]float64{"btctwd": 3_450_000, "btcusd": 108_000, "ethtwd": 118_000}
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			elapsed := now.Sub(started).Seconds()
			for index, market := range markets {
				price := base[market]
				if price == 0 {
					price = 50_000 + float64(index)*10_000
				}
				wave := math.Sin(elapsed/8+float64(index)) * price * 0.0018
				mid := price + wave
				spreadBPS := 4 + math.Abs(math.Sin(elapsed/11))*8
				spread := mid * spreadBPS / 10_000
				imbalance := math.Sin(elapsed/6+float64(index)) * 0.55
				vol := 0.004 + math.Abs(math.Sin(elapsed/17))*0.008
				severity := domain.SeverityNormal
				reasons := []string{"market conditions are within configured limits"}
				if math.Abs(imbalance) > 0.5 {
					severity = domain.SeverityWarning
					reasons = []string{"order book imbalance is elevated"}
				}
				state.Update(domain.RiskSnapshot{
					SnapshotID:       now.Format("150405.000") + market,
					Market:           market,
					ObservedAt:       now.UTC(),
					LastEventAt:      now.UTC(),
					BestBid:          mid - spread/2,
					BestAsk:          mid + spread/2,
					MidPrice:         mid,
					SpreadBPS:        spreadBPS,
					BookImbalance:    imbalance,
					WeightedMidPrice: mid + imbalance*spread/2,
					RealizedVol:      vol,
					PriceShockBPS:    math.Cos(elapsed/5) * 3,
					FeedAgeSeconds:   0,
					Severity:         severity,
					Reasons:          reasons,
					EventCount:       int64(elapsed) * 10,
				})
			}
		}
	}
}
