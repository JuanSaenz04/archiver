FROM golang:1.26-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN --mount=type=cache,id=archiver-go-mod,target=/go/pkg/mod \
    go mod download

COPY . ./
RUN --mount=type=cache,id=archiver-go-mod,target=/go/pkg/mod \
    --mount=type=cache,id=archiver-go-build,target=/root/.cache/go-build \
    go build -o worker ./cmd/worker/main.go

FROM webrecorder/browsertrix-crawler:1.14.1

COPY --from=builder /app/worker /usr/local/bin/worker

RUN chmod +x /usr/local/bin/worker

ENTRYPOINT ["/usr/local/bin/worker"]
