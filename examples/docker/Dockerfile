FROM alpine:latest

COPY --chmod=755 entrypoint.sh /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
HEALTHCHECK --interval=1s --timeout=1s \
  CMD test -f /tmp/healthy
