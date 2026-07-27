package risk

import (
	"testing"
	"time"

	"github.com/rufushsu9987/max-sentinel/internal/domain"
)

func TestEngineNormalSnapshot(t *testing.T) {
	thresholds := DefaultThresholds()
	engine := NewEngine(thresholds)
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	event := domain.MarketEvent{
		EventID:    "evt-1",
		Type:       domain.EventBookSnapshot,
		Market:     "btctwd",
		OccurredAt: now,
		ReceivedAt: now,
		Bids:       []domain.PriceLevel{{Price: "3500000", Quantity: "1.2"}},
		Asks:       []domain.PriceLevel{{Price: "3501000", Quantity: "1.1"}},
	}
	snapshot := engine.Apply(event, now)
	if snapshot.Severity != domain.SeverityNormal {
		t.Fatalf("expected NORMAL, got %s (%v)", snapshot.Severity, snapshot.Reasons)
	}
	if snapshot.MidPrice != 3500500 {
		t.Fatalf("unexpected mid price: %v", snapshot.MidPrice)
	}
}

func TestEngineDetectsStaleFeed(t *testing.T) {
	thresholds := DefaultThresholds()
	thresholds.StaleAfter = 5 * time.Second
	engine := NewEngine(thresholds)
	started := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	engine.Apply(domain.MarketEvent{
		Type:       domain.EventBookSnapshot,
		Market:     "btctwd",
		OccurredAt: started,
		Bids:       []domain.PriceLevel{{Price: "100", Quantity: "1"}},
		Asks:       []domain.PriceLevel{{Price: "101", Quantity: "1"}},
	}, started)

	snapshot, ok := engine.Evaluate("btctwd", started.Add(6*time.Second))
	if !ok {
		t.Fatal("expected existing market")
	}
	if snapshot.Severity != domain.SeverityCritical {
		t.Fatalf("expected CRITICAL, got %s", snapshot.Severity)
	}
}

func TestEngineDetectsWideSpread(t *testing.T) {
	thresholds := DefaultThresholds()
	thresholds.WarnSpreadBPS = 10
	thresholds.CriticalSpreadBPS = 20
	engine := NewEngine(thresholds)
	now := time.Now().UTC()
	snapshot := engine.Apply(domain.MarketEvent{
		Type:       domain.EventBookSnapshot,
		Market:     "btctwd",
		OccurredAt: now,
		Bids:       []domain.PriceLevel{{Price: "100", Quantity: "1"}},
		Asks:       []domain.PriceLevel{{Price: "101", Quantity: "1"}},
	}, now)
	if snapshot.Severity != domain.SeverityCritical {
		t.Fatalf("expected CRITICAL, got %s (%v)", snapshot.Severity, snapshot.Reasons)
	}
}

func TestEngineDeduplicatesReplayedEvent(t *testing.T) {
	engine := NewEngine(DefaultThresholds())
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	event := domain.MarketEvent{
		EventID:    "stable-event-id",
		Type:       domain.EventBookSnapshot,
		Market:     "btctwd",
		OccurredAt: now,
		Bids:       []domain.PriceLevel{{Price: "100", Quantity: "1"}},
		Asks:       []domain.PriceLevel{{Price: "101", Quantity: "1"}},
	}
	first := engine.Apply(event, now)
	second := engine.Apply(event, now.Add(time.Second))
	if first.SnapshotID != second.SnapshotID {
		t.Fatalf("duplicate event produced a new snapshot: %s != %s", first.SnapshotID, second.SnapshotID)
	}
	if second.EventCount != 1 {
		t.Fatalf("duplicate event incremented event count: %d", second.EventCount)
	}
}
