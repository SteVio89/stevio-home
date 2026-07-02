FROM golang:1.26-bookworm AS builder

WORKDIR /app

COPY . .
RUN go vet ./...
RUN go test ./...
RUN go build -o /server .

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates tzdata curl gosu \
    && rm -rf /var/lib/apt/lists/*

# Pre-create mount-point directories so bind mounts work correctly
RUN mkdir -p /assets/icons /assets/screenshots /apps /data

RUN groupadd -r -g 1001 stevio && useradd -r -u 1001 -g stevio stevio
RUN mkdir -p /home/stevio/.cache && chown -R stevio:stevio /home/stevio /assets /apps /data

COPY --from=builder /server /server
COPY entrypoint.sh /entrypoint.sh

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=5s --start-period=30s --retries=5 \
  CMD curl -f http://localhost:8080/healthz || exit 1

ENTRYPOINT ["/entrypoint.sh"]
CMD ["/server"]
