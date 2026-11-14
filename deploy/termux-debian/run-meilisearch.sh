#!/bin/bash
set -eu

mkdir -p /root/.local/share/meilisearch
export MEILI_ENV="${MEILI_ENV:-development}"
export MEILI_DB_PATH="${MEILI_DB_PATH:-/root/.local/share/meilisearch/data.ms}"
export MEILI_HTTP_ADDR="${MEILI_HTTP_ADDR:-127.0.0.1:7700}"
export MEILI_NO_ANALYTICS=true

exec /usr/local/bin/meilisearch
