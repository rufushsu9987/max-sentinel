package orderbook

import (
	"sort"
	"strconv"
	"sync"

	"github.com/rufushsu9987/max-sentinel/internal/domain"
)

// Book is a concurrency-safe price-level order book. Quantities are stored as
// float64 for a compact portfolio demo; production financial systems should use
// fixed-point decimal integers or a decimal library.
type Book struct {
	mu   sync.RWMutex
	bids map[float64]float64
	asks map[float64]float64
}

func New() *Book {
	return &Book{
		bids: make(map[float64]float64),
		asks: make(map[float64]float64),
	}
}

func (b *Book) Reset(bids, asks []domain.PriceLevel) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.bids = make(map[float64]float64, len(bids))
	b.asks = make(map[float64]float64, len(asks))
	applyLevels(b.bids, bids)
	applyLevels(b.asks, asks)
}

func (b *Book) Apply(bids, asks []domain.PriceLevel) {
	b.mu.Lock()
	defer b.mu.Unlock()
	applyLevels(b.bids, bids)
	applyLevels(b.asks, asks)
}

func applyLevels(target map[float64]float64, levels []domain.PriceLevel) {
	for _, level := range levels {
		price, errPrice := strconv.ParseFloat(level.Price, 64)
		quantity, errQuantity := strconv.ParseFloat(level.Quantity, 64)
		if errPrice != nil || errQuantity != nil || price <= 0 {
			continue
		}
		if quantity <= 0 {
			delete(target, price)
			continue
		}
		target[price] = quantity
	}
}

func (b *Book) Best() (bestBid, bestAsk float64, ok bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if len(b.bids) == 0 || len(b.asks) == 0 {
		return 0, 0, false
	}
	for price := range b.bids {
		if price > bestBid {
			bestBid = price
		}
	}
	for price := range b.asks {
		if bestAsk == 0 || price < bestAsk {
			bestAsk = price
		}
	}
	return bestBid, bestAsk, bestBid > 0 && bestAsk > bestBid
}

func (b *Book) Top(depth int) (bids, asks [][2]float64) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if depth <= 0 {
		depth = 10
	}
	for price, quantity := range b.bids {
		bids = append(bids, [2]float64{price, quantity})
	}
	for price, quantity := range b.asks {
		asks = append(asks, [2]float64{price, quantity})
	}
	sort.Slice(bids, func(i, j int) bool { return bids[i][0] > bids[j][0] })
	sort.Slice(asks, func(i, j int) bool { return asks[i][0] < asks[j][0] })
	if len(bids) > depth {
		bids = bids[:depth]
	}
	if len(asks) > depth {
		asks = asks[:depth]
	}
	return bids, asks
}

func (b *Book) Metrics(depth int) (bestBid, bestAsk, mid, spreadBPS, imbalance, weightedMid float64, ok bool) {
	bids, asks := b.Top(depth)
	if len(bids) == 0 || len(asks) == 0 {
		return 0, 0, 0, 0, 0, 0, false
	}
	bestBid, bestAsk = bids[0][0], asks[0][0]
	if bestBid <= 0 || bestAsk <= bestBid {
		return 0, 0, 0, 0, 0, 0, false
	}
	mid = (bestBid + bestAsk) / 2
	spreadBPS = (bestAsk - bestBid) / mid * 10_000

	var bidVolume, askVolume float64
	for _, level := range bids {
		bidVolume += level[1]
	}
	for _, level := range asks {
		askVolume += level[1]
	}
	total := bidVolume + askVolume
	if total > 0 {
		imbalance = (bidVolume - askVolume) / total
		// Microprice / weighted mid: more ask liquidity moves the estimate toward bid and vice versa.
		weightedMid = (bestAsk*bidVolume + bestBid*askVolume) / total
	} else {
		weightedMid = mid
	}
	return bestBid, bestAsk, mid, spreadBPS, imbalance, weightedMid, true
}
