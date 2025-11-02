import { createResizeObserver } from "@solid-primitives/resize-observer";
import { FaSolidSpinner } from "solid-icons/fa";
import { createEffect, createSignal, onCleanup, onMount, Show } from "solid-js";
import { newMessage } from "./video/connection";
import type { DetectionObject, ServerToClientMessage } from "~/shared";

class MjpegPlayer {
    private canvas: HTMLCanvasElement;
    private ctx: CanvasRenderingContext2D;
    private img: HTMLImageElement | null = null;
    private detectionObjects: DetectionObject[] = [];
    private animationFrameId = 0;
    private isDestroyed = false;
    private sourceWidth = 0;
    private sourceHeight = 0;
    private onDrawingStateChange: (isDrawing: boolean) => void;

    constructor(
        canvas: HTMLCanvasElement,
        onDrawingStateChange: (isDrawing: boolean) => void
    ) {
        this.canvas = canvas;
        this.ctx = canvas.getContext('2d')!;
        this.onDrawingStateChange = onDrawingStateChange;
        this.startRenderLoop();
    }

    public handleMessage(message: ServerToClientMessage): void {
        if (this.isDestroyed) return;

        if (message.type === 'object_detection') {
            this.detectionObjects = message.objects;
            return;
        }

        if (message.type === 'codec') {
            this.sourceWidth = message.width;
            this.sourceHeight = message.height;
            return;
        }

        if (message.type === 'frame' && message.data) {
            const blob = new Blob([message.data as any], { type: 'image/jpeg' });
            const url = URL.createObjectURL(blob);

            const img = new Image();
            img.onload = () => {
                if (this.img) {
                    URL.revokeObjectURL(this.img.src);
                }
                this.img = img;
                if (!this.sourceWidth || !this.sourceHeight) {
                    this.sourceWidth = img.naturalWidth;
                    this.sourceHeight = img.naturalHeight;
                }
                this.onDrawingStateChange(true);
            };
            img.src = url;
        }
    }

    private renderLoop = () => {
        if (this.isDestroyed) return;

        this.ctx.clearRect(0, 0, this.canvas.width, this.canvas.height);

        if (this.img) {
            const geom = this.calculateRenderGeometry();
            const r = 16;

            const x = geom.offsetX;
            const y = geom.offsetY;
            const w = geom.renderWidth;
            const h = geom.renderHeight;

            this.ctx.save();
            this.ctx.beginPath();
            this.ctx.moveTo(x + r, y);
            this.ctx.lineTo(x + w - r, y);
            this.ctx.quadraticCurveTo(x + w, y, x + w, y + r);
            this.ctx.lineTo(x + w, y + h - r);
            this.ctx.quadraticCurveTo(x + w, y + h, x + w - r, y + h);
            this.ctx.lineTo(x + r, y + h);
            this.ctx.quadraticCurveTo(x, y + h, x, y + h - r);
            this.ctx.lineTo(x, y + r);
            this.ctx.quadraticCurveTo(x, y, x + r, y);
            this.ctx.closePath();
            this.ctx.clip();

            this.ctx.drawImage(
                this.img,
                geom.offsetX,
                geom.offsetY,
                geom.renderWidth,
                geom.renderHeight
            );
            this.ctx.restore();

            this.drawDetections(geom);
        }

        this.animationFrameId = requestAnimationFrame(this.renderLoop);
    }

    private calculateRenderGeometry() {
        const canvasWidth = this.canvas.width;
        const canvasHeight = this.canvas.height;
        const videoWidth = this.sourceWidth || this.img?.naturalWidth || 1;
        const videoHeight = this.sourceHeight || this.img?.naturalHeight || 1;

        const canvasAspect = canvasWidth / canvasHeight;
        const videoAspect = videoWidth / videoHeight;

        let renderWidth: number, renderHeight: number, offsetX: number, offsetY: number;

        if (canvasAspect > videoAspect) {
            renderHeight = canvasHeight;
            renderWidth = renderHeight * videoAspect;
            offsetX = (canvasWidth - renderWidth) / 2;
            offsetY = 0;
        } else {
            renderWidth = canvasWidth;
            renderHeight = renderWidth / videoAspect;
            offsetX = 0;
            offsetY = (canvasHeight - renderHeight) / 2;
        }

        return { renderWidth, renderHeight, offsetX, offsetY };
    }

    private drawDetections(geom: { renderWidth: number; renderHeight: number; offsetX: number; offsetY: number }) {
        if (this.detectionObjects.length === 0 || !this.sourceWidth || !this.sourceHeight) return;

        const videoWidth = this.sourceWidth;
        const videoHeight = this.sourceHeight;
        const MODEL_INPUT_WIDTH = 640;
        const MODEL_INPUT_HEIGHT = 640;

        const videoAspect = videoWidth / videoHeight;
        const modelAspect = MODEL_INPUT_WIDTH / MODEL_INPUT_HEIGHT;

        let modelScale: number, modelOffsetX = 0, modelOffsetY = 0;
        if (videoAspect > modelAspect) {
            modelScale = MODEL_INPUT_WIDTH / videoWidth;
            modelOffsetY = (MODEL_INPUT_HEIGHT - videoHeight * modelScale) / 2;
        } else {
            modelScale = MODEL_INPUT_HEIGHT / videoHeight;
            modelOffsetX = (MODEL_INPUT_WIDTH - videoWidth * modelScale) / 2;
        }

        const imageWidthInModel = videoWidth * modelScale;
        const canvasRenderScale = geom.renderWidth / imageWidthInModel;

        this.ctx.save();
        this.ctx.strokeStyle = '#FF0000';
        this.ctx.lineWidth = 2;
        this.ctx.font = '14px Arial';
        this.ctx.textBaseline = 'bottom';

        this.detectionObjects.forEach(obj => {
            const { x1, y1, x2, y2 } = obj.box;

            const scaledX = geom.offsetX + (x1 - modelOffsetX) * canvasRenderScale;
            const scaledY = geom.offsetY + (y1 - modelOffsetY) * canvasRenderScale;
            const scaledWidth = (x2 - x1) * canvasRenderScale;
            const scaledHeight = (y2 - y1) * canvasRenderScale;

            this.ctx.strokeRect(
                Math.floor(scaledX),
                Math.floor(scaledY),
                Math.floor(scaledWidth),
                Math.floor(scaledHeight)
            );

            const text = `${obj.label} (${(obj.confidence * 100).toFixed(1)}%)`;
            const textMetrics = this.ctx.measureText(text);
            const textWidth = textMetrics.width;
            const textHeight = 15;
            const labelY = scaledY > textHeight + 5 ? scaledY : scaledY + scaledHeight + textHeight;

            this.ctx.fillStyle = 'rgba(0, 0, 0, 0.7)';
            this.ctx.fillRect(
                Math.floor(scaledX),
                Math.floor(labelY - textHeight),
                Math.ceil(textWidth + 10),
                Math.ceil(textHeight + 2)
            );
            this.ctx.fillStyle = '#FFFFFF';
            this.ctx.fillText(text, scaledX + 5, labelY);
        });

        this.ctx.restore();
    }

    private startRenderLoop() {
        this.animationFrameId = requestAnimationFrame(this.renderLoop);
    }

    public updateCanvasSize(width: number, height: number) {
        if (this.isDestroyed) return;
        this.canvas.width = width;
        this.canvas.height = height;
    }

    public destroy() {
        this.isDestroyed = true;
        cancelAnimationFrame(this.animationFrameId);
        if (this.img) {
            URL.revokeObjectURL(this.img.src);
            this.img = null;
        }
        console.log("MjpegPlayer destroyed.");
    }
}

export default function createCanvasVideo(props: { stream_id: string }) {
    const [canvasRef, setCanvasRef] = createSignal<HTMLCanvasElement>();
    const [containerRef, setContainerRef] = createSignal<HTMLDivElement>();
    const [isDrawing, setIsDrawing] = createSignal(false);

    let player: MjpegPlayer | null = null;

    createEffect(() => {
        const canvas = canvasRef();
        if (canvas && !player) {
            player = new MjpegPlayer(canvas, setIsDrawing);
        }
    });

    createEffect(() => {
        const message = newMessage();
        if (message?.stream_id === props.stream_id) {
            player?.handleMessage(message);
        }
    });

    createEffect(() => {
        const container = containerRef();
        if (!container) return;
        createResizeObserver(container, ({ width, height }) => {
            if (width > 0 && height > 0) {
                player?.updateCanvasSize(width, height);
            }
        });
    });

    onMount(() => setIsDrawing(false));

    onCleanup(() => {
        player?.destroy();
        player = null;
    });

    return (
        <div ref={setContainerRef}
            style={{ position: "relative", width: "100%", height: "100%" }}
        >

            <canvas
                ref={setCanvasRef}
                style={{
                    position: "absolute",
                    top: 0,
                    left: 0,
                    width: "100%",
                    height: "100%",
                    display: "block"
                }}
            />
            <Show when={!isDrawing()}>
                <div style={{
                    position: "absolute",
                    top: "50%",
                    left: "50%",
                    transform: "translate(-50%, -50%)",
                    color: "white"
                }}>
                    <div class="animate-spin">
                        <FaSolidSpinner size={48} />
                    </div>
                </div>
            </Show>
        </div>
    );
}