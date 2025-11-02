#!/bin/sh

docker build --tag psflip-example .

exec docker run \
    --init \
    --sig-proxy \
    --rm \
    --env CONTAINER_NAME \
    --name "${CONTAINER_NAME:-example}" \
    psflip-example
