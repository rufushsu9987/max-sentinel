package orderbook

import (
	"math"
	"testing"

	"github.com/rufushsu9987/max-sentinel/internal/domain"
)

func TestBookSnapshotAndUpdate(t *testing.T) {
	book := New()
	book.Reset(
		[]domain.PriceLevel{{Price: "100", Quantity: "2"}, {Price: "99", Quantity: "3"}},
		[]domain.PriceLevel{{Price: "101", Quantity: "1"}, {Price: "102", Quantity: "4"}},
	)

	bid, ask, ok := book.Best()
	if !ok || bid != 100 || ask != 101 {
		t.Fatalf("unexpected best prices: bid=%v ask=%v ok=%v", bid, ask, ok)
	}

	book.Apply(
		[]domain.PriceLevel{{Price: "100", Quantity: "0"}, {Price: "100.5", Quantity: "1"}},
		[]domain.PriceLevel{{Price: "101", Quantity: "2"}},
	)
	bid, ask, ok = book.Best()
	if !ok || bid != 100.5 || ask != 101 {
		t.Fatalf("unexpected updated prices: bid=%v ask=%v ok=%v", bid, ask, ok)
	}
}

func TestBookMetrics(t *testing.T) {
	book := New()
	book.Reset(
		[]domain.PriceLevel{{Price: "100", Quantity: "4"}},
		[]domain.PriceLevel{{Price: "102", Quantity: "1"}},
	)
	bid, ask, mid, spread, imbalance, weightedMid, ok := book.Metrics(5)
	if !ok {
		t.Fatal("expected valid metrics")
	}
	if bid != 100 || ask != 102 || mid != 101 {
		t.Fatalf("unexpected prices: %v %v %v", bid, ask, mid)
	}
	if math.Abs(spread-198.01980198) > 0.0001 {
		t.Fatalf("unexpected spread: %v", spread)
	}
	if math.Abs(imbalance-0.6) > 0.0001 {
		t.Fatalf("unexpected imbalance: %v", imbalance)
	}
	if math.Abs(weightedMid-101.6) > 0.0001 {
		t.Fatalf("unexpected weighted mid: %v", weightedMid)
	}
}
