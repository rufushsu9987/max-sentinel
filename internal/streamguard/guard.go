package streamguard

import (
	"fmt"
	"strings"
	"sync"

	"github.com/rufushsu9987/max-sentinel/internal/domain"
)

// Gate enforces the MAX order-book update contract before events enter Kafka.
// Updates received before the first snapshot are buffered, stale updates are
// dropped, and sequence/version gaps force the WebSocket client to reconnect.
type Gate struct {
	mu          sync.Mutex
	maxBuffered int
	markets     map[string]*cursor
}

type cursor struct {
	ready    bool
	version  int64
	lastID   int64
	buffered []domain.MarketEvent
}

func New(maxBuffered int) *Gate {
	if maxBuffered <= 0 {
		maxBuffered = 512
	}
	return &Gate{maxBuffered: maxBuffered, markets: make(map[string]*cursor)}
}

// Accept returns zero or more events that are safe to publish. A snapshot may
// release updates that were buffered while waiting for that snapshot.
func (g *Gate) Accept(event domain.MarketEvent) ([]domain.MarketEvent, error) {
	if event.Type != domain.EventBookSnapshot && event.Type != domain.EventBookUpdate {
		return []domain.MarketEvent{event}, nil
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	market := strings.ToLower(strings.TrimSpace(event.Market))
	state := g.markets[market]
	if state == nil {
		state = &cursor{}
		g.markets[market] = state
	}

	if event.Type == domain.EventBookSnapshot {
		state.ready = true
		state.version = event.BookVersion
		state.lastID = lastUpdateID(event)
		ready := []domain.MarketEvent{event}
		buffered := state.buffered
		state.buffered = nil
		for _, update := range buffered {
			accepted, err := applyUpdate(market, state, update)
			if err != nil {
				delete(g.markets, market)
				return nil, err
			}
			ready = append(ready, accepted...)
		}
		return ready, nil
	}

	if !state.ready {
		state.buffered = append(state.buffered, event)
		if len(state.buffered) > g.maxBuffered {
			delete(g.markets, market)
			return nil, fmt.Errorf("%s order-book buffer exceeded %d updates before snapshot", market, g.maxBuffered)
		}
		return nil, nil
	}

	accepted, err := applyUpdate(market, state, event)
	if err != nil {
		delete(g.markets, market)
		return nil, err
	}
	return accepted, nil
}

func applyUpdate(market string, state *cursor, event domain.MarketEvent) ([]domain.MarketEvent, error) {
	if state.version > 0 && event.BookVersion > 0 && event.BookVersion != state.version {
		return nil, fmt.Errorf("%s order-book version changed from %d to %d", market, state.version, event.BookVersion)
	}
	if state.version == 0 && event.BookVersion > 0 {
		state.version = event.BookVersion
	}

	firstID := event.FirstUpdateID
	lastID := lastUpdateID(event)
	if firstID == 0 {
		firstID = lastID
	}

	// Some replay fixtures and compatibility payloads do not carry update IDs.
	// They are accepted, but live MAX events should include fi/li metadata.
	if state.lastID == 0 || lastID == 0 {
		if lastID > state.lastID {
			state.lastID = lastID
		}
		return []domain.MarketEvent{event}, nil
	}

	if lastID <= state.lastID {
		return nil, nil // stale or duplicate batch
	}
	expected := state.lastID + 1
	if firstID > expected || lastID < expected {
		return nil, fmt.Errorf(
			"%s order-book update gap: local_li=%d event_fi=%d event_li=%d",
			market, state.lastID, firstID, lastID,
		)
	}
	state.lastID = lastID
	return []domain.MarketEvent{event}, nil
}

func lastUpdateID(event domain.MarketEvent) int64 {
	if event.LastUpdateID > 0 {
		return event.LastUpdateID
	}
	return event.Sequence
}
