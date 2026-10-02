FROM golang:1.27-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN --mount=type=cache,id=archiver-go-mod,target=/go/pkg/mod \
    go mod download

COPY . ./
RUN --mount=type=cache,id=archiver-go-mod,target=/go/pkg/mod \
    --mount=type=cache,id=archiver-go-build,target=/root/.cache/go-build \
    go build -o worker ./cmd/worker

FROM webrecorder/browsertrix-crawler:1.15.0

COPY --from=builder /app/worker /usr/local/bin/worker
COPY --from=builder /app/internal/crawler/drivers/anubis.mjs /app/drivers/anubis.mjs

RUN chmod +x /usr/local/bin/worker

ENTRYPOINT ["/usr/local/bin/worker"]
