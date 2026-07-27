# Performance and Reliability Test Plan

## Goals

Measure real performance rather than placing unsupported numbers on a résumé.

## Scenarios

1. **Steady state:** 1, 5, 10, and 25 markets with representative book/trade rates.
2. **Burst:** 10× event rate for 60 seconds.
3. **Reconnect:** terminate the MAX/WebSocket replay source and restore it.
4. **Duplicate:** replay 5% duplicate event IDs.
5. **Sequence gap:** remove one book delta and verify the stream guard disconnects, reconnects, and waits for a fresh snapshot before releasing buffered deltas.
6. **Broker interruption:** restart Redpanda while producers and consumers run.
7. **Poison message:** inject malformed JSON and verify evidence reaches the DLQ without blocking later records.
8. **Database degradation:** inject 200 ms write latency and temporary failures.
9. **Slow dashboard client:** throttle an SSE consumer.

## Metrics

- events received, normalized, dropped, and processed per second
- Kafka producer latency and consumer lag
- risk computation p50/p95/p99
- event receive → browser snapshot p50/p95/p99
- stale-snapshot ratio
- reconnect duration
- duplicate persistence rate
- DLQ publish rate, age, and replay success rate
- CPU and memory per service

## Suggested tools

- replay generator written in Go
- `vegeta` or `k6` for REST and SSE connection tests
- Prometheus recording rules
- `toxiproxy` for latency and disconnection injection
- `pprof` and Go execution traces

## Acceptance targets

Set targets before running tests. Treat the following as initial hypotheses only:

- no data races under `go test -race`
- zero duplicate persisted events
- no accepted paper orders while the feed is stale
- bounded memory under slow SSE clients
- service recovery without manual database repair
