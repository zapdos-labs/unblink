import fs from "fs/promises";
import type { AVPixelFormat, AVSampleFormat, Frame, Packet, Stream } from "node-av";
import {
    AV_CODEC_ID_AAC,
    AV_CODEC_ID_MJPEG,
    AV_PIX_FMT_BGR24,
    AV_PIX_FMT_BGR4,
    AV_PIX_FMT_BGR4_BYTE,
    AV_PIX_FMT_BGR8,
    AV_PIX_FMT_GRAY8,
    AV_PIX_FMT_MONOBLACK,
    AV_PIX_FMT_MONOWHITE,
    AV_PIX_FMT_PAL8,
    AV_PIX_FMT_RGB24,
    AV_PIX_FMT_RGB4,
    AV_PIX_FMT_RGB4_BYTE,
    AV_PIX_FMT_RGB8,
    AV_PIX_FMT_UYVY422,
    AV_PIX_FMT_UYYVYY411,
    AV_PIX_FMT_YUV410P,
    AV_PIX_FMT_YUV411P,
    AV_PIX_FMT_YUV420P,
    AV_PIX_FMT_YUV422P,
    AV_PIX_FMT_YUV444P,
    AV_PIX_FMT_YUVJ420P,
    AV_PIX_FMT_YUVJ422P,
    AV_PIX_FMT_YUVJ444P,
    AV_PIX_FMT_YUYV422,
    AV_SAMPLE_FMT_FLTP,
    avGetCodecStringHls,
    avGetMimeTypeDash,
    Decoder,
    Encoder,
    FF_ENCODER_AAC,
    FF_ENCODER_MJPEG,
    FilterAPI,
    FilterPreset,
    MediaInput,
    MediaOutput,
} from "node-av";
import path from "path";
import { FRAMES_DIR, RECORDINGS_DIR } from "~/backend/appdir";
import { logger as _logger } from "~/backend/logger";
import type { StreamMessage } from "~/shared";

const logger = _logger.child({ worker: 'stream' });
const MAX_SIZE = 720;
const OUTPUT_ROLLING_INTERVAL_MS = 3600 * 1000; // 1 hour
// const OUTPUT_ROLLING_INTERVAL_MS = 60 * 1000; // 1 min for testing

function getCodecs(
    videoStream: Stream,
    audioStream: Stream | undefined,
): StreamMessage {
    const videoCodecString = avGetCodecStringHls(videoStream.codecpar);
    const audioCodecString = audioStream
        ? avGetCodecStringHls(audioStream.codecpar)
        : null;

    const codecStrings = audioCodecString
        ? `${videoCodecString},${audioCodecString}`
        : videoCodecString;

    const mimeType = avGetMimeTypeDash(videoStream.codecpar);
    const fullCodec = `${mimeType}; codecs="${codecStrings}"`;

    const codecs: StreamMessage = {
        type: "codec",
        mimeType,
        videoCodec: videoCodecString,
        audioCodec: audioCodecString,
        codecString: codecStrings,
        fullCodec,
        width: videoStream.codecpar.width,
        height: videoStream.codecpar.height,
        hasAudio: !!audioStream,
    };

    return codecs;
}

async function raceWithTimeout<T>(
    promise: Promise<IteratorResult<T, any>>,
    abortSignal: AbortSignal,
    ms: number
): Promise<IteratorResult<T, any> | undefined> {
    let timeoutId: NodeJS.Timeout | undefined = undefined;

    const timeoutPromise = new Promise<never>((_, reject) => {
        timeoutId = setTimeout(() => {
            logger.warn('Timeout receiving packets');
            reject(new Error('Timeout receiving packets'));
        }, ms);
    });

    const abort_promise = new Promise<never>((_, reject) => {
        if (abortSignal.aborted) {
            return reject(new DOMException('Aborted', 'AbortError'));
        }
        abortSignal.addEventListener('abort', () => {
            reject(new DOMException('Aborted', 'AbortError'));
        }, { once: true });
    });

    try {
        const result = await Promise.race([promise, timeoutPromise, abort_promise]);
        return result as IteratorResult<T, any>;
    } finally {
        clearTimeout(timeoutId);
    }
}

function shouldSkipTranscode(videoStream: Stream): boolean {
    const SUPPORTED_FORMATS: (AVPixelFormat | AVSampleFormat)[] = [
        AV_PIX_FMT_YUV420P,
        AV_PIX_FMT_YUYV422,
        AV_PIX_FMT_RGB24,
        AV_PIX_FMT_BGR24,
        AV_PIX_FMT_YUV422P,
        AV_PIX_FMT_YUV444P,
        AV_PIX_FMT_YUV410P,
        AV_PIX_FMT_YUV411P,
        AV_PIX_FMT_GRAY8,
        AV_PIX_FMT_MONOWHITE,
        AV_PIX_FMT_MONOBLACK,
        AV_PIX_FMT_PAL8,
        AV_PIX_FMT_YUVJ420P,
        AV_PIX_FMT_YUVJ422P,
        AV_PIX_FMT_YUVJ444P,
        AV_PIX_FMT_UYVY422,
        AV_PIX_FMT_UYYVYY411,
        AV_PIX_FMT_BGR8,
        AV_PIX_FMT_BGR4,
        AV_PIX_FMT_BGR4_BYTE,
        AV_PIX_FMT_RGB8,
        AV_PIX_FMT_RGB4,
        AV_PIX_FMT_RGB4_BYTE
    ];

    const isMjpeg = videoStream.codecpar.codecId === AV_CODEC_ID_MJPEG;
    const hasCompatibleFormat = SUPPORTED_FORMATS.includes(videoStream.codecpar.format);

    return isMjpeg && hasCompatibleFormat;
}

type OutputFileObject = {
    from: Date;
    mediaOutput: MediaOutput;
    videoFileOutputIndex: number;
    path: string;
};
class OutputFile {
    static async create(streamId: string, videoStream: Stream): Promise<OutputFileObject> {
        const from = new Date();
        const dir = `${RECORDINGS_DIR}/${streamId}`;
        await fs.mkdir(dir, { recursive: true });
        const filePath = path.join(dir, `from_${from.getTime()}_ms.mkv`);
        const mediaOutput = await MediaOutput.open(filePath, {
            format: 'matroska',  // Always use Matroska/MKV
        });
        const videoFileOutputIndex = mediaOutput.addStream(videoStream);
        return { from, mediaOutput, videoFileOutputIndex, path: filePath };
    }

    static async close(obj: OutputFileObject) {
        await obj.mediaOutput.close();
        // Rename to have closed_at timestamp
        const to = new Date();
        const newName = `from_${obj.from.getTime()}_ms_to_${to.getTime()}_ms.mkv`;
        const newPath = path.join(path.dirname(obj.path), newName);
        await fs.rename(obj.path, newPath);
        logger.info({ old: obj.path, new: newPath }, "Closed output file");
    }
}


export async function streamMedia(stream: {
    id: string;
    uri: string;
}, onMessage: (msg: StreamMessage) => void, signal: AbortSignal) {
    logger.info({ uri: stream.uri }, 'Starting streamMedia for');

    logger.info(`Opening media input: ${stream.uri}`);
    await using input = await MediaInput.open(stream.uri, {
        options: stream.uri.toLowerCase().startsWith("rtsp://")
            ? { rtsp_transport: "tcp" }
            : undefined,
    });

    const videoStream = input.video();
    if (!videoStream) {
        throw new Error("No video stream found");
    }

    logger.info(`Done opening media input`);

    let audioPipeline: {
        decoder: Decoder;
        encoder: Encoder;
        filter: FilterAPI;
    } | undefined = undefined;

    const audioStream = input.audio();
    if (audioStream && audioStream.codecpar.codecId !== AV_CODEC_ID_AAC) {
        const decoder = await Decoder.create(audioStream);

        const targetSampleRate = 48000;
        const filterChain = FilterPreset.chain()
            .aformat(AV_SAMPLE_FMT_FLTP, targetSampleRate, "stereo")
            .asetnsamples(1024)
            .build();

        const filter = FilterAPI.create(filterChain, {
            timeBase: audioStream.timeBase,
        });

        const encoder = await Encoder.create(FF_ENCODER_AAC, {
            timeBase: { num: 1, den: targetSampleRate },
        });

        audioPipeline = { encoder, decoder, filter };
    }

    const videoDecoder = await Decoder.create(videoStream);

    const longer_side = Math.max(
        videoStream.codecpar.width,
        videoStream.codecpar.height,
    );
    const scale = MAX_SIZE / longer_side;
    const newWidth = Math.round(videoStream.codecpar.width * scale);
    const newHeight = Math.round(videoStream.codecpar.height * scale);

    logger.info({ newWidth, newHeight }, "Scaling video to:");

    const filterChain = FilterPreset.chain()
        .format(AV_PIX_FMT_YUVJ420P)
        .scale(newWidth, newHeight, {
            flags: "lanczos",
        })
        .build();
    const videoFilter = FilterAPI.create(filterChain, {
        timeBase: videoStream.timeBase,
    });

    logger.info({
        format: videoStream.codecpar.format,
        codecId: videoStream.codecpar.codecId,
    }, "Input video:");

    const skipTranscode = shouldSkipTranscode(videoStream);

    logger.info({
        skipTranscode,
        format: videoStream.codecpar.format,
        codecId: videoStream.codecpar.codecId,
        AV_CODEC_ID_MJPEG,
    }, "Transcode decision:");

    const codecItem = getCodecs(videoStream, audioStream);
    logger.info(codecItem, "Initialized stream codecs");
    onMessage(codecItem);

    using videoEncoder = await Encoder.create(FF_ENCODER_MJPEG, {
        timeBase: videoStream.timeBase,
        frameRate: videoStream.avgFrameRate,
        bitrate: '2M',
        options: {
            strict: 'experimental',
        },
    });

    async function sendFrameMessage(packet: Packet) {
        if (!packet.data) return;
        const frame_msg: StreamMessage = {
            type: "frame",
            data: packet.data,
        };
        onMessage(frame_msg);
    }



    let last_save_time = 0;
    async function saveFrameForObjectDetection(encodedData: Uint8Array) {
        const now = Date.now();
        if (now - last_save_time < 1000) return;
        last_save_time = now;

        const path = `${FRAMES_DIR}/${stream.id}.jpg`;
        await Bun.write(path, encodedData);

        const frame_file_msg: StreamMessage = {
            type: "frame_file",
            path,
        };
        onMessage(frame_file_msg);
    }

    async function processPacket(packet: Packet, decodedFrame: Frame) {
        let filteredFrame: Frame | null = null;

        try {
            // Filter once
            if (videoFilter) {
                filteredFrame = await videoFilter.process(decodedFrame);
                if (!filteredFrame) return;
            }

            const frameToUse = filteredFrame || decodedFrame;

            // Send frame for streaming
            if (skipTranscode) {
                await sendFrameMessage(packet);
                // For skipTranscode, we still need to encode for object detection
                using encodedPacket = await videoEncoder.encode(frameToUse);
                if (encodedPacket?.data) {
                    await saveFrameForObjectDetection(encodedPacket.data);
                }
            } else {
                // Encode once and reuse for both streaming and object detection
                using encodedPacket = await videoEncoder.encode(frameToUse);
                if (encodedPacket?.data) {
                    await sendFrameMessage(encodedPacket);
                    await saveFrameForObjectDetection(encodedPacket.data);
                }
            }
        } finally {
            // Always free the filtered frame
            filteredFrame?.free();
        }
    }

    const packets = input.packets();
    let last_send_time = 0;

    logger.info("Entering main streaming loop");

    let output = null;

    while (true) {
        const res = await raceWithTimeout(packets.next(), signal, 10000);

        if (!res || res.done) {
            logger.info("Stream ended or timed out");
            break;
        }

        const packet = res.value;

        // Initialize rolling file output on first video packet
        // Or if it has been 1 hour since last output creation
        const now = Date.now();
        if (output === null || (now - output.from.getTime() >= OUTPUT_ROLLING_INTERVAL_MS)) {
            if (output) {
                logger.info("Closing previous file output due to rolling interval");
                await OutputFile.close(output);
            }

            output = await OutputFile.create(stream.id, videoStream);
            logger.info({ path: output.path }, "Created new rolling output file");
        }

        if (packet.streamIndex === videoStream.index) {
            // Write to file output
            await output.mediaOutput.writePacket(packet, output.videoFileOutputIndex);

            const decodedFrame = await videoDecoder.decode(packet);

            if (!decodedFrame) {
                packet.free();
                continue;
            }

            const now = Date.now();
            if (now - last_send_time < 1000 / 30) {
                packet.free();
                decodedFrame.free();
                continue;
            }
            last_send_time = now;

            try {
                await processPacket(packet, decodedFrame);
            } catch (error) {
                logger.error({ error: (error as Error).message }, "Error processing packet");
            } finally {
                packet.free();
                decodedFrame.free();
            }
        } else {
            packet.free();
        }
    }

    logger.info("Streaming loop ended");
}