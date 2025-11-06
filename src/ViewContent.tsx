import { formatDistance } from "date-fns";
import { FaSolidChevronLeft, FaSolidChevronRight } from "solid-icons/fa";
import { For, Show, createEffect, createSignal, onCleanup } from "solid-js";
import ArkSwitch from "./ark/ArkSwitch";
import CanvasVideo from "./CanvasVideo";
import { cameras, relevantAgentCards, setSubscription, settings, tab } from "./shared";

const GAP_SIZE = '8px';

const chunk = <T,>(arr: T[]): T[][] => {
    const n = arr.length;
    const size = n === 0 ? 1 : Math.ceil(Math.sqrt(n));
    if (size <= 0) {
        return arr.length ? [arr] : [];
    }
    return Array.from({ length: Math.ceil(arr.length / size) }, (v, i) =>
        arr.slice(i * size, i * size + size)
    );
}



function useAgentBar() {
    const [showAgentBar, setShowAgentBar] = createSignal(true);

    return {
        showAgentBar,
        setShowAgentBar,
        Toggle: () => <button
            onClick={() => setShowAgentBar(prev => !prev)}
            class="px-2 py-1.5 text-xs font-medium text-white bg-neu-800 rounded-lg hover:bg-neu-850 border border-neu-750 focus:outline-none flex items-center space-x-1">
            <Show when={showAgentBar()} fallback={<FaSolidChevronLeft class="w-4 h-4 " />}>
                <FaSolidChevronRight class="w-4 h-4 " />
            </Show>
            <div>Agent</div>
        </button>
    }
}

export default function ViewContent() {
    const [showDetections, setShowDetections] = createSignal(true);
    const agentBar = useAgentBar();


    const viewedMedias = () => {
        const t = tab();
        return t.type === 'view' ? t.medias : [];
    }


    // Handle subscriptions
    createEffect(() => {
        const medias = viewedMedias();
        if (medias && medias.length > 0) {
            console.log('Subscribing to streams:', medias);
            const session_id = crypto.randomUUID();

            setSubscription({
                session_id,
                streams: medias.map(media => ({ id: media.stream_id, file_name: media.file_name })),
            });
        } else {
            setSubscription();
        }
    });

    const cols = () => {

        const n = viewedMedias().length;
        return n === 0 ? 1 : Math.ceil(Math.sqrt(n));
    }

    const rowsOfMedias = () => chunk(viewedMedias());


    // Cleanup subscriptions on unmount
    onCleanup(() => {
        console.log('ViewContent unmounting, clearing subscriptions');
        setSubscription();
    });

    const some_media_is_live = () => {
        return viewedMedias().some(media => !media.file_name);
    }

    return (
        <div class="flex items-start h-screen">
            <div class="flex-1 flex flex-col h-screen ">
                <div class="flex-1 mr-2 my-2">
                    <Show
                        when={rowsOfMedias().length > 0}
                        fallback={<div class="flex justify-center items-center h-full">No camera selected</div>}
                    >
                        <div class="h-full w-full flex flex-col space-y-2">
                            <div class="flex-none flex items-center space-x-6 py-2 px-4 bg-neu-900 rounded-2xl border border-neu-800 h-14">
                                <div class="flex-1 text-sm text-neu-400 line-clamp-1">Viewing {viewedMedias().length} streams</div>
                                <Show when={settings()['object_detection_enabled'] === 'true'}>
                                    <div>
                                        <ArkSwitch
                                            label="Show detection boxes"
                                            checked={showDetections}
                                            onCheckedChange={(e) => setShowDetections(e.checked)}
                                        />
                                    </div>
                                </Show>

                                <Show when={!agentBar.showAgentBar()}>
                                    <agentBar.Toggle />
                                </Show>
                            </div>
                            <div class="flex-1 flex flex-col" style={{ gap: GAP_SIZE }}>
                                <For each={rowsOfMedias()}>
                                    {(row, rowIndex) => (
                                        <div
                                            class="flex flex-1"
                                            style={{
                                                'justify-content': rowIndex() === rowsOfMedias().length - 1 && row.length < cols() ? 'center' : 'flex-start',
                                                gap: GAP_SIZE,
                                            }}
                                        >
                                            <For each={row}>
                                                {(media) => {
                                                    return <div style={{ width: `calc((100% - (${cols() - 1} * ${GAP_SIZE})) / ${cols()})`, height: '100%' }}>
                                                        <CanvasVideo stream_id={media.stream_id} file_name={media.file_name} showDetections={showDetections} />
                                                    </div>
                                                }}
                                            </For>
                                        </div>
                                    )}
                                </For>
                            </div>
                        </div>
                    </Show>
                </div>
            </div>

            <Show when={some_media_is_live()}>
                <div
                    data-show={agentBar.showAgentBar()}
                    class="flex-none data-[show=true]:w-xl w-0 h-screen transition-[width] duration-300 ease-in-out overflow-hidden  flex flex-col">
                    <div class="border-l border-neu-800 bg-neu-900 shadow-2xl rounded-2xl flex-1 mr-2 my-2 flex flex-col h-full overflow-hidden">
                        <div class="h-14 flex items-center p-2">
                            <agentBar.Toggle />
                        </div>

                        <Show when={agentBar.showAgentBar()}>
                            <div class="flex-1 p-2 overflow-y-auto space-y-4">
                                <Show when={relevantAgentCards().length > 0} fallback={<div class="text-neu-500">Waiting for VLM responses...</div>}>
                                    <For each={relevantAgentCards()}>
                                        {(card) => {
                                            const stream_name = () => {
                                                const camera = cameras().find(c => c.id === card.stream_id);
                                                return camera ? camera.name : 'Unknown Stream';
                                            }
                                            return <div class="animate-push-down p-4 bg-neu-850 rounded-2xl space-y-2">
                                                <div class="font-semibold">{stream_name()}</div>
                                                <div class="text-neu-400 text-sm">{formatDistance(card.created_at, Date.now(), {
                                                    addSuffix: true
                                                })}</div>
                                                <div>{card.content}</div>
                                            </div>
                                        }}
                                    </For>
                                </Show>
                            </div>
                        </Show>

                    </div>
                </div>
            </Show>
        </div>

    );
}