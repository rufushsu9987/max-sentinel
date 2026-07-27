# ADR 002: Keep order guardrails deterministic

- Status: Accepted
- Date: 2026-07-27

## Context

Risk decisions must be predictable, low latency, testable, and explainable during incidents.

## Decision

Implement threshold and policy evaluation in regular code. Permit AI only to summarize evidence and retrieve runbooks.

## Consequences

Positive: reproducibility, auditability, straightforward testing, bounded latency.

Negative: rules need explicit maintenance and may be less adaptive than learned models.
