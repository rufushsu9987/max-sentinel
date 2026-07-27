# Security Model

## Scope

The default project consumes only public market data and never sends orders to MAX. This materially reduces credential and financial risk while still allowing exchange-oriented engineering to be demonstrated.

## Threat model

| Threat | Primary control |
|---|---|
| API key theft | no private key in core flow; future keys stored in a managed secret service only |
| forged market events | TLS, broker authentication, topic ACLs, producer identity, payload validation |
| replay / duplicate events | immutable event ID and idempotent database constraint |
| stale or incomplete book | sequence monitoring, freshness thresholds, reconnect, and wait-for-fresh-snapshot recovery |
| abuse of the public paper-order demo | strict input validation, request-size limit, deterministic rejection; add authentication/RBAC/rate limiting before multi-user exposure |
| SSRF / arbitrary upstream | fixed MAX endpoint allowlist; no user-controlled fetch URL |
| container breakout | non-root, read-only filesystem, dropped capabilities, RuntimeDefault seccomp |
| lateral movement | default-deny NetworkPolicy with per-workload egress ports and narrowly scoped service ingress |
| dependency compromise | pinned direct versions, Dependabot, and CodeQL; add SBOM generation, image scanning, and signing before a production release |
| secret overexposure | separate Kafka and database Secrets; only the risk workload receives `DATABASE_URL` |
| sensitive log leakage | structured allowlisted fields; redact credentials and request headers |

## Private API extension checklist

Before adding authenticated trading endpoints:

1. Create an exchange subaccount with the minimum permissions.
2. Disable withdrawal permission for application keys.
3. Encrypt credentials using KMS-backed secrets and rotate them.
4. Enforce IP allowlists where supported.
5. Separate signal generation, risk approval, and execution services.
6. Require an idempotency key and a durable order state machine.
7. Add kill switches per account, market, strategy, and total notional.
8. Reconcile every internal order against exchange order and trade history.
9. Store immutable audit evidence and operator identity.
10. Conduct threat modeling, SAST, dependency, container, and penetration testing.

## Paper-order controls already implemented

The portfolio demo is intentionally unauthenticated and should not be exposed as a multi-user service without an identity and rate-limit layer.

- 1 MiB request body limit
- strict JSON fields
- finite positive quantity validation
- configurable maximum notional in NT$
- stale-feed rejection
- critical-risk rejection
- limit-price deviation rejection
- decision reason and generated order ID

## Explicit non-goals

- custody or wallet key management
- real trade execution
- profit prediction
- autonomous AI approval of financial actions
