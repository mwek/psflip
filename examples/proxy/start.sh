#!/bin/sh

docker build --tag psflip-proxy .

export PORT="${1:-8080}"

exec docker run \
    --init \
    --sig-proxy \
    --rm \
    --env CONTAINER_NAME \
    -p "${PORT}:8080" \
    --name "${CONTAINER_NAME:-proxy}" \
    --tmpfs "/content:mode=1777" \
    psflip-proxy
