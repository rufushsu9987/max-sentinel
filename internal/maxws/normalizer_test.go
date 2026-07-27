package maxws

import (
	"testing"
	"time"

	"github.com/rufushsu9987/max-sentinel/internal/domain"
)

func TestNormalizeBookSnapshot(t *testing.T) {
	raw := []byte(`{"event":"book.snapshot","market":"btctwd","bids":[["3500000","1.2"]],"asks":[["3501000","0.8"]],"timestamp":1785144000000}`)
	event, ok, err := Normalize(raw, time.Now().UTC())
	if err != nil || !ok {
		t.Fatalf("normalize failed: ok=%v err=%v", ok, err)
	}
	if event.Type != domain.EventBookSnapshot || event.Market != "btctwd" {
		t.Fatalf("unexpected event: %#v", event)
	}
	if len(event.Bids) != 1 || event.Bids[0].Price != "3500000" {
		t.Fatalf("unexpected bids: %#v", event.Bids)
	}
}

func TestNormalizeCompactBookUpdate(t *testing.T) {
	raw := []byte(`{"e":"book.update","M":"ethtwd","b":[["120000","2"]],"a":[["120100","1"]],"T":1785144000000}`)
	event, ok, err := Normalize(raw, time.Now().UTC())
	if err != nil || !ok {
		t.Fatalf("normalize failed: ok=%v err=%v", ok, err)
	}
	if event.Type != domain.EventBookUpdate || event.Market != "ethtwd" {
		t.Fatalf("unexpected event: %#v", event)
	}
}

func TestNormalizeMAXChannelEnvelope(t *testing.T) {
	raw := []byte(`{"c":"book","e":"snapshot","M":"btctwd","b":[["3500000","1.2"]],"a":[["3501000","0.8"]],"T":1785144000000}`)
	event, ok, err := Normalize(raw, time.Now().UTC())
	if err != nil || !ok {
		t.Fatalf("normalize failed: ok=%v err=%v", ok, err)
	}
	if event.Type != domain.EventBookSnapshot {
		t.Fatalf("expected snapshot, got %s", event.Type)
	}
}

func TestEventIDIsStableForReplay(t *testing.T) {
	raw := []byte(`{"event":"ticker.update","market":"btctwd","last":"3500000"}`)
	first, ok, err := Normalize(raw, time.Date(2026, 7, 27, 1, 0, 0, 0, time.UTC))
	if err != nil || !ok {
		t.Fatalf("first normalize failed: ok=%v err=%v", ok, err)
	}
	second, ok, err := Normalize(raw, time.Date(2026, 7, 27, 1, 0, 1, 0, time.UTC))
	if err != nil || !ok {
		t.Fatalf("second normalize failed: ok=%v err=%v", ok, err)
	}
	if first.EventID != second.EventID {
		t.Fatalf("expected stable event ID, got %s and %s", first.EventID, second.EventID)
	}
}

func TestNormalizeOfficialOrderBookMetadata(t *testing.T) {
	raw := []byte(`{"c":"book","e":"snapshot","M":"btctwd","a":[],"b":[],"T":1591869939634,"fi":12141725,"li":12141725,"v":1725602999632}`)
	event, ok, err := Normalize(raw, time.Now().UTC())
	if err != nil || !ok {
		t.Fatalf("normalize failed: ok=%v err=%v", ok, err)
	}
	if event.FirstUpdateID != 12141725 || event.LastUpdateID != 12141725 || event.BookVersion != 1725602999632 {
		t.Fatalf("unexpected metadata: %#v", event)
	}
	if event.Type != domain.EventBookSnapshot {
		t.Fatalf("expected empty snapshot to be retained, got %s", event.Type)
	}
}

func TestNormalizeOfficialCompactTrade(t *testing.T) {
	raw := []byte(`{"c":"trade","e":"update","M":"btctwd","t":[{"p":"5337.3","v":"0.1","T":123456789,"tr":"up"}],"T":123456789}`)
	event, ok, err := Normalize(raw, time.Now().UTC())
	if err != nil || !ok {
		t.Fatalf("normalize failed: ok=%v err=%v", ok, err)
	}
	if len(event.Trades) != 1 || event.Trades[0].Price != "5337.3" || event.Trades[0].Trend != "up" {
		t.Fatalf("unexpected trade: %#v", event.Trades)
	}
}

func TestNormalizeOfficialNestedTicker(t *testing.T) {
	raw := []byte(`{"c":"ticker","e":"update","M":"btctwd","tk":{"M":"btctwd","O":"280007.1","H":"280017.2","L":"280005.3","C":"280004.5","v":"71.01"},"T":123456789}`)
	event, ok, err := Normalize(raw, time.Now().UTC())
	if err != nil || !ok {
		t.Fatalf("normalize failed: ok=%v err=%v", ok, err)
	}
	if event.Ticker == nil || event.Ticker.Last != "280004.5" || event.Ticker.Open != "280007.1" {
		t.Fatalf("unexpected ticker: %#v", event.Ticker)
	}
}

func TestNormalizeOfficialErrorResponse(t *testing.T) {
	raw := []byte(`{"e":"error","E":["E-1004: invalid action"],"i":"client1","T":1678096431125}`)
	_, ok, err := Normalize(raw, time.Now().UTC())
	if ok || err == nil {
		t.Fatalf("expected MAX server error, ok=%v err=%v", ok, err)
	}
	serverErr, typed := err.(*ServerError)
	if !typed || len(serverErr.Messages) != 1 {
		t.Fatalf("unexpected error: %#v", err)
	}
}
