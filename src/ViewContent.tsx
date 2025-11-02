import { Show, createEffect, createSignal, onCleanup, untrack, type JSX } from "solid-js";
import { setSubscription, viewedMedias } from "./shared";
import createCanvasVideo from "./CanvasVideo";

export default function ViewContent() {
    const _streamId = () => viewedMedias()[0];
    const [streamId, setStreamId] = createSignal<string>();
    const [canvasVideo, setCanvasVideo] = createSignal<JSX.Element>();

    // Handle subscriptions
    createEffect(() => {
        const id = streamId();
        if (id) {
            console.log('Subscribing to stream:', id);
            const session_id = crypto.randomUUID();

            setSubscription({
                session_id,
                stream_ids: [id],
            });
        } else {
            setSubscription();
        }
    });

    createEffect(() => {
        const id = _streamId();
        const currentId = untrack(streamId);
        if (id !== currentId) {
            console.log('Updating streamId to:', id);
            setStreamId(id);
        }
    })

    createEffect(() => {
        const id = streamId();
        if (!id) {
            setCanvasVideo(undefined);
            return;
        }
        console.log('streamId changed to:', id);
        const _canvasVideo = createCanvasVideo({ stream_id: id });
        setCanvasVideo(_canvasVideo);
    })


    // Cleanup subscriptions on unmount
    onCleanup(() => {
        console.log('ViewContent unmounting, clearing subscriptions');
        setSubscription();
    });

    return (
        <div class="flex flex-col h-screen">
            <div class="flex-1 mr-2 my-2">
                <Show
                    when={streamId()}
                    fallback={<div>No camera selected</div>}
                >
                    {canvasVideo()}
                </Show>
            </div>
        </div>
    );
}