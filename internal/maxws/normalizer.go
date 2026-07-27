package maxws

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rufushsu9987/max-sentinel/internal/domain"
)

// Normalize converts MAX WebSocket payloads into a stable internal event.
// The parser accepts both documented channel names and minor envelope changes,
// which keeps the adapter resilient while preserving the raw payload for audit.
func Normalize(raw []byte, receivedAt time.Time) (domain.MarketEvent, bool, error) {
	var envelope map[string]any
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return domain.MarketEvent{}, false, fmt.Errorf("decode MAX payload: %w", err)
	}
	if serverErr := parseServerError(envelope); serverErr != nil {
		return domain.MarketEvent{}, false, serverErr
	}
	if isControlMessage(envelope) {
		return domain.MarketEvent{}, false, nil
	}

	payload := envelope
	if nested, ok := envelope["data"].(map[string]any); ok {
		payload = mergeMaps(envelope, nested)
	}

	eventName := firstString(payload, "event", "type", "e")
	channel := firstString(payload, "channel", "c")
	if channel != "" && !strings.Contains(strings.ToLower(eventName), strings.ToLower(channel)) {
		eventName = channel + "." + eventName
	}
	market := strings.ToLower(firstString(payload, "market", "M", "m", "symbol"))
	if market == "" {
		if subscription, ok := payload["subscription"].(map[string]any); ok {
			market = strings.ToLower(firstString(subscription, "market", "symbol"))
		}
	}
	if market == "" {
		return domain.MarketEvent{}, false, nil
	}

	eventType := classifyEvent(eventName, payload)
	if eventType == "" {
		return domain.MarketEvent{}, false, nil
	}

	occurredAt := timestamp(payload, receivedAt)
	event := domain.MarketEvent{
		EventID:       eventID(raw),
		Source:        "max",
		Type:          eventType,
		Market:        market,
		Sequence:      firstInt64(payload, "last_id", "li", "sequence", "seq"),
		FirstUpdateID: firstInt64(payload, "first_id", "fi"),
		LastUpdateID:  firstInt64(payload, "last_id", "li"),
		BookVersion:   firstInt64(payload, "version", "v"),
		OccurredAt:    occurredAt,
		ReceivedAt:    receivedAt.UTC(),
		Raw:           append(json.RawMessage(nil), raw...),
	}

	switch eventType {
	case domain.EventBookSnapshot, domain.EventBookUpdate:
		event.Bids = parseLevels(firstValue(payload, "bids", "b"))
		event.Asks = parseLevels(firstValue(payload, "asks", "a"))
		if eventType == domain.EventBookUpdate && len(event.Bids) == 0 && len(event.Asks) == 0 {
			return domain.MarketEvent{}, false, nil
		}
	case domain.EventTradeUpdate:
		event.Trades = parseTrades(payload)
		if len(event.Trades) == 0 {
			return domain.MarketEvent{}, false, nil
		}
	case domain.EventTickerUpdate:
		event.Ticker = parseTicker(payload)
	}
	return event, true, nil
}

type ServerError struct {
	Messages []string
}

func (e *ServerError) Error() string {
	if len(e.Messages) == 0 {
		return "MAX WebSocket returned an error"
	}
	return "MAX WebSocket error: " + strings.Join(e.Messages, "; ")
}

func parseServerError(payload map[string]any) *ServerError {
	event := strings.ToLower(firstString(payload, "event", "e", "type"))
	if event != "error" {
		return nil
	}
	messages := make([]string, 0)
	if values, ok := firstValue(payload, "errors", "E").([]any); ok {
		for _, value := range values {
			if message := stringify(value); message != "" {
				messages = append(messages, message)
			}
		}
	} else if message := firstString(payload, "errors", "E", "message"); message != "" {
		messages = append(messages, message)
	}
	return &ServerError{Messages: messages}
}

func classifyEvent(name string, payload map[string]any) domain.EventType {
	name = strings.ToLower(name)
	switch {
	case strings.Contains(name, "book") && strings.Contains(name, "snapshot"):
		return domain.EventBookSnapshot
	case strings.Contains(name, "book"):
		if firstBool(payload, "snapshot", "is_snapshot") {
			return domain.EventBookSnapshot
		}
		return domain.EventBookUpdate
	case strings.Contains(name, "trade"):
		return domain.EventTradeUpdate
	case strings.Contains(name, "ticker"):
		return domain.EventTickerUpdate
	}
	if firstValue(payload, "bids", "b") != nil || firstValue(payload, "asks", "a") != nil {
		return domain.EventBookUpdate
	}
	if firstValue(payload, "trades", "t") != nil || firstValue(payload, "price", "p") != nil && firstValue(payload, "volume", "v", "quantity", "q") != nil {
		return domain.EventTradeUpdate
	}
	return ""
}

func parseLevels(value any) []domain.PriceLevel {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	levels := make([]domain.PriceLevel, 0, len(items))
	for _, item := range items {
		switch level := item.(type) {
		case []any:
			if len(level) < 2 {
				continue
			}
			price, quantity := stringify(level[0]), stringify(level[1])
			if price != "" && quantity != "" {
				levels = append(levels, domain.PriceLevel{Price: price, Quantity: quantity})
			}
		case map[string]any:
			price := firstString(level, "price", "p")
			quantity := firstString(level, "quantity", "volume", "q", "v")
			if price != "" && quantity != "" {
				levels = append(levels, domain.PriceLevel{Price: price, Quantity: quantity})
			}
		}
	}
	return levels
}

func parseTrades(payload map[string]any) []domain.Trade {
	if items, ok := firstValue(payload, "trades", "t").([]any); ok {
		trades := make([]domain.Trade, 0, len(items))
		for _, item := range items {
			if tradeMap, ok := item.(map[string]any); ok {
				if trade := parseTrade(tradeMap); trade.Price != "" {
					trades = append(trades, trade)
				}
			}
		}
		return trades
	}
	trade := parseTrade(payload)
	if trade.Price == "" {
		return nil
	}
	return []domain.Trade{trade}
}

func parseTrade(payload map[string]any) domain.Trade {
	return domain.Trade{
		Price:     firstString(payload, "price", "p"),
		Quantity:  firstString(payload, "volume", "quantity", "v", "q"),
		Side:      strings.ToLower(firstString(payload, "side", "sd", "S")),
		Trend:     strings.ToLower(firstString(payload, "trend", "tr")),
		TradeID:   firstString(payload, "trade_id", "id", "tid"),
		Timestamp: firstInt64(payload, "timestamp", "ts", "T", "t"),
	}
}

func parseTicker(payload map[string]any) *domain.Ticker {
	tickerPayload := payload
	if nested, ok := firstValue(payload, "ticker", "tk").(map[string]any); ok {
		tickerPayload = nested
	}
	timestampValue := firstInt64(tickerPayload, "timestamp", "ts", "T")
	if timestampValue == 0 {
		timestampValue = firstInt64(payload, "timestamp", "ts", "T")
	}
	return &domain.Ticker{
		Last:      firstString(tickerPayload, "last", "last_price", "close", "C"),
		Open:      firstString(tickerPayload, "open", "o", "O"),
		High:      firstString(tickerPayload, "high", "h", "H"),
		Low:       firstString(tickerPayload, "low", "l", "L"),
		Volume:    firstString(tickerPayload, "volume", "v"),
		Bid:       firstString(tickerPayload, "bid", "buy", "b"),
		Ask:       firstString(tickerPayload, "ask", "sell", "a"),
		Timestamp: timestampValue,
	}
}

func isControlMessage(payload map[string]any) bool {
	action := strings.ToLower(firstString(payload, "action", "event", "type"))
	return strings.Contains(action, "subscribed") || strings.Contains(action, "unsubscribed") || action == "pong" || action == "ping" || firstValue(payload, "success") == true
}

func timestamp(payload map[string]any, fallback time.Time) time.Time {
	value := firstInt64(payload, "timestamp", "ts", "T", "t")
	if value <= 0 {
		return fallback.UTC()
	}
	if value < 10_000_000_000 {
		return time.Unix(value, 0).UTC()
	}
	if value < 10_000_000_000_000 {
		return time.UnixMilli(value).UTC()
	}
	return time.UnixMicro(value).UTC()
}

func firstValue(payload map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, exists := payload[key]; exists {
			return value
		}
	}
	return nil
}

func firstString(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, exists := payload[key]; exists {
			if text := stringify(value); text != "" {
				return text
			}
		}
	}
	return ""
}

func firstInt64(payload map[string]any, keys ...string) int64 {
	for _, key := range keys {
		if value, exists := payload[key]; exists {
			switch typed := value.(type) {
			case float64:
				return int64(typed)
			case string:
				parsed, _ := strconv.ParseInt(typed, 10, 64)
				return parsed
			case json.Number:
				parsed, _ := typed.Int64()
				return parsed
			}
		}
	}
	return 0
}

func firstBool(payload map[string]any, keys ...string) bool {
	for _, key := range keys {
		if value, exists := payload[key]; exists {
			switch typed := value.(type) {
			case bool:
				return typed
			case string:
				parsed, _ := strconv.ParseBool(typed)
				return parsed
			}
		}
	}
	return false
}

func stringify(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case json.Number:
		return typed.String()
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	default:
		return ""
	}
}

func mergeMaps(base, nested map[string]any) map[string]any {
	merged := make(map[string]any, len(base)+len(nested))
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range nested {
		merged[key] = value
	}
	return merged
}

func eventID(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:16])
}
