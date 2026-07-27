package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config contains runtime settings shared by the services.
type Config struct {
	Environment         string
	MAXWebSocketURL     string
	Markets             []string
	KafkaBrokers        []string
	KafkaTLSEnabled     bool
	KafkaSASLMechanism  string
	KafkaSASLUsername   string
	KafkaSASLPassword   string
	MarketEventsTopic   string
	RiskSnapshotsTopic  string
	DatabaseURL         string
	RequireDatabase     bool
	MarketDataHTTPAddr  string
	RiskGRPCAddr        string
	RiskHTTPAddr        string
	GatewayHTTPAddr     string
	RiskGroupID         string
	GatewayGroupID      string
	BookDepth           int
	RiskWindowSize      int
	WarnSpreadBPS       float64
	CriticalSpreadBPS   float64
	WarnVolatility      float64
	CriticalVolatility  float64
	StaleAfter          time.Duration
	PaperMaxNotionalTWD float64
}

func Load() Config {
	return Config{
		Environment:         env("APP_ENV", "development"),
		MAXWebSocketURL:     env("MAX_WS_URL", "wss://max-stream.maicoin.com/ws"),
		Markets:             csvEnv("MARKETS", []string{"btctwd", "ethtwd"}),
		KafkaBrokers:        csvEnv("KAFKA_BROKERS", []string{"localhost:19092"}),
		KafkaTLSEnabled:     boolEnv("KAFKA_TLS_ENABLED", false),
		KafkaSASLMechanism:  env("KAFKA_SASL_MECHANISM", ""),
		KafkaSASLUsername:   env("KAFKA_SASL_USERNAME", ""),
		KafkaSASLPassword:   env("KAFKA_SASL_PASSWORD", ""),
		MarketEventsTopic:   env("MARKET_EVENTS_TOPIC", "market.events.v1"),
		RiskSnapshotsTopic:  env("RISK_SNAPSHOTS_TOPIC", "risk.snapshots.v1"),
		DatabaseURL:         env("DATABASE_URL", "postgres://maxsentinel:maxsentinel@localhost:5432/maxsentinel?sslmode=disable"),
		RequireDatabase:     boolEnv("REQUIRE_DATABASE", false),
		MarketDataHTTPAddr:  env("MARKETDATA_HTTP_ADDR", ":8081"),
		RiskGRPCAddr:        env("RISK_GRPC_ADDR", ":9090"),
		RiskHTTPAddr:        env("RISK_HTTP_ADDR", ":8082"),
		GatewayHTTPAddr:     env("GATEWAY_HTTP_ADDR", ":8080"),
		RiskGroupID:         env("RISK_GROUP_ID", "max-sentinel-risk"),
		GatewayGroupID:      env("GATEWAY_GROUP_ID", "max-sentinel-gateway"),
		BookDepth:           intEnv("MAX_BOOK_DEPTH", 20),
		RiskWindowSize:      intEnv("RISK_WINDOW_SIZE", 120),
		WarnSpreadBPS:       floatEnv("RISK_WARN_SPREAD_BPS", 35),
		CriticalSpreadBPS:   floatEnv("RISK_CRITICAL_SPREAD_BPS", 80),
		WarnVolatility:      floatEnv("RISK_WARN_VOLATILITY", 0.015),
		CriticalVolatility:  floatEnv("RISK_CRITICAL_VOLATILITY", 0.030),
		StaleAfter:          time.Duration(intEnv("RISK_STALE_AFTER_SECONDS", 10)) * time.Second,
		PaperMaxNotionalTWD: floatEnv("PAPER_MAX_NOTIONAL_TWD", 100000),
	}
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func csvEnv(key string, fallback []string) []string {
	raw := env(key, strings.Join(fallback, ","))
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(strings.ToLower(part)); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func intEnv(key string, fallback int) int {
	value, err := strconv.Atoi(env(key, strconv.Itoa(fallback)))
	if err != nil {
		return fallback
	}
	return value
}

func floatEnv(key string, fallback float64) float64 {
	value, err := strconv.ParseFloat(env(key, strconv.FormatFloat(fallback, 'f', -1, 64)), 64)
	if err != nil {
		return fallback
	}
	return value
}

func boolEnv(key string, fallback bool) bool {
	value, err := strconv.ParseBool(env(key, strconv.FormatBool(fallback)))
	if err != nil {
		return fallback
	}
	return value
}
