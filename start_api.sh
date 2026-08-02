#!/bin/bash

export REDIS_URL="redis://:8f6dc2418040a6b36d60ffdc519ff85b32f3027f8d487517eed23669b93b2250@localhost:6379/0"
export ARCHIVES_DIR=./archives
export APP_PUBLIC_URL="${APP_PUBLIC_URL:-http://localhost:1080}"
export REPLAY_PUBLIC_URL="${REPLAY_PUBLIC_URL:-http://localhost:1081}"


go build -o api.out ./cmd/api/main.go

./api.out
