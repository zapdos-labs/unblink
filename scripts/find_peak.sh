#!/bin/bash
# Find peak VLM concurrency

set -e

API_KEY="sk-2b686e2bef839b3f6c29bb999488f3c0001d7d5a27a22691"

echo "Finding peak VLM concurrency..."
echo ""

for concurrency in 5 10 15 20 25 30; do
    echo "=========================================="
    echo "Testing concurrency: $concurrency"
    echo "=========================================="
    ./test_vlm.sh -apikey="$API_KEY" -concurrency=$concurrency -requests=30 2>&1 | grep -E "(Concurrency|Throughput|Avg)"
    echo ""
    sleep 2
done

echo "Done! Check results above to find optimal concurrency."
