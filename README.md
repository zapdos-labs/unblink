<p align="center">
<img width="300" src="assets/logo.svg">
</p>


[![Status](https://github.com/tri2820/unblink/actions/workflows/release.yml/badge.svg)](https://github.com/tri2820/unblink/actions)
[![GitHub Stars](https://img.shields.io/github/stars/tri2820/unblink?style=flat)](https://github.com/tri2820/unblink/stargazers)
[![Discord](https://img.shields.io/badge/Discord-Join%20Server-5865F2?style=flat&logo=discord&logoColor=white)](https://discord.gg/YMAjT8A6e2)

# Unblink

Unblink is a camera monitoring application that runs AI vision models on your camera streams in real-time. Key features:

- 👀 Object detection with D-FINE
- 🤓 Contextual understanding with SmolVLM2
- 🔎 Intelligent search across your video feeds.

Live demo: [https://app.zapdoslabs.com](https://app.zapdoslabs.com)

## Getting Started

## Installation

### Method 1: Directly from source


#### Prerequisites

- [Bun](https://bun.sh) runtime installed on your system


```bash
# Clone the repository
git clone https://github.com/tri2820/unblink
cd unblink

# Install dependencies
bun install

# Start the application
bun dev

# Or you can build the binary and run that (faster load time & more efficient in production)
# bun build.ts
# ./dist/unblink-linux

```

### Method 2: Binary executable

1. Go to Unblink [release page](https://github.com/tri2820/unblink/releases/latest)
2. Download the file suitable for your operating system
3. Double click and run

📌 This method is experimetal, if you have any problem please file a bug report

The application will start and be accessible at `http://localhost:3000` (or can be configured via `PORT` env variable).

## Screenshots

### Setup & Camera Configuration
Add and configure multiple camera sources with support for RTSP, MJPEG, and other protocols.

![Setup Screen](/assets/screenshots/setup.png)

### Multi-View Dashboard
Monitor all your cameras simultaneously with real-time feeds and status indicators.

![Multi-View](/assets/screenshots/multiview.png)

### Vision-Language Model (VLM) Interaction
Ask natural language questions about what's happening in your camera feeds using SmolVLM2.

![VLM Interface](/assets/screenshots/vlm.png)

### Semantic Search
Search through captured frames using natural language queries. Find specific events, objects, or scenes across your camera history.

![Search Interface](/assets/screenshots/search.png)

### Object Detection
Real-time object detection and tracking powered by D-FINE model.

![Object Detection](/assets/screenshots/object_detection.png)

### Alerts
Send detections & description via webhooks and other communication channels
![Alerts](/assets/screenshots/alerts.png)

### Authentication
Securely gate your instance with role-based access
![Alerts](/assets/screenshots/authentication.png)

## AI Models
- **D-FINE**: State-of-the-art object detection for identifying and tracking objects in real-time
- **SmolVLM2**: Vision-language model for understanding context and answering questions about camera feeds

## Q&A

**Why is my CPU usage so high?**

D-FINE object detection is resource-intensive. If you experience performance issues, you could consider disabling object detection from the Settings page. I would add some optimization to this soon.

**Where is the code to run the models?** 

The model inference code is in a separate repository at [https://github.com/tri2820/unblink-engine](https://github.com/tri2820/unblink-engine). This separation allows the AI models to run with GPU acceleration in Python, while keeping the app lightweight.

Currently I have the engine hosted on my GPU server that you can use (the client app automatically connects to it), so hosting the engine yourself is optional. If you need to, you can mofidy `ENGINE_URL` env var and the client app will connect there instead.

## Project Status

| Feature | Status | Notes |
|---------|--------|-------|
| Multi-camera Dashboard | ✅ Stable | Tested with several camera protocols |
| D-FINE Object Detection | ✅ Stable | |
| SmolVLM2 Integration | ✅ Stable | |
| Semantic Search | 🤔 WIP | Need to rework UI |
| Video Recording & Playback | 🤔 WIP | Need to implement controls (help needed) |
| Binary Distribution | 🤔 WIP | Testing... |
| Motion Detection | 🚧 Coming Soon |  |
| ONVIF Support | 🚧 Coming Soon |  |
| Webhook | ✅ Stable |  |
| Automation | 🚧 Coming Soon |  |

**Legend**: ✅ Stable | 🤔 WIP | 🚧 Coming Soon

## Contributing

Contributions are welcome! Please feel free to submit issues, feature requests, or pull requests.

## Acknowledgments

The tech that does the major lifting of the stream ingestion work is done by `seydx` through the amazing [node-av](https://github.com/seydx/node-av) library. 

---

Built with ❤️ and ramen. Star Unblink to save it for later. 🌟
