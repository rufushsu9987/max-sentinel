CREATE TABLE IF NOT EXISTS market_events (
    event_id TEXT PRIMARY KEY,
    source TEXT NOT NULL,
    event_type TEXT NOT NULL,
    market TEXT NOT NULL,
    sequence BIGINT NOT NULL DEFAULT 0,
    occurred_at TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_market_events_market_occurred
    ON market_events (market, occurred_at DESC);

CREATE TABLE IF NOT EXISTS risk_snapshots (
    snapshot_id TEXT PRIMARY KEY,
    market TEXT NOT NULL,
    observed_at TIMESTAMPTZ NOT NULL,
    last_event_at TIMESTAMPTZ NOT NULL,
    mid_price DOUBLE PRECISION NOT NULL,
    spread_bps DOUBLE PRECISION NOT NULL,
    book_imbalance DOUBLE PRECISION NOT NULL,
    realized_volatility DOUBLE PRECISION NOT NULL,
    price_shock_bps DOUBLE PRECISION NOT NULL,
    feed_age_seconds DOUBLE PRECISION NOT NULL,
    severity TEXT NOT NULL,
    reasons TEXT[] NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_risk_snapshots_market_observed
    ON risk_snapshots (market, observed_at DESC);
CREATE INDEX IF NOT EXISTS idx_risk_snapshots_severity_observed
    ON risk_snapshots (severity, observed_at DESC);
