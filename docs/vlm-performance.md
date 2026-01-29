# VLM Performance Benchmarks

## Peak Throughput Analysis

| Concurrency | Throughput (req/s) | Avg Latency |
|-------------|-------------------|-------------|
| 5           | 2.59              | 1.8s        |
| 10          | 4.74              | 1.9s        |
| 15          | 6.18              | 2.1s        |
| 20          | 6.31              | 2.3s        |
| 25          | 6.84              | 2.5s        |
| 30          | 9.15              | 2.6s        |
| 40          | 11.99             | 2.8s        |
| 50          | 12.76             | 2.7s        |
| 55          | 15.08             | 2.9s        |
| **65**      | **16.75**         | **3.0s**    |
| 70          | 15.99             | 3.1s        |
| 80          | 15.42             | 3.1s        |

**Peak Performance: 16.75 req/s at 65 concurrent requests**

## System Specifications
- Model: `Qwen/Qwen3-VL-4B-Instruct`
- Endpoint: `https://vllm.unblink.net/v1`
- Image Size: 640x480
- Test Date: 2026-01-29
