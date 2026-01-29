#!/bin/bash

# VLM Throughput Test Script
# Usage: ./test_vlm.sh [options]

set -e

cd "$(dirname "$0")"

# Default values (can be overridden by command line args)
URL="${URL:-https://vllm.unblink.net/v1}"
MODEL="${MODEL:-Qwen/Qwen3-VL-4B-Instruct}"
CONCURRENCY="${CONCURRENCY:-10}"
REQUESTS="${REQUESTS:-100}"

echo "Building test script..."
go build -o /tmp/test_vlm_throughput ./test_vlm_throughput.go

echo ""
echo "Running throughput test..."
echo ""

/tmp/test_vlm_throughput \
  -url="$URL" \
  -model="$MODEL" \
  -concurrency="$CONCURRENCY" \
  -requests="$REQUESTS" \
  "$@"

# Clean up
rm /tmp/test_vlm_throughput
