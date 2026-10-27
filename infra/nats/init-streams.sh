#!/bin/sh
# Creates/updates the JetStream streams defined in ./streams/*.json (D-049).
# Runs as the one-shot `nats-init` compose service on every `up`; idempotent:
# missing stream -> `stream add`, existing stream -> `stream edit --force`.
# Services never create streams, they only consume/publish. Edit the JSON, re-run nats-init.
set -eu

SERVER="${NATS_URL:-nats://nats:4222}"
DIR="$(dirname "$0")/streams"

for f in "$DIR"/*.json; do
  name="$(jq -r '.name' "$f")"
  if nats --server "$SERVER" stream info "$name" >/dev/null 2>&1; then
    nats --server "$SERVER" stream edit "$name" --config "$f" --force >/dev/null
    echo "stream $name: updated"
  else
    nats --server "$SERVER" stream add "$name" --config "$f" >/dev/null
    echo "stream $name: created"
  fi
done
