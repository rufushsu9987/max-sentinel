# Résumé and GitHub Copy

## Project title

**MAX Sentinel — Go-based Real-Time Crypto Market Risk Platform**

## Chinese résumé bullets

- 以 Go 建置事件驅動微服務平台，串接 MAX Exchange 公開 WebSocket，正規化 order book、trade 與 ticker 事件，並以 Kafka-compatible Redpanda 解耦即時資料擷取、風控計算與 API gateway。
- 實作 L2 order book 重建與 deterministic risk engine，計算 spread、book imbalance、weighted mid-price、realized volatility、price shock 與 stale-feed 狀態，並以單元測試驗證異常市場分類。
- 建置 REST、SSE 與 gRPC 介面及 paper-order guardrails，依資料新鮮度、風險等級、價格偏離與 NT$ 名目金額上限進行可稽核的接受／拒絕決策。
- 完成 PostgreSQL idempotent audit persistence、Kafka retry／DLQ、Prometheus／Grafana observability、Docker Compose、Kubernetes Gateway HPA／PDB／NetworkPolicy，以及 AWS EKS／MSK with TLS/SASL-SCRAM／RDS Terraform 參考架構。

## English résumé bullets

- Built an event-driven Go microservices platform that consumes MAX Exchange public WebSocket data, normalizes order-book/trade/ticker events, and decouples ingestion, risk processing, and API delivery through Kafka-compatible Redpanda.
- Implemented deterministic L2 order-book reconstruction and risk signals including spread, depth imbalance, weighted mid-price, realized volatility, price shock, and stale-feed detection with focused unit tests.
- Developed REST, SSE, and gRPC interfaces plus auditable paper-order guardrails based on feed freshness, severity, limit-price deviation, and configurable notional limits.
- Provisioned an operational stack with idempotent PostgreSQL audit records, Kafka retry/DLQ handling, Prometheus/Grafana, Docker Compose, Gateway autoscaling and Kubernetes security policies, plus an AWS EKS/MSK with TLS/SASL-SCRAM/RDS Terraform reference architecture.

## Metrics version — fill only after measurement

Replace brackets with reproducible results from `docs/PERFORMANCE_PLAN.md`:

- Processed **[X events/sec]** with **[Y ms p95]** risk-computation latency and **[Z ms p99]** end-to-end snapshot latency under a **[N-minute]** replay test.
- Achieved **[availability/freshness result]** during reconnect and broker-failure tests, with duplicate events safely handled through idempotent event IDs.
- Maintained **[test coverage]%** domain test coverage across order-book and risk-classification logic.

Do not invent these values.

## GitHub description

> Real-time MAX Exchange market-data and deterministic risk-monitoring platform built with Go, Kafka/Redpanda, gRPC, PostgreSQL, Prometheus, Kubernetes, and Terraform. Includes paper-order guardrails and replayable backtesting; never places real orders.

## Suggested repository topics

`golang`, `microservices`, `cryptocurrency`, `exchange`, `websocket`, `kafka`, `grpc`, `postgresql`, `prometheus`, `grafana`, `kubernetes`, `terraform`, `devsecops`
