FROM golang:1.25-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download


COPY . .

RUN go build -o api ./cmd/api/main.go

FROM gcr.io/distroless/static-debian13:latest

USER 1000:1000

COPY --from=builder /app/api /api

ENTRYPOINT ["/api"]