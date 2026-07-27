package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rufushsu9987/max-sentinel/internal/bus"
	"github.com/rufushsu9987/max-sentinel/internal/config"
	"github.com/rufushsu9987/max-sentinel/internal/domain"
	"github.com/rufushsu9987/max-sentinel/internal/grpcapi"
	riskengine "github.com/rufushsu9987/max-sentinel/internal/risk"
	"github.com/rufushsu9987/max-sentinel/internal/store"
	"github.com/segmentio/kafka-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

type provider struct{ engine *riskengine.Engine }

func (p provider) Snapshot(market string, now time.Time) (domain.RiskSnapshot, bool) {
	return p.engine.Evaluate(market, now)
}

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	thresholds := riskengine.DefaultThresholds()
	thresholds.WindowSize = cfg.RiskWindowSize
	thresholds.BookDepth = cfg.BookDepth
	thresholds.WarnSpreadBPS = cfg.WarnSpreadBPS
	thresholds.CriticalSpreadBPS = cfg.CriticalSpreadBPS
	thresholds.WarnVolatility = cfg.WarnVolatility
	thresholds.CriticalVolatility = cfg.CriticalVolatility
	thresholds.StaleAfter = cfg.StaleAfter
	engine := riskengine.NewEngine(thresholds)

	processed := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "max_sentinel_risk_events_total", Help: "Market events processed by risk service."}, []string{"market", "type"})
	severity := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "max_sentinel_risk_severity", Help: "Risk severity: 0 normal, 1 warning, 2 critical."}, []string{"market"})
	spread := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "max_sentinel_spread_bps", Help: "Current bid-ask spread in basis points."}, []string{"market"})
	volatility := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "max_sentinel_realized_volatility", Help: "Rolling realized volatility."}, []string{"market"})
	processingLatency := prometheus.NewHistogram(prometheus.HistogramOpts{Name: "max_sentinel_risk_processing_duration_seconds", Help: "Risk event processing latency.", Buckets: prometheus.DefBuckets})
	prometheus.MustRegister(processed, severity, spread, volatility, processingLatency)
	recordSnapshotMetrics := func(snapshot domain.RiskSnapshot) {
		spread.WithLabelValues(snapshot.Market).Set(snapshot.SpreadBPS)
		volatility.WithLabelValues(snapshot.Market).Set(snapshot.RealizedVol)
		severity.WithLabelValues(snapshot.Market).Set(severityValue(snapshot.Severity))
	}
	var publishedSeverity sync.Map

	publisher, err := bus.NewPublisher(cfg.KafkaBrokers, cfg.RiskSnapshotsTopic, bus.Options{
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
	consumer, err := bus.NewConsumer(cfg.KafkaBrokers, cfg.MarketEventsTopic, cfg.RiskGroupID, logger, bus.Options{
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

	var repository *store.Postgres
	if db, err := store.Open(ctx, cfg.DatabaseURL, logger); err != nil {
		if cfg.RequireDatabase {
			logger.Error("PostgreSQL is required but unavailable", "error", err)
			os.Exit(1)
		}
		logger.Warn("PostgreSQL unavailable; continuing without persistence in non-required mode", "error", err)
	} else {
		repository = db
		defer repository.Close()
	}

	grpcServer := grpc.NewServer()
	grpcapi.RegisterRiskServiceServer(grpcServer, grpcapi.NewRiskService(provider{engine: engine}))
	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	reflection.Register(grpcServer)
	listener, err := net.Listen("tcp", cfg.RiskGRPCAddr)
	if err != nil {
		logger.Error("listen gRPC", "error", err)
		os.Exit(1)
	}
	go func() {
		logger.Info("risk gRPC server started", "addr", cfg.RiskGRPCAddr)
		if err := grpcServer.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			logger.Error("risk gRPC server failed", "error", err)
			stop()
		}
	}()

	httpServer := &http.Server{Addr: cfg.RiskHTTPAddr, Handler: riskHTTPMux(), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		logger.Info("risk metrics server started", "addr", cfg.RiskHTTPAddr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("risk metrics server failed", "error", err)
			stop()
		}
	}()

	var shutdown sync.Once
	go func() {
		<-ctx.Done()
		shutdown.Do(func() {
			healthServer.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
			grpcServer.GracefulStop()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = httpServer.Shutdown(shutdownCtx)
		})
	}()

	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				for _, market := range engine.Markets() {
					snapshot, ok := engine.Evaluate(market, now.UTC())
					if !ok {
						continue
					}
					previous, exists := publishedSeverity.Load(market)
					if exists && previous == snapshot.Severity {
						continue
					}
					if repository != nil {
						if err := repository.SaveRiskSnapshot(ctx, snapshot); err != nil {
							logger.Error("persist freshness snapshot", "error", err, "snapshot_id", snapshot.SnapshotID)
							if cfg.RequireDatabase {
								continue
							}
						}
					}
					if err := publisher.Publish(ctx, snapshot.Market, snapshot); err != nil {
						logger.Error("publish freshness snapshot", "error", err, "market", market)
						continue
					}
					publishedSeverity.Store(market, snapshot.Severity)
					recordSnapshotMetrics(snapshot)
				}
			}
		}
	}()

	err = consumer.Run(ctx, func(ctx context.Context, message kafka.Message) error {
		started := time.Now()
		var event domain.MarketEvent
		if err := json.Unmarshal(message.Value, &event); err != nil {
			return bus.Permanent(err)
		}
		if repository != nil {
			if err := repository.SaveMarketEvent(ctx, event); err != nil {
				if cfg.RequireDatabase {
					return err
				}
				logger.Error("persist market event", "error", err, "event_id", event.EventID)
			}
		}
		snapshot := engine.Apply(event, time.Now().UTC())
		if repository != nil {
			if err := repository.SaveRiskSnapshot(ctx, snapshot); err != nil {
				if cfg.RequireDatabase {
					return err
				}
				logger.Error("persist risk snapshot", "error", err, "snapshot_id", snapshot.SnapshotID)
			}
		}
		if err := publisher.Publish(ctx, snapshot.Market, snapshot); err != nil {
			return err
		}
		publishedSeverity.Store(snapshot.Market, snapshot.Severity)
		processed.WithLabelValues(event.Market, string(event.Type)).Inc()
		recordSnapshotMetrics(snapshot)
		processingLatency.Observe(time.Since(started).Seconds())
		return nil
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("risk consumer stopped", "error", err)
		os.Exit(1)
	}
}

func severityValue(value domain.Severity) float64 {
	switch value {
	case domain.SeverityCritical:
		return 2
	case domain.SeverityWarning:
		return 1
	default:
		return 0
	}
}

func riskHTTPMux() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("GET /metrics", promhttp.Handler())
	return mux
}
