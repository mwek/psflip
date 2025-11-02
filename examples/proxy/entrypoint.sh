#!/bin/sh

CONTAINER_NAME="${CONTAINER_NAME:-$$}"
echo '{"status": "ok", "container": "'"${CONTAINER_NAME}"'"}' > /content/health
exec python3 -m http.server -d /content 8080
