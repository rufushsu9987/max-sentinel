# ADR 001: Use an event stream between ingestion and risk services

- Status: Accepted
- Date: 2026-07-27

## Context

Market data must support multiple downstream consumers, independent scaling, replay, and failure recovery.

## Decision

Publish normalized events to a Kafka-compatible stream keyed by market. Use direct gRPC only for synchronous queries.

## Consequences

Positive: replayability, loose coupling, partition ordering, easier fan-out.

Negative: eventual consistency, broker operations, schema governance, consumer-lag monitoring.
