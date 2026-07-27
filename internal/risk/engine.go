package risk

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/rufushsu9987/max-sentinel/internal/domain"
	"github.com/rufushsu9987/max-sentinel/internal/orderbook"
)

type Thresholds struct {
	WindowSize         int
	BookDepth          int
	WarnSpreadBPS      float64
	CriticalSpreadBPS  float64
	WarnVolatility     float64
	CriticalVolatility float64
	WarnImbalance      float64
	CriticalImbalance  float64
	WarnPriceShockBPS  float64
	CriticalShockBPS   float64
	StaleAfter         time.Duration
}

func DefaultThresholds() Thresholds {
	return Thresholds{
		WindowSize:         120,
		BookDepth:          10,
		WarnSpreadBPS:      35,
		CriticalSpreadBPS:  80,
		WarnVolatility:     0.015,
		CriticalVolatility: 0.030,
		WarnImbalance:      0.65,
		CriticalImbalance:  0.85,
		WarnPriceShockBPS:  80,
		CriticalShockBPS:   180,
		StaleAfter:         10 * time.Second,
	}
}

type marketState struct {
	book         *orderbook.Book
	prices       []float64
	lastEventAt  time.Time
	eventCount   int64
	lastMidPrice float64
	seen         map[string]struct{}
	seenOrder    []string
	lastSnapshot domain.RiskSnapshot
}

type Engine struct {
	mu         sync.Mutex
	thresholds Thresholds
	markets    map[string]*marketState
}

func NewEngine(thresholds Thresholds) *Engine {
	if thresholds.WindowSize <= 1 {
		thresholds.WindowSize = 120
	}
	if thresholds.BookDepth <= 0 {
		thresholds.BookDepth = 10
	}
	return &Engine{thresholds: thresholds, markets: make(map[string]*marketState)}
}

func (e *Engine) Apply(event domain.MarketEvent, now time.Time) domain.RiskSnapshot {
	e.mu.Lock()
	defer e.mu.Unlock()

	state := e.markets[event.Market]
	if state == nil {
		state = &marketState{book: orderbook.New(), seen: make(map[string]struct{})}
		e.markets[event.Market] = state
	}
	if event.EventID != "" {
		if _, duplicate := state.seen[event.EventID]; duplicate && state.lastSnapshot.Market != "" {
			return state.lastSnapshot
		}
	}

	switch event.Type {
	case domain.EventBookSnapshot:
		state.book.Reset(event.Bids, event.Asks)
	case domain.EventBookUpdate:
		state.book.Apply(event.Bids, event.Asks)
	}

	if !event.OccurredAt.IsZero() {
		state.lastEventAt = event.OccurredAt
	} else {
		state.lastEventAt = event.ReceivedAt
	}
	if state.lastEventAt.IsZero() {
		state.lastEventAt = now
	}
	state.eventCount++

	bestBid, bestAsk, mid, spreadBPS, imbalance, weightedMid, ok := state.book.Metrics(e.thresholds.BookDepth)
	if event.Ticker != nil {
		if tickerMid := tickerMidPrice(event.Ticker); tickerMid > 0 && !ok {
			mid = tickerMid
			bestBid = parseFloat(event.Ticker.Bid)
			bestAsk = parseFloat(event.Ticker.Ask)
			if bestBid > 0 && bestAsk > bestBid {
				spreadBPS = (bestAsk - bestBid) / mid * 10_000
			}
		}
	}
	if mid > 0 {
		state.prices = append(state.prices, mid)
		if len(state.prices) > e.thresholds.WindowSize {
			state.prices = state.prices[len(state.prices)-e.thresholds.WindowSize:]
		}
	}

	volatility := realizedVolatility(state.prices)
	shock := priceShockBPS(state.lastMidPrice, mid)
	if mid > 0 {
		state.lastMidPrice = mid
	}
	feedAge := now.Sub(state.lastEventAt).Seconds()
	if feedAge < 0 {
		feedAge = 0
	}

	severity, reasons := classify(e.thresholds, ok, spreadBPS, imbalance, volatility, shock, feedAge)
	identity := event.EventID
	if identity == "" {
		identity = now.UTC().Format(time.RFC3339Nano)
	}
	snapshot := domain.RiskSnapshot{
		SnapshotID:       snapshotID(event.Market, identity, state.eventCount),
		Market:           event.Market,
		ObservedAt:       now.UTC(),
		LastEventAt:      state.lastEventAt.UTC(),
		BestBid:          bestBid,
		BestAsk:          bestAsk,
		MidPrice:         mid,
		SpreadBPS:        spreadBPS,
		BookImbalance:    imbalance,
		WeightedMidPrice: weightedMid,
		RealizedVol:      volatility,
		PriceShockBPS:    shock,
		FeedAgeSeconds:   feedAge,
		Severity:         severity,
		Reasons:          reasons,
		EventCount:       state.eventCount,
	}
	state.lastSnapshot = snapshot
	if event.EventID != "" {
		state.seen[event.EventID] = struct{}{}
		state.seenOrder = append(state.seenOrder, event.EventID)
		if len(state.seenOrder) > 10_000 {
			oldest := state.seenOrder[0]
			state.seenOrder = state.seenOrder[1:]
			delete(state.seen, oldest)
		}
	}
	return snapshot
}

func (e *Engine) Evaluate(market string, now time.Time) (domain.RiskSnapshot, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	state := e.markets[market]
	if state == nil {
		return domain.RiskSnapshot{}, false
	}
	bestBid, bestAsk, mid, spreadBPS, imbalance, weightedMid, ok := state.book.Metrics(e.thresholds.BookDepth)
	volatility := realizedVolatility(state.prices)
	feedAge := now.Sub(state.lastEventAt).Seconds()
	severity, reasons := classify(e.thresholds, ok, spreadBPS, imbalance, volatility, 0, feedAge)
	return domain.RiskSnapshot{
		SnapshotID:       snapshotID(market, "evaluate:"+now.UTC().Format(time.RFC3339Nano), state.eventCount),
		Market:           market,
		ObservedAt:       now.UTC(),
		LastEventAt:      state.lastEventAt.UTC(),
		BestBid:          bestBid,
		BestAsk:          bestAsk,
		MidPrice:         mid,
		SpreadBPS:        spreadBPS,
		BookImbalance:    imbalance,
		WeightedMidPrice: weightedMid,
		RealizedVol:      volatility,
		FeedAgeSeconds:   math.Max(feedAge, 0),
		Severity:         severity,
		Reasons:          reasons,
		EventCount:       state.eventCount,
	}, true
}

func (e *Engine) Markets() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	markets := make([]string, 0, len(e.markets))
	for market := range e.markets {
		markets = append(markets, market)
	}
	sort.Strings(markets)
	return markets
}

func classify(t Thresholds, validBook bool, spread, imbalance, volatility, shock, feedAge float64) (domain.Severity, []string) {
	severity := domain.SeverityNormal
	reasons := make([]string, 0, 5)
	promote := func(next domain.Severity, reason string) {
		if next == domain.SeverityCritical || severity == domain.SeverityNormal {
			severity = next
		}
		reasons = append(reasons, reason)
	}
	if !validBook {
		promote(domain.SeverityWarning, "order book is incomplete")
	}
	if feedAge >= t.StaleAfter.Seconds() {
		promote(domain.SeverityCritical, "market data feed is stale")
	}
	if spread >= t.CriticalSpreadBPS {
		promote(domain.SeverityCritical, "spread exceeds critical threshold")
	} else if spread >= t.WarnSpreadBPS {
		promote(domain.SeverityWarning, "spread exceeds warning threshold")
	}
	absoluteImbalance := math.Abs(imbalance)
	if absoluteImbalance >= t.CriticalImbalance {
		promote(domain.SeverityCritical, "order book imbalance is extreme")
	} else if absoluteImbalance >= t.WarnImbalance {
		promote(domain.SeverityWarning, "order book imbalance is elevated")
	}
	if volatility >= t.CriticalVolatility {
		promote(domain.SeverityCritical, "realized volatility exceeds critical threshold")
	} else if volatility >= t.WarnVolatility {
		promote(domain.SeverityWarning, "realized volatility exceeds warning threshold")
	}
	if math.Abs(shock) >= t.CriticalShockBPS {
		promote(domain.SeverityCritical, "single-event price shock is critical")
	} else if math.Abs(shock) >= t.WarnPriceShockBPS {
		promote(domain.SeverityWarning, "single-event price shock is elevated")
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "market conditions are within configured limits")
	}
	return severity, reasons
}

func realizedVolatility(prices []float64) float64 {
	if len(prices) < 3 {
		return 0
	}
	returns := make([]float64, 0, len(prices)-1)
	for i := 1; i < len(prices); i++ {
		if prices[i-1] <= 0 || prices[i] <= 0 {
			continue
		}
		returns = append(returns, math.Log(prices[i]/prices[i-1]))
	}
	if len(returns) < 2 {
		return 0
	}
	var mean float64
	for _, value := range returns {
		mean += value
	}
	mean /= float64(len(returns))
	var variance float64
	for _, value := range returns {
		delta := value - mean
		variance += delta * delta
	}
	variance /= float64(len(returns) - 1)
	return math.Sqrt(variance)
}

func priceShockBPS(previous, current float64) float64 {
	if previous <= 0 || current <= 0 {
		return 0
	}
	return (current - previous) / previous * 10_000
}

func tickerMidPrice(t *domain.Ticker) float64 {
	bid, ask := parseFloat(t.Bid), parseFloat(t.Ask)
	if bid > 0 && ask > bid {
		return (bid + ask) / 2
	}
	return parseFloat(t.Last)
}

func parseFloat(value string) float64 {
	var parsed float64
	_, _ = fmtSscan(value, &parsed)
	return parsed
}

// fmtSscan is isolated to make the numerical parser easy to replace with fixed-point decimals.
var fmtSscan = func(value string, out *float64) (int, error) {
	return fmt.Sscan(value, out)
}

func snapshotID(market, identity string, count int64) string {
	sum := sha256.Sum256([]byte(market + identity + strconv.FormatInt(count, 10)))
	return hex.EncodeToString(sum[:12])
}
