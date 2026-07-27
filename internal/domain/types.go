package domain

import (
	"encoding/json"
	"time"
)

type EventType string

const (
	EventBookSnapshot EventType = "book.snapshot"
	EventBookUpdate   EventType = "book.update"
	EventTradeUpdate  EventType = "trade.update"
	EventTickerUpdate EventType = "ticker.update"
)

type PriceLevel struct {
	Price    string `json:"price"`
	Quantity string `json:"quantity"`
}

type Trade struct {
	Price     string `json:"price"`
	Quantity  string `json:"quantity"`
	Side      string `json:"side,omitempty"`
	Trend     string `json:"trend,omitempty"`
	TradeID   string `json:"trade_id,omitempty"`
	Timestamp int64  `json:"timestamp,omitempty"`
}

type Ticker struct {
	Last      string `json:"last,omitempty"`
	Open      string `json:"open,omitempty"`
	High      string `json:"high,omitempty"`
	Low       string `json:"low,omitempty"`
	Volume    string `json:"volume,omitempty"`
	Bid       string `json:"bid,omitempty"`
	Ask       string `json:"ask,omitempty"`
	Timestamp int64  `json:"timestamp,omitempty"`
}

type MarketEvent struct {
	EventID       string          `json:"event_id"`
	Source        string          `json:"source"`
	Type          EventType       `json:"type"`
	Market        string          `json:"market"`
	Sequence      int64           `json:"sequence,omitempty"`
	FirstUpdateID int64           `json:"first_update_id,omitempty"`
	LastUpdateID  int64           `json:"last_update_id,omitempty"`
	BookVersion   int64           `json:"book_version,omitempty"`
	OccurredAt    time.Time       `json:"occurred_at"`
	ReceivedAt    time.Time       `json:"received_at"`
	Bids          []PriceLevel    `json:"bids,omitempty"`
	Asks          []PriceLevel    `json:"asks,omitempty"`
	Trades        []Trade         `json:"trades,omitempty"`
	Ticker        *Ticker         `json:"ticker,omitempty"`
	Raw           json.RawMessage `json:"raw,omitempty"`
}

type Severity string

const (
	SeverityNormal   Severity = "NORMAL"
	SeverityWarning  Severity = "WARNING"
	SeverityCritical Severity = "CRITICAL"
)

type RiskSnapshot struct {
	SnapshotID       string    `json:"snapshot_id"`
	Market           string    `json:"market"`
	ObservedAt       time.Time `json:"observed_at"`
	LastEventAt      time.Time `json:"last_event_at"`
	BestBid          float64   `json:"best_bid"`
	BestAsk          float64   `json:"best_ask"`
	MidPrice         float64   `json:"mid_price"`
	SpreadBPS        float64   `json:"spread_bps"`
	BookImbalance    float64   `json:"book_imbalance"`
	WeightedMidPrice float64   `json:"weighted_mid_price"`
	RealizedVol      float64   `json:"realized_volatility"`
	PriceShockBPS    float64   `json:"price_shock_bps"`
	FeedAgeSeconds   float64   `json:"feed_age_seconds"`
	Severity         Severity  `json:"severity"`
	Reasons          []string  `json:"reasons"`
	EventCount       int64     `json:"event_count"`
}

type PaperOrderRequest struct {
	Market     string `json:"market"`
	Side       string `json:"side"`
	Quantity   string `json:"quantity"`
	LimitPrice string `json:"limit_price,omitempty"`
}

type PaperOrderDecision struct {
	OrderID              string    `json:"order_id"`
	Accepted             bool      `json:"accepted"`
	Market               string    `json:"market"`
	Side                 string    `json:"side"`
	Quantity             float64   `json:"quantity"`
	ReferencePrice       float64   `json:"reference_price"`
	EstimatedNotional    float64   `json:"estimated_notional_twd"`
	EstimatedSlippageBPS float64   `json:"estimated_slippage_bps"`
	Severity             Severity  `json:"severity"`
	Reasons              []string  `json:"reasons"`
	CreatedAt            time.Time `json:"created_at"`
}
