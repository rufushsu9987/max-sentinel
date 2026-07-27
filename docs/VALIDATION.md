# Validation Status

This file separates checks already executed in the artifact-generation environment from checks that must be run on a normal development machine before publishing the repository.

## Executed successfully

- dependency-free Go tests for `internal/orderbook`, `internal/risk`, `internal/streamguard`, `internal/runtimeid`, and `cmd/backtest`
- MAX payload normalizer unit tests using the source files directly
- `go vet` for dependency-free packages
- YAML and JSON parsing for Compose, Kubernetes, Prometheus, Grafana, and OpenAPI assets
- relative Markdown link validation
- JavaScript syntax validation with `node --check`
- shell syntax validation with `bash -n`

## Not executed in this environment

The current sandbox cannot resolve `proxy.golang.org` and does not include Docker or Terraform. Therefore the following checks remain mandatory before the repository is made public:

```bash
go mod tidy
go mod verify
go test ./... -race -coverprofile=coverage.out
make validate

docker compose -f deploy/docker-compose.yml config
docker compose -f deploy/docker-compose.yml up --build

terraform -chdir=infra/terraform fmt -check
terraform -chdir=infra/terraform init -backend=false
terraform -chdir=infra/terraform validate
```

## Evidence to save for the application

- green GitHub Actions run
- dashboard and Grafana screenshots
- a 60–90 second demo recording
- load-test command, fixture version, machine specification, duration, and raw report
- only measured p50/p95/p99, throughput, error rate, lag, CPU, and memory numbers
