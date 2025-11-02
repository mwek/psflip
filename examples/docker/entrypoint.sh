#!/bin/sh

CONTAINER_NAME="${CONTAINER_NAME:-$$}"
INIT_TIME="${INIT_TIME:-5}"
current=0
while true; do
  echo "$(date -uIseconds) hello from ${CONTAINER_NAME}"
  current=$((current + 1))
  if [ "${current}" -eq "${INIT_TIME}" ]; then
    touch /tmp/healthy
    echo "$(date -uIseconds) marking ${CONTAINER_NAME} as healthy"
  fi
  sleep 1;
done
