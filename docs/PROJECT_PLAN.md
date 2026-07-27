# Application Project Plan

## Recommended scope for the first public release

### Milestone 1 — core engineering story

- public MAX market-data ingestion
- order-book reconstruction and risk engine
- unit tests and replay fixtures
- live dashboard and paper-order guardrails
- Docker Compose and screenshots

### Milestone 2 — production operations story

- Prometheus/Grafana screenshots
- end-to-end load test and measured metrics
- Kubernetes deployment on a low-cost cluster
- CI, CodeQL, image scan, SBOM, signed image

### Milestone 3 — differentiating AI story

- evidence-backed incident explainer
- versioned runbook RAG
- evaluation dataset and hallucination metrics

## Suggested Git history

1. `feat: add MAX websocket protocol adapter`
2. `feat: implement deterministic L2 order book`
3. `feat: publish normalized market events`
4. `feat: add risk engine and severity rules`
5. `feat: expose snapshot and paper order APIs`
6. `feat: add realtime operations dashboard`
7. `ops: add prometheus grafana and compose stack`
8. `ops: add kubernetes and terraform reference deployment`
9. `test: add replay fixtures and performance plan`
10. `docs: add architecture security and interview guide`

## Portfolio deliverables

- GitHub repository with a concise English README
- 60–90 second screen recording
- architecture diagram
- Grafana dashboard screenshot
- one load-test chart with reproducible command
- one technical article explaining order-book consistency and stale-feed risk
