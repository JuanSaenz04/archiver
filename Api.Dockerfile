# Stage 1: Build Frontend
FROM node:22-alpine AS frontend-builder
WORKDIR /app/front
COPY front/package.json front/pnpm-lock.yaml ./
RUN corepack enable && pnpm install --frozen-lockfile
COPY front/ .
RUN pnpm build

# Stage 2: Build Backend
FROM golang:1.25-alpine AS backend-builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Copy compiled frontend to the location expected by Go embed
COPY --from=frontend-builder /app/front/dist ./internal/api/dist

RUN go build -o api ./cmd/api/main.go

# Stage 3: Final Image
FROM gcr.io/distroless/static-debian13:latest

USER 1000:1000

COPY --from=backend-builder /app/api /api

ENTRYPOINT ["/api"]