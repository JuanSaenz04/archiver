FROM golang:1.25-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN go build -o worker ./cmd/worker/main.go

FROM webrecorder/browsertrix-crawler:latest

COPY --from=builder /app/worker /usr/local/bin/worker

RUN chmod +x /usr/local/bin/worker

ENTRYPOINT ["/usr/local/bin/worker"]