FROM alpine:latest

RUN apk add --no-cache \
    curl \
    python3 \
    && true

COPY --chmod=755 entrypoint.sh /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
HEALTHCHECK --interval=1s --timeout=1s \
  CMD curl http://localhost:8080/health
