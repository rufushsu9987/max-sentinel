package store

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rufushsu9987/max-sentinel/internal/domain"
)

type Postgres struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

func Open(ctx context.Context, databaseURL string, logger *slog.Logger) (*Postgres, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create PostgreSQL pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping PostgreSQL: %w", err)
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Postgres{pool: pool, logger: logger}, nil
}

func (p *Postgres) Close() { p.pool.Close() }

func (p *Postgres) SaveMarketEvent(ctx context.Context, event domain.MarketEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = p.pool.Exec(ctx, `
		INSERT INTO market_events(event_id, source, event_type, market, sequence, occurred_at, received_at, payload)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (event_id) DO NOTHING`,
		event.EventID, event.Source, event.Type, event.Market, event.Sequence, event.OccurredAt, event.ReceivedAt, payload,
	)
	return err
}

func (p *Postgres) SaveRiskSnapshot(ctx context.Context, snapshot domain.RiskSnapshot) error {
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	_, err = p.pool.Exec(ctx, `
		INSERT INTO risk_snapshots(
			snapshot_id, market, observed_at, last_event_at, mid_price, spread_bps,
			book_imbalance, realized_volatility, price_shock_bps, feed_age_seconds,
			severity, reasons, payload
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT (snapshot_id) DO NOTHING`,
		snapshot.SnapshotID, snapshot.Market, snapshot.ObservedAt, snapshot.LastEventAt,
		snapshot.MidPrice, snapshot.SpreadBPS, snapshot.BookImbalance, snapshot.RealizedVol,
		snapshot.PriceShockBPS, snapshot.FeedAgeSeconds, snapshot.Severity, snapshot.Reasons, payload,
	)
	return err
}
