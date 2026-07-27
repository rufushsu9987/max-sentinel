package api

import (
	"sort"
	"strings"
	"sync"

	"github.com/rufushsu9987/max-sentinel/internal/domain"
)

type State struct {
	mu          sync.RWMutex
	latest      map[string]domain.RiskSnapshot
	subscribers map[int]chan domain.RiskSnapshot
	nextID      int
}

func NewState() *State {
	return &State{
		latest:      make(map[string]domain.RiskSnapshot),
		subscribers: make(map[int]chan domain.RiskSnapshot),
	}
}

func (s *State) Update(snapshot domain.RiskSnapshot) {
	s.mu.Lock()
	s.latest[strings.ToLower(snapshot.Market)] = snapshot
	for _, subscriber := range s.subscribers {
		select {
		case subscriber <- snapshot:
		default:
		}
	}
	s.mu.Unlock()
}

func (s *State) Snapshot(market string) (domain.RiskSnapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snapshot, ok := s.latest[strings.ToLower(market)]
	return snapshot, ok
}

func (s *State) Markets() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	markets := make([]string, 0, len(s.latest))
	for market := range s.latest {
		markets = append(markets, market)
	}
	sort.Strings(markets)
	return markets
}

func (s *State) Subscribe(buffer int) (<-chan domain.RiskSnapshot, func()) {
	if buffer <= 0 {
		buffer = 16
	}
	s.mu.Lock()
	id := s.nextID
	s.nextID++
	channel := make(chan domain.RiskSnapshot, buffer)
	s.subscribers[id] = channel
	s.mu.Unlock()

	return channel, func() {
		s.mu.Lock()
		if existing, ok := s.subscribers[id]; ok {
			delete(s.subscribers, id)
			close(existing)
		}
		s.mu.Unlock()
	}
}
