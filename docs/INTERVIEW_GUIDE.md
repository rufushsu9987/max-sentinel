# Interview Guide

## 90-second project introduction

> 我做了一個以 MAX 公開 WebSocket 為資料來源的即時市場風控平台 MAX Sentinel。系統由三個 Go 微服務組成：marketdata 負責訂單簿、成交與 ticker 正規化；risk 透過 Kafka-compatible event stream 重建 L2 order book，計算 spread、imbalance、realized volatility、price shock 與資料新鮮度；gateway 用 REST 與 SSE 提供即時 dashboard，並實作 deterministic paper-order guardrails。Audit events 會以 event ID 做 idempotent persistence；暫時性失敗保留 Kafka offset 重試，poison message 則進入 DLQ。專案另外附上 gRPC contract、Prometheus/Grafana、Docker Compose、Kubernetes 與 Terraform。AI 只負責未來的事件解釋，不參與訂單核准，確保風控可測試、可稽核。

## Why this project is relevant

It is not another price dashboard. The difficult parts are exchange-specific:

- maintaining a correct order book from snapshots and deltas
- handling reconnects, duplicates, ordering, and stale feeds
- balancing latency, reliability, and auditability
- preventing unsafe actions during abnormal market conditions
- operating multiple services with observable SLOs

## Likely questions and strong answers

### Why Kafka / Redpanda instead of direct gRPC between all services?

The event bus decouples ingestion from risk consumers, preserves replay capability, and allows new consumers such as surveillance, analytics, and data-lake sinks. gRPC remains appropriate for synchronous point queries. The trade-off is greater operational complexity and eventual consistency.

### How do you guarantee order-book correctness?

Use snapshot plus ordered deltas, track sequence numbers per market, reject gaps, and request a new snapshot after reconnect or gap detection. Route each market to one Kafka partition. The current adapter preserves raw messages so parser defects can be reproduced.

### What happens when PostgreSQL is down?

The behavior is environment-controlled. In production-required mode, startup fails when PostgreSQL is unavailable, and runtime audit-write failures keep the Kafka offset uncommitted so the same record is retried. Optional demo mode may continue without persistence. This makes the durability trade-off explicit rather than silently losing evidence.

### Why SSE rather than WebSocket for the dashboard?

The browser only needs server-to-client updates, so SSE gives simpler reconnection semantics and works well with HTTP infrastructure. MAX ingestion still uses WebSocket because the upstream protocol is bidirectional and persistent.

### Why not use an LLM for risk decisions?

Financial guardrails must be deterministic, low-latency, reproducible, and auditable. An LLM can summarize evidence or retrieve runbooks, but must not override or approve an order.

### How would you scale it?

Gateway replicas can scale horizontally because each instance receives the full low-volume risk-snapshot stream and owns an independent cache. Marketdata and risk remain single-active in the reference deployment: simply increasing replicas would duplicate upstream subscriptions or lose local order-book state after a rebalance. Before scaling risk consumers by market partition, add a compacted state changelog or durable state restoration, then use consumer lag and CPU as scaling signals.

### What would you improve first?

1. Rebalance-safe order-book state restoration with a compacted changelog.
2. Fixed-point decimal arithmetic.
3. End-to-end latency, DLQ, and Kafka-lag dashboards.
4. Replay load test with measured p95/p99.
5. OIDC authentication and per-user paper portfolios.

## Demo order

1. Start `make demo` and show real-time risk state.
2. Submit a valid paper order.
3. Change the notional above the limit and show rejection reasons.
4. Explain how stale data changes severity to critical.
5. Open Grafana and point to latency / severity metrics.
6. Walk through `risk/engine.go`, one test, and the Kubernetes security context.

## Avoid these interview mistakes

- claiming production-grade throughput without a benchmark
- implying the project sends real orders
- presenting float arithmetic as suitable for final settlement
- saying Kafka guarantees exactly-once business semantics automatically
- describing AI as the control plane for financial risk
