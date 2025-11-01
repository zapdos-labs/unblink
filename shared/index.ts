export type FrameMessage = {
    type: "frame_file";
    path: string;
}

export type StreamMessage = {
    type: "codec";
    mimeType: string | null;
    videoCodec: string | null;
    audioCodec: string | null;
    codecString: string | null;
    fullCodec: string;
    width: number;
    height: number;
    hasAudio: boolean;
} | {
    type: 'frame';
    data: Uint8Array;
} | FrameMessage;

export type MediaUnit = {
    id: string;
    description: string | null;
    at_time: Date;
    embedding: number[] | null;
    media_id: string;
    path: string;
}

export type Subscription = {
    session_id: string;
    stream_ids: string[];
}

export type ClientToServerMessage = {
    type: 'set_subscription';
    subscription: Subscription | undefined | null;
}

export type WorkerToServerMessage = WorkerObjectDetectionToServerMessage | WorkerStreamToServerMessage
export type ServerToClientMessage = WorkerToServerMessage & {
    session_id?: string;
}

export type WorkerStreamToServerMessage = (StreamMessage & { stream_id: string }) | {
    type: "error";
    stream_id: string;
} | {
    type: "restarting";
    stream_id: string;
} | {
    type: 'starting';
    stream_id: string;
}

export type ServerToWorkerStreamMessage = WorkerStreamToServerMessage | {
    type: 'start_stream',
    stream_id: string,
    uri: string,
} | {
    type: 'stop_stream',
    stream_id: string,
}

export type ServerToWorkerObjectDetectionMessage = {
    stream_id: string;
} & FrameMessage

export type DetectionObject = {
    label: string;
    confidence: number;
    box: {
        x1: number;
        y1: number;
        x2: number;
        y2: number;
    }
}

export type WorkerObjectDetectionToServerMessage = {
    type: 'object_detection';
    stream_id: string;
    objects: DetectionObject[];
}