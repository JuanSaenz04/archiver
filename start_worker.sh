#!/bin/bash

export REDIS_URL="redis://:8f6dc2418040a6b36d60ffdc519ff85b32f3027f8d487517eed23669b93b2250@172.18.0.2:6379/0"

go build -o worker.out ./cmd/worker/main.go

./worker.out