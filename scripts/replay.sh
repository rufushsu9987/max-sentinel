#!/usr/bin/env sh
set -eu

INPUT="${1:-testdata/max_ws_messages.jsonl}"
HOST_BROKERS="${KAFKA_BROKERS:-localhost:19092}"
COMPOSE_FILE="${COMPOSE_FILE:-deploy/docker-compose.yml}"

if [ ! -f "$INPUT" ]; then
  echo "replay input not found: $INPUT" >&2
  exit 1
fi

if command -v rpk >/dev/null 2>&1; then
  BROKERS="$HOST_BROKERS"
  rpk_exec() {
    rpk "$@"
  }
elif command -v docker >/dev/null 2>&1 && docker compose -f "$COMPOSE_FILE" ps redpanda >/dev/null 2>&1; then
  BROKERS="${KAFKA_CONTAINER_BROKERS:-localhost:9092}"
  rpk_exec() {
    docker compose -f "$COMPOSE_FILE" exec -T redpanda rpk "$@"
  }
else
  echo "rpk was not found and the Redpanda Compose service is not available." >&2
  echo "Install rpk or start the stack with: docker compose -f $COMPOSE_FILE up -d redpanda" >&2
  exit 1
fi

TOPIC="${MARKET_EVENTS_TOPIC:-market.events.v1}"
rpk_exec topic create "$TOPIC" --brokers "$BROKERS" >/dev/null 2>&1 || true

COUNT=0
while IFS= read -r event; do
  [ -z "$event" ] && continue
  printf '%s\n' "$event" | rpk_exec topic produce "$TOPIC" --brokers "$BROKERS" >/dev/null
  COUNT=$((COUNT + 1))
done < "$INPUT"

printf 'Replayed %s events into %s via %s.\n' "$COUNT" "$TOPIC" "$BROKERS"
