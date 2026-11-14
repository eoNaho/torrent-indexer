#!/bin/bash
set -euo pipefail
umask 077
ENV_FILE="${TINDEXER_ENV:-/etc/torrent-indexer.env}"
if [[ ! -r "$ENV_FILE" ]]; then
  echo "Cannot read environment file: $ENV_FILE" >&2
  exit 1
fi
set -a
. "$ENV_FILE"
set +a
exec /usr/local/bin/torrent-indexer
