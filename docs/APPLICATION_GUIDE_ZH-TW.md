# MaiCoin 投遞指南：MAX Sentinel

## 1. 建議投遞職缺順序

1. **Micro Service Software Engineer**：本專案最完整對應 Go、microservices、gRPC、relational database、Kubernetes、high scalability、low latency、Prometheus 與 Grafana。
2. **Trading Software Engineer**：以 MAX WebSocket、order book、event streaming、backtest、latency 與 market microstructure 作為第二條敘事線。
3. **Blockchain Engineer**：可補上 wallet address watcher 或 EVM deposit-confirmation service 後再投。

你的主軸不要寫成「做了一個幣價 Dashboard」，而要寫成：

> 我建立了一個交易所等級的事件驅動市場資料與 deterministic risk platform，聚焦即時資料一致性、故障降級、可觀測性、稽核與安全邊界。

## 2. 投遞前必做清單

- 將 repository module path 與容器 image path 換成自己的 GitHub 帳號。
- 執行 `go mod tidy`，提交產生的 `go.sum`。
- 執行 `make test`、`make validate`，保留 CI 成功畫面。
- 以 Docker Compose 啟動完整環境，拍攝 Dashboard 與 Grafana 截圖。
- 用 replay data 跑一次效能測試，只把可重現的 p95/p99、throughput、error rate 寫入履歷。
- 錄製 60–90 秒操作影片，展示行情流入、風險升級、stale feed 與 paper order 被拒絕。
- README 首頁放一張架構圖、一張 UI 截圖、一張 Grafana 截圖。
- 在 GitHub Release 附上 SBOM、container digest 與操作影片連結。

## 3. 本機啟動

### 立即展示 UI

```bash
cp .env.example .env
go mod tidy
make demo
```

開啟 `http://localhost:8080`。

### 完整事件流

```bash
docker compose -f deploy/docker-compose.yml up --build
```

主要入口：

- Operations Dashboard：`http://localhost:8080`
- Prometheus：`http://localhost:9091`
- Grafana：`http://localhost:3000`

### 回測與重播

```bash
make backtest
docker compose -f deploy/docker-compose.yml stop marketdata
make replay
```

## 4. 五分鐘面試 Demo 腳本

### 0:00–0:40｜問題與邊界

- 交易所不只需要看價格，還需要知道行情是否可信、風險是否惡化、下游是否可以安全採取行動。
- 專案只讀公開 market data，絕不送出真實訂單；paper order 只展示 deterministic guardrails。

### 0:40–1:40｜資料流

- `marketdata-service` 連接 MAX WebSocket，處理 reconnect、ping/pong 與 compact message normalization。
- 標準化事件送入 Kafka-compatible Redpanda，將 ingestion、risk processing 與 delivery 解耦。
- raw payload 被保留，方便 replay、debug 與 evidence audit。

### 1:40–2:40｜風控與一致性

- L2 order book 以 sequence-aware update 重建。
- 計算 spread、depth imbalance、weighted mid-price、realized volatility、price shock 與 feed age。
- stale feed 直接升為 critical；重複 event ID 不會重複寫入或造成二次決策。

### 2:40–3:30｜API 與產品畫面

- REST 提供 snapshot 與 paper-order decision。
- SSE 對瀏覽器推送即時狀態；gRPC 提供服務間風險評估。
- 示範正常訂單，以及因 stale、critical、oversize 或價格偏離而被拒絕的訂單。

### 3:30–4:20｜營運與安全

- Prometheus/Grafana 監控 spread、volatility、processing latency 與 decision count；marketdata readiness 會反映 MAX WebSocket 是否實際連線。
- Kubernetes 包含 Gateway HPA/PDB、readiness/liveness、default-deny NetworkPolicy；行情與風控服務在尚未實作 leader election／state restoration 前維持單一 active replica，避免假擴充造成重複或不完整 order book。
- AWS reference architecture 使用 private EKS、encrypted RDS、MSK TLS + SASL/SCRAM 與 KMS。

### 4:20–5:00｜取捨與下一步

- 風控核心保持 deterministic；LLM 只能做 evidence-backed incident explanation，不能核准交易。
- 下一步是 replay load test、consumer lag SLO、chaos testing，以及 authenticated private API integration with a dedicated least-privilege subaccount。

## 5. 面試常見追問

### 為什麼用 Kafka，而不是服務直接呼叫？

市場資料具有多個獨立消費者。Event log 能保留 partition 內順序、支援 replay、隔離下游故障並讓新消費者加入；代價是 eventual consistency、consumer lag、重複投遞與 rebalance state restoration，因此必須有 event ID、idempotency、lag monitoring 與明確的 state ownership。

### 如何避免行情斷線後還讓訂單通過？

風險服務不只在新事件抵達時運算，還會週期性重新評估 `feed_age`。超過 threshold 即標為 stale/critical，Gateway 在做 paper-order decision 前再次 freshen snapshot，形成 defense in depth。

### Kafka consumer 處理失敗怎麼辦？

暫時性錯誤（包含 production-required 模式下的 PostgreSQL audit write failure）會在同一 offset 重試，不會直接 commit 跳過；JSON 等永久性解析錯誤則保留原始 key、value、headers、partition、offset 與錯誤原因，寫入 `<source-topic>.dlq` 後才 commit。正式環境還應補上 bounded retry、DLQ lag/volume alerting、人工或自動重放流程，以及 idempotent storage。

### 為什麼 LLM 不直接判斷是否可交易？

交易核准需要可重現、可測試、低延遲且可稽核。LLM 輸出具有非決定性，因此只能根據 immutable metrics 與 versioned runbook 解釋事故，不能覆寫 deterministic policy。

### 哪些效能數字可以放履歷？

只放在固定硬體、資料集、事件速率與測試期間下可重跑的數字。至少包含 throughput、risk calculation p95/p99、end-to-end latency p95/p99、consumer lag、error rate 與 CPU/memory；不要使用估算值。

## 6. 履歷與 GitHub 呈現方式

履歷選 3–4 點，優先順序：

1. Go event-driven microservices 與 MAX WebSocket。
2. L2 order book、deterministic risk、stale-feed protection。
3. Kafka idempotency、PostgreSQL audit、REST/SSE/gRPC。
4. Prometheus/Grafana、Kubernetes、Terraform、DevSecOps。

完整文案位於 `RESUME_BULLETS.md`，面試故事位於 `INTERVIEW_GUIDE.md`。量測前不要填入任何效能數字。

## 7. 差異化加分功能

核心版本完成後，建議只挑一項深化：

- **可靠性方向**：chaos test + SLO/error budget + runbook。
- **交易方向**：order-book replay、slippage model、maker/taker fee simulation。
- **資安方向**：signed images、SBOM、admission policy、secret rotation。
- **AI 方向**：RAG incident explainer，回答必須引用 metrics、event IDs 與 runbook version。

對 MaiCoin 投遞時，可靠性與交易方向應優先於 AI 功能；AI 是差異化層，不是核心風控替代品。
