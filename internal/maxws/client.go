package maxws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rufushsu9987/max-sentinel/internal/domain"
	"github.com/rufushsu9987/max-sentinel/internal/streamguard"
)

type Client struct {
	URL               string
	Markets           []string
	Depth             int
	Logger            *slog.Logger
	OnEvent           func(context.Context, domain.MarketEvent) error
	OnRawDrop         func(reason string)
	OnConnectionState func(connected bool)
}

type retryAfterError struct {
	cause error
	until time.Time
}

func (e *retryAfterError) Error() string {
	return fmt.Sprintf("%v; retry after %s", e.cause, e.until.UTC().Format(time.RFC3339))
}

func (e *retryAfterError) Unwrap() error { return e.cause }

func (c *Client) Run(ctx context.Context) error {
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
	if c.OnEvent == nil {
		return errors.New("maxws: OnEvent callback is required")
	}
	backoff := time.Second
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		connectedAt := time.Now()
		err := c.runConnection(ctx)
		if err == nil || errors.Is(err, context.Canceled) {
			return err
		}
		if time.Since(connectedAt) >= 2*time.Minute {
			backoff = time.Second
		}
		jitter := time.Duration(rand.Int63n(int64(backoff / 2)))
		wait := backoff + jitter
		if retryWait, ok := retryDelay(err, time.Now()); ok && retryWait > wait {
			wait = retryWait
		}
		c.Logger.Warn("MAX WebSocket disconnected; reconnecting", "error", err, "backoff", wait)
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

func (c *Client) runConnection(ctx context.Context) error {
	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second, Proxy: http.ProxyFromEnvironment}
	conn, response, err := dialer.DialContext(ctx, c.URL, http.Header{"User-Agent": []string{"max-sentinel/1.0"}})
	if err != nil {
		if response != nil {
			status := response.Status
			retryHeader := response.Header.Get("Retry-After")
			_ = response.Body.Close()
			wrapped := fmt.Errorf("dial MAX WebSocket: %w (status=%s)", err, status)
			if response.StatusCode == http.StatusTooManyRequests {
				if until, ok := parseRetryAfter(retryHeader, time.Now()); ok {
					return &retryAfterError{cause: wrapped, until: until}
				}
			}
			return wrapped
		}
		return fmt.Errorf("dial MAX WebSocket: %w", err)
	}
	defer conn.Close()
	conn.SetReadLimit(8 << 20)
	_ = conn.SetReadDeadline(time.Now().Add(75 * time.Second))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(75 * time.Second))
	})
	conn.SetPingHandler(func(message string) error {
		_ = conn.SetReadDeadline(time.Now().Add(75 * time.Second))
		return conn.WriteControl(websocket.PongMessage, []byte(message), time.Now().Add(5*time.Second))
	})

	if err := conn.WriteJSON(c.subscriptionRequest()); err != nil {
		return fmt.Errorf("subscribe MAX channels: %w", err)
	}
	c.connectionState(true)
	defer c.connectionState(false)
	c.Logger.Info("connected to MAX WebSocket", "markets", c.Markets, "depth", c.Depth)

	connectionCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go c.keepAlive(connectionCtx, conn)
	gate := streamguard.New(512)

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read MAX WebSocket: %w", err)
		}
		now := time.Now().UTC()
		event, ok, err := Normalize(raw, now)
		if err != nil {
			var serverErr *ServerError
			if errors.As(err, &serverErr) {
				c.drop("server_error")
				return serverErr
			}
			c.drop("decode_error")
			c.Logger.Debug("dropping malformed MAX message", "error", err, "raw", string(raw))
			continue
		}
		if !ok {
			c.drop("control_or_unknown")
			continue
		}
		ready, err := gate.Accept(event)
		if err != nil {
			c.drop("book_sequence_error")
			return fmt.Errorf("validate MAX order-book stream: %w", err)
		}
		for _, accepted := range ready {
			if err := c.OnEvent(ctx, accepted); err != nil {
				return fmt.Errorf("handle MAX event: %w", err)
			}
		}
	}
}

func (c *Client) keepAlive(ctx context.Context, conn *websocket.Conn) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := conn.WriteControl(websocket.PingMessage, []byte("max-sentinel"), time.Now().Add(5*time.Second)); err != nil {
				c.Logger.Warn("send MAX WebSocket ping", "error", err)
				_ = conn.Close()
				return
			}
		}
	}
}

func (c *Client) subscriptionRequest() subscribeRequest {
	return newSubscriptionRequest(c.Markets, c.Depth)
}

func (c *Client) drop(reason string) {
	if c.OnRawDrop != nil {
		c.OnRawDrop(reason)
	}
}

func (c *Client) connectionState(connected bool) {
	if c.OnConnectionState != nil {
		c.OnConnectionState(connected)
	}
}

func retryDelay(err error, now time.Time) (time.Duration, bool) {
	var retryErr *retryAfterError
	if !errors.As(err, &retryErr) {
		return 0, false
	}
	wait := time.Until(retryErr.until)
	if !now.IsZero() {
		wait = retryErr.until.Sub(now)
	}
	if wait < 0 {
		wait = 0
	}
	return wait, true
}

func parseRetryAfter(value string, now time.Time) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	if number, err := strconv.ParseInt(value, 10, 64); err == nil {
		// MAX documents this header as a Unix timestamp; accepting a short
		// delta-seconds value also preserves normal HTTP Retry-After behavior.
		if number > 1_000_000_000 {
			return time.Unix(number, 0), true
		}
		return now.Add(time.Duration(number) * time.Second), true
	}
	parsed, err := http.ParseTime(value)
	return parsed, err == nil
}

func SubscriptionJSON(markets []string, depth int) ([]byte, error) {
	return json.Marshal(newSubscriptionRequest(markets, depth))
}
