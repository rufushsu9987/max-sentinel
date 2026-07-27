package streamguard

import (
	"testing"

	"github.com/rufushsu9987/max-sentinel/internal/domain"
)

func TestGateBuffersUntilSnapshot(t *testing.T) {
	gate := New(10)
	update := domain.MarketEvent{Type: domain.EventBookUpdate, Market: "btctwd", FirstUpdateID: 11, LastUpdateID: 11, BookVersion: 7}
	ready, err := gate.Accept(update)
	if err != nil || len(ready) != 0 {
		t.Fatalf("expected buffered update, ready=%d err=%v", len(ready), err)
	}
	snapshot := domain.MarketEvent{Type: domain.EventBookSnapshot, Market: "btctwd", LastUpdateID: 10, BookVersion: 7}
	ready, err = gate.Accept(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if len(ready) != 2 || ready[0].Type != domain.EventBookSnapshot || ready[1].LastUpdateID != 11 {
		t.Fatalf("unexpected released events: %#v", ready)
	}
}

func TestGateRejectsGap(t *testing.T) {
	gate := New(10)
	_, _ = gate.Accept(domain.MarketEvent{Type: domain.EventBookSnapshot, Market: "btctwd", LastUpdateID: 10, BookVersion: 7})
	_, err := gate.Accept(domain.MarketEvent{Type: domain.EventBookUpdate, Market: "btctwd", FirstUpdateID: 12, LastUpdateID: 12, BookVersion: 7})
	if err == nil {
		t.Fatal("expected update gap error")
	}
}

func TestGateRejectsVersionChange(t *testing.T) {
	gate := New(10)
	_, _ = gate.Accept(domain.MarketEvent{Type: domain.EventBookSnapshot, Market: "btctwd", LastUpdateID: 10, BookVersion: 7})
	_, err := gate.Accept(domain.MarketEvent{Type: domain.EventBookUpdate, Market: "btctwd", FirstUpdateID: 11, LastUpdateID: 11, BookVersion: 8})
	if err == nil {
		t.Fatal("expected version mismatch error")
	}
}

func TestGateDropsDuplicateBatch(t *testing.T) {
	gate := New(10)
	_, _ = gate.Accept(domain.MarketEvent{Type: domain.EventBookSnapshot, Market: "btctwd", LastUpdateID: 10, BookVersion: 7})
	ready, err := gate.Accept(domain.MarketEvent{Type: domain.EventBookUpdate, Market: "btctwd", FirstUpdateID: 9, LastUpdateID: 10, BookVersion: 7})
	if err != nil || len(ready) != 0 {
		t.Fatalf("expected duplicate batch to be dropped, ready=%d err=%v", len(ready), err)
	}
}
