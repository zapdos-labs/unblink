import { logger } from "../logger";

import type { ServerToWorkerObjectDetectionMessage, WorkerObjectDetectionToServerMessage } from "../../shared";
import { downloadModelFile, loadObjectDetectionModel, warmup } from "../inference/local";
import { buffersFromPaths, detect_objects } from "../inference/object_detection";
import { encode } from "cbor-x";

declare var self: Worker;

logger.info("Worker 'object detection' started");

const INFERENCE_INTERVAL_MS = 10; // How often to run the inference loop
const MAX_IMAGES_TO_PROCESS = 30;

// Map to hold the latest image for each stream
const latestImageMap = new Map<string, ServerToWorkerObjectDetectionMessage>();

function sendMessage(msg: WorkerObjectDetectionToServerMessage) {
    console.log("Sending message to server:", msg);
    const worker_msg = encode(msg);
    self.postMessage(worker_msg, [worker_msg.buffer]);
}

// Download models
await downloadModelFile();
const objectDetectionModel = await loadObjectDetectionModel();

async function continuousInferenceLoop() {
    if (latestImageMap.size === 0) {
        return;
    }

    // Process images from the map (get up to MAX_IMAGES_TO_PROCESS)
    const imagesToProcess: ServerToWorkerObjectDetectionMessage[] = [];
    let count = 0;

    for (const [stream_id, message] of latestImageMap.entries()) {
        if (count >= MAX_IMAGES_TO_PROCESS) {
            break;
        }
        imagesToProcess.push(message);
        count++;
    }

    // Clear the map for the next interval
    latestImageMap.clear();

    try {
        // logger.info(`Processing ${imagesToProcess.length} unique images for object detection`);

        const paths = imagesToProcess.map(item => item.path);
        const buffers = await buffersFromPaths(paths);
        const detections = await detect_objects(buffers, objectDetectionModel);

        imagesToProcess.forEach((message, i) => {
            const detectionResults = detections[i] || [];
            const result = {
                type: 'object_detection' as const,
                stream_id: message.stream_id,
                objects: detectionResults.map(obj => {
                    const x1 = parseFloat(obj.box[0] || '0');
                    const y1 = parseFloat(obj.box[1] || '0');
                    const x2 = parseFloat(obj.box[2] || '0');
                    const y2 = parseFloat(obj.box[3] || '0');

                    return {
                        label: obj.label || 'unknown',
                        confidence: parseFloat(obj.score || '0'),
                        box: {
                            x1,
                            y1,
                            x2,
                            y2
                        }
                    };
                })
            };
            sendMessage(result);
        });
    } catch (error) {
        logger.error({ error }, 'Error during continuous inference');
    }
}

self.addEventListener("message", (event) => {
    const msg: ServerToWorkerObjectDetectionMessage = event.data;
    console.log("Object Detection Worker received message:", msg);
    if (msg.type === 'frame_file') {
        // Store the latest message for the stream
        latestImageMap.set(msg.stream_id, msg);
    }
});

async function startInferenceLoop() {
    while (true) {
        await continuousInferenceLoop();
        await new Promise(resolve => setTimeout(resolve, INFERENCE_INTERVAL_MS));
    }
}

startInferenceLoop();
