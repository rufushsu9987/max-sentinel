package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rufushsu9987/max-sentinel/internal/domain"
)

type Server struct {
	state           *State
	logger          *slog.Logger
	maxNotionalTWD  float64
	staleAfter      time.Duration
	ordersTotal     *prometheus.CounterVec
	streamClients   prometheus.Gauge
	requestDuration *prometheus.HistogramVec
}

func NewServer(state *State, logger *slog.Logger, maxNotionalTWD float64, staleAfter time.Duration) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	if maxNotionalTWD <= 0 {
		maxNotionalTWD = 100_000
	}
	if staleAfter <= 0 {
		staleAfter = 10 * time.Second
	}
	server := &Server{
		state:          state,
		logger:         logger,
		maxNotionalTWD: maxNotionalTWD,
		staleAfter:     staleAfter,
		ordersTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "max_sentinel_paper_orders_total",
			Help: "Paper order decisions by result.",
		}, []string{"market", "result"}),
		streamClients: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "max_sentinel_sse_clients",
			Help: "Current number of SSE clients.",
		}),
		requestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "max_sentinel_http_request_duration_seconds",
			Help:    "HTTP request latency.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "route", "status"}),
	}
	prometheus.MustRegister(server.ordersTotal, server.streamClients, server.requestDuration)
	return server
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /readyz", s.ready)
	mux.HandleFunc("GET /api/v1/markets", s.markets)
	mux.HandleFunc("GET /api/v1/markets/{market}/snapshot", s.snapshot)
	mux.HandleFunc("GET /api/v1/stream", s.stream)
	mux.HandleFunc("POST /api/v1/paper/orders", s.paperOrder)
	mux.Handle("GET /metrics", promhttp.Handler())
	mux.Handle("GET /", s.staticHandler())
	return s.instrument(mux)
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "time": time.Now().UTC()})
}

func (s *Server) ready(w http.ResponseWriter, _ *http.Request) {
	if len(s.state.Markets()) == 0 {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "not_ready", "reason": "no market snapshot received"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ready"})
}

func (s *Server) markets(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"markets": s.state.Markets()})
}

func (s *Server) snapshot(w http.ResponseWriter, request *http.Request) {
	market := strings.ToLower(request.PathValue("market"))
	snapshot, ok := s.state.Snapshot(market)
	if !ok {
		writeError(w, http.StatusNotFound, "market snapshot not found")
		return
	}
	writeJSON(w, http.StatusOK, s.freshen(snapshot, time.Now().UTC()))
}

func (s *Server) stream(w http.ResponseWriter, request *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming is unsupported")
		return
	}
	marketFilter := strings.ToLower(strings.TrimSpace(request.URL.Query().Get("market")))
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	updates, cancel := s.state.Subscribe(32)
	defer cancel()
	s.streamClients.Inc()
	defer s.streamClients.Dec()

	if marketFilter != "" {
		if snapshot, exists := s.state.Snapshot(marketFilter); exists {
			_ = writeSSE(w, s.freshen(snapshot, time.Now().UTC()))
			flusher.Flush()
		}
	}
	refresh := time.NewTicker(5 * time.Second)
	defer refresh.Stop()
	for {
		select {
		case <-request.Context().Done():
			return
		case now := <-refresh.C:
			sent := false
			if marketFilter != "" {
				if snapshot, exists := s.state.Snapshot(marketFilter); exists {
					if err := writeSSE(w, s.freshen(snapshot, now.UTC())); err != nil {
						return
					}
					sent = true
				}
			} else {
				for _, market := range s.state.Markets() {
					if snapshot, exists := s.state.Snapshot(market); exists {
						if err := writeSSE(w, s.freshen(snapshot, now.UTC())); err != nil {
							return
						}
						sent = true
					}
				}
			}
			if !sent {
				_, _ = io.WriteString(w, ": keep-alive\n\n")
			}
			flusher.Flush()
		case snapshot, open := <-updates:
			if !open {
				return
			}
			if marketFilter != "" && snapshot.Market != marketFilter {
				continue
			}
			if err := writeSSE(w, s.freshen(snapshot, time.Now().UTC())); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (s *Server) paperOrder(w http.ResponseWriter, request *http.Request) {
	defer request.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	var input domain.PaperOrderRequest
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeError(w, http.StatusBadRequest, "request body must contain exactly one JSON object")
		return
	}
	decision, status := s.evaluatePaperOrder(input)
	result := "accepted"
	if !decision.Accepted {
		result = "rejected"
	}
	s.ordersTotal.WithLabelValues(decision.Market, result).Inc()
	writeJSON(w, status, decision)
}

func (s *Server) evaluatePaperOrder(input domain.PaperOrderRequest) (domain.PaperOrderDecision, int) {
	market := strings.ToLower(strings.TrimSpace(input.Market))
	side := strings.ToLower(strings.TrimSpace(input.Side))
	quantity, quantityErr := strconv.ParseFloat(input.Quantity, 64)
	decision := domain.PaperOrderDecision{
		OrderID:   randomID(),
		Market:    market,
		Side:      side,
		Quantity:  quantity,
		CreatedAt: time.Now().UTC(),
		Accepted:  true,
		Severity:  domain.SeverityNormal,
	}
	reject := func(reason string) {
		decision.Accepted = false
		decision.Reasons = append(decision.Reasons, reason)
	}
	if market == "" {
		reject("market is required")
	}
	if side != "buy" && side != "sell" {
		reject("side must be buy or sell")
	}
	if quantityErr != nil || quantity <= 0 || math.IsInf(quantity, 0) || math.IsNaN(quantity) {
		reject("quantity must be a positive finite number")
		quantity = 0
		decision.Quantity = 0
	}
	snapshot, ok := s.state.Snapshot(market)
	if !ok {
		reject("market snapshot is unavailable")
		decision.Severity = domain.SeverityCritical
		return decision, http.StatusConflict
	}
	snapshot = s.freshen(snapshot, time.Now().UTC())
	decision.Severity = snapshot.Severity
	if side == "buy" {
		decision.ReferencePrice = snapshot.BestAsk
	} else {
		decision.ReferencePrice = snapshot.BestBid
	}
	if decision.ReferencePrice <= 0 {
		decision.ReferencePrice = snapshot.MidPrice
	}
	decision.EstimatedNotional = decision.ReferencePrice * quantity
	if decision.EstimatedNotional > s.maxNotionalTWD {
		reject(fmt.Sprintf("estimated notional exceeds NT$%.0f paper-order limit", s.maxNotionalTWD))
	}
	if snapshot.Severity == domain.SeverityCritical {
		reject("critical market-risk state blocks new paper orders")
	}
	if snapshot.FeedAgeSeconds >= s.staleAfter.Seconds() {
		reject("market data is stale")
	}
	if input.LimitPrice != "" {
		limitPrice, err := strconv.ParseFloat(input.LimitPrice, 64)
		if err != nil || limitPrice <= 0 || math.IsInf(limitPrice, 0) || math.IsNaN(limitPrice) {
			reject("limit_price must be a positive finite number")
		} else if decision.ReferencePrice > 0 {
			decision.EstimatedSlippageBPS = math.Abs(limitPrice-decision.ReferencePrice) / decision.ReferencePrice * 10_000
			if decision.EstimatedSlippageBPS > 150 {
				reject("limit price is more than 150 bps from the current reference price")
			}
		}
	}
	if decision.Accepted {
		decision.Reasons = append(decision.Reasons, "paper order passed deterministic risk guardrails")
		return decision, http.StatusCreated
	}
	return decision, http.StatusUnprocessableEntity
}

func (s *Server) freshen(snapshot domain.RiskSnapshot, now time.Time) domain.RiskSnapshot {
	if snapshot.LastEventAt.IsZero() {
		snapshot.FeedAgeSeconds = s.staleAfter.Seconds() + 1
	} else {
		snapshot.FeedAgeSeconds = math.Max(now.Sub(snapshot.LastEventAt).Seconds(), 0)
	}
	if snapshot.FeedAgeSeconds >= s.staleAfter.Seconds() {
		snapshot.Severity = domain.SeverityCritical
		const staleReason = "market data feed is stale"
		found := false
		for _, reason := range snapshot.Reasons {
			if reason == staleReason {
				found = true
				break
			}
		}
		filtered := snapshot.Reasons[:0]
		for _, reason := range snapshot.Reasons {
			if reason != "market conditions are within configured limits" {
				filtered = append(filtered, reason)
			}
		}
		snapshot.Reasons = filtered
		if !found {
			snapshot.Reasons = append(snapshot.Reasons, staleReason)
		}
	}
	return snapshot
}

func (s *Server) staticHandler() http.Handler {
	root := "web"
	if _, err := os.Stat(filepath.Join(root, "index.html")); err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			writeError(w, http.StatusNotFound, "dashboard assets not found; run from repository root")
		})
	}
	fileServer := http.FileServer(http.Dir(root))
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if strings.HasPrefix(request.URL.Path, "/api/") || request.URL.Path == "/metrics" {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		fileServer.ServeHTTP(w, request)
	})
}

func (s *Server) instrument(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, request)
		route := request.Pattern
		if route == "" {
			route = "unmatched"
		}
		s.requestDuration.WithLabelValues(request.Method, route, strconv.Itoa(recorder.status)).Observe(time.Since(started).Seconds())
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(status int) {
	if r.wroteHeader {
		return
	}
	r.wroteHeader = true
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(payload []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	return r.ResponseWriter.Write(payload)
}

func (r *statusRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

func writeSSE(w io.Writer, snapshot domain.RiskSnapshot) error {
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "id: %s\nevent: risk.snapshot\ndata: %s\n\n", snapshot.SnapshotID, payload)
	return err
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": message, "status": status})
}

func randomID() string {
	buffer := make([]byte, 12)
	if _, err := rand.Read(buffer); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(buffer)
}
