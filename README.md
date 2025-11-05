# Unblink

Unblink is a camera monitoring application that runs AI vision models on your camera streams in real-time. It features object detection with D-FINE, contextual understanding with SmolVLM2, and intelligent search capabilities across your video feeds.

## Getting Started

### Prerequisites

- [Bun](https://bun.sh) runtime installed on your system

### Installation

Currently, Unblink runs directly from source (binary distribution coming soon):

```bash
# Clone the repository
git clone https://github.com/tri2820/unblink
cd unblink

# Install dependencies
bun install

# Start the application
bun dev
```

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

## AI Models
- **D-FINE**: State-of-the-art object detection for identifying and tracking objects in real-time
- **SmolVLM2**: Vision-language model for understanding context and answering questions about camera feeds

## Q&A

**Why does it consume so many resources?**
D-FINE object detection is resource-intensive. If you experience performance issues, consider disabling object detection from the Settings page.

**Where is the code to run the models?** 
The model inference code is in a separate repository at [https://github.com/tri2820/unblink-engine](https://github.com/tri2820/unblink-engine). This separation allows the AI models to run with GPU acceleration in Python, while keeping the app lightweight.

## Project Status

| Feature | Status | Notes |
|---------|--------|-------|
| Multi-camera Dashboard | ✅ Added | Tested with several camera protocols |
| D-FINE Object Detection | ✅ Added | |
| SmolVLM2 Integration | ✅ Added | |
| Semantic Search | 🤔 WIP | Need to rework UI |
| Video Recording & Playback | 🤔 WIP | Need to implement controls (help needed) |
| Binary Distribution | 🤔 WIP | Need to implement Github Action that runs build.ts (help needed) |
| Motion Detection | 🚧 Coming Soon |  |
| ONVIF Support | 🚧 Coming Soon |  |

**Legend**: ✅ Added | 🤔 WIP | 🚧 Coming Soon

## Contributing

Contributions are welcome! Please feel free to submit issues, feature requests, or pull requests.

## License

[Your License Here]

## Acknowledgments

The tech that does the major lifting of the camera stream ingestion is done by `seydx` through the amazing `node-av` library. 

---

Built with ❤️ and ramen