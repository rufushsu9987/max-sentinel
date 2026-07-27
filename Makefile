.PHONY: help bootstrap fmt test test-pure build run demo backtest replay compose-up compose-down migrate lint validate zip

help:
	@echo "MAX Sentinel commands"
	@echo "  make bootstrap    Download and verify Go dependencies"
	@echo "  make fmt          Format Go code"
	@echo "  make test         Run all Go tests (requires dependencies)"
	@echo "  make test-pure    Run dependency-free domain tests"
	@echo "  make build        Build all services"
	@echo "  make compose-up   Start local infrastructure and services"
	@echo "  make compose-down Stop local stack"
	@echo "  make demo         Run gateway in deterministic demo mode"
	@echo "  make backtest     Run the sample mean-reversion simulation"
	@echo "  make replay       Replay normalized fixture events into Redpanda"
	@echo "  make validate     Validate YAML, JSON, JavaScript, and shell syntax"

bootstrap:
	go mod tidy
	go mod verify

fmt:
	gofmt -w $$(find cmd internal -name '*.go')

test:
	go test ./... -race -coverprofile=coverage.out

test-pure:
	go test ./internal/orderbook ./internal/risk ./internal/streamguard ./internal/runtimeid ./cmd/backtest

build:
	mkdir -p bin
	go build -o bin/marketdata ./cmd/marketdata
	go build -o bin/risk ./cmd/risk
	go build -o bin/gateway ./cmd/gateway
	go build -o bin/backtest ./cmd/backtest

run:
	docker compose -f deploy/docker-compose.yml up --build

compose-up:
	docker compose -f deploy/docker-compose.yml up --build -d

compose-down:
	docker compose -f deploy/docker-compose.yml down -v

migrate:
	psql "$${DATABASE_URL}" -f migrations/001_init.sql

demo:
	APP_ENV=demo GATEWAY_HTTP_ADDR=:8080 go run ./cmd/gateway

backtest:
	go run ./cmd/backtest -input testdata/btctwd_ticks.csv

replay:
	./scripts/replay.sh

validate:
	python3 scripts/validate_assets.py
	node --check web/app.js
	bash -n scripts/replay.sh

zip:
	cd .. && zip -r max-sentinel.zip max-sentinel -x 'max-sentinel/.git/*' 'max-sentinel/bin/*'
