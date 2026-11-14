#!/bin/bash
set -euo pipefail
umask 077
ENV_FILE="${MEDIA_AGENT_ENV:-/etc/media-agent.env}"
if [[ ! -r "$ENV_FILE" ]]; then
  echo "Cannot read environment file: $ENV_FILE" >&2
  exit 1
fi
set -a
. "$ENV_FILE"
set +a
cd "$(dirname "$0")"
exec python3 media_agent.py
