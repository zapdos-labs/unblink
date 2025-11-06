
import { BsGithub } from 'solid-icons/bs';
import { FaSolidChevronDown } from 'solid-icons/fa';
import { FiBell, FiClock, FiFilm, FiGrid, FiMonitor, FiSearch, FiSettings } from 'solid-icons/fi';
import { createMemo, createSignal, For, onMount, Show } from 'solid-js';
import logoSVG from '~/assets/logo.svg';
import AddCameraButton from './AddCameraButton';
import { cameras, camerasLoading, fetchCameras, setTab, tab, type Camera } from './shared';

function MediaGroup(props: { group: { label: string; cameras: Camera[] } }) {
    const [isOpen, setIsOpen] = createSignal(true);

    return (
        <div class="mx-2 select-none">
            <div
                onClick={() => setIsOpen((o) => !o)}
                class="cursor-pointer flex items-center px-1 rounded-lg py-2  hover:bg-neutral-800 text-neutral-500 group"
            >
                <div class="ml-2 mr-2 group-hover:text-white">
                    <FaSolidChevronDown
                        data-open={isOpen()}
                        class="w-4 h-4 data-[open=true]:-rotate-180 transition-transform"
                    />
                </div>

                <div class="font-semibold ml-1 group-hover:text-white">{props.group.label}</div>

                <div class="flex-1" />

                <div
                    onClick={(e) => {
                        e.stopPropagation();
                        setTab({
                            type: 'view',
                            medias: props.group.cameras.map(c => ({ stream_id: c.id })),
                        });
                    }}
                    class="p-1.5 rounded hover:bg-neu-700 hover:text-white opacity-0 group-hover:opacity-100 transition-all">
                    <FiMonitor class="w-4 h-4" />
                </div>
            </div>
            <div
                data-open={isOpen()}
                class="mt-1 pl-0.5 ml-5 data-[open=false]:max-h-0 overflow-hidden transition-all duration-200 max-h-[1000px]"
            >
                <For each={props.group.cameras}>
                    {(camera) => {
                        const viewedMedias = () => {
                            const _tab = tab();
                            if (_tab.type === 'view') {
                                return _tab.medias;
                            }
                            return [];
                        };
                        const isViewed = () => viewedMedias().some(m => m.stream_id === camera.id && !m.file_name);
                        const onlyViewed = () => viewedMedias().length === 1 && isViewed();
                        return (
                            <div
                                onClick={() => {
                                    setTab({
                                        type: 'view',
                                        medias: [{
                                            stream_id: camera.id,
                                        }],
                                    });
                                }}
                                data-viewed={onlyViewed()}
                                class="cursor-pointer px-3 py-2 mx-2 space-x-3 rounded-lg hover:bg-neutral-800 flex items-center text-neutral-400 hover:text-white data-[viewed=true]:bg-neu-800 data-[viewed=true]:text-white"
                            >
                                <div class="text-sm line-clamp-1 break-all">{camera.name}</div>
                            </div>
                        );
                    }}
                </For>
            </div>
        </div>
    );
}

export default function SideBar() {

    onMount(fetchCameras);

    const cameraGroups = createMemo(() => {
        const groups = new Map<string, Camera[]>();
        const unlabeled: Camera[] = [];

        for (const camera of cameras()) {
            if (!camera.labels || camera.labels.length === 0) {
                unlabeled.push(camera);
            } else {
                for (const label of camera.labels) {
                    if (!groups.has(label)) {
                        groups.set(label, []);
                    }
                    groups.get(label)!.push(camera);
                }
            }
        }

        const labeledGroups = Array.from(groups.entries())
            .map(([label, cameras]) => ({ label, cameras }))
            .sort((a, b) => a.label.localeCompare(b.label));

        const result: { label: string; cameras: Camera[] }[] = [];
        if (unlabeled.length > 0) {
            result.push({ label: 'Unlabeled', cameras: unlabeled });
        }

        return result.concat(labeledGroups);
    });

    const tabs = [
        {
            id: 'home',
            name: 'Home',
            icon: FiGrid,
        },
        {
            id: 'search',
            name: 'Search',
            icon: FiSearch,
        },
        {
            id: 'moments',
            name: 'Moments',
            icon: FiFilm,
        },
        {
            id: 'history',
            name: 'History',
            icon: FiClock,
        },
    ];

    return <div class="w-80 h-screen pl-2 py-2">
        <div class="bg-neu-900 h-full rounded-2xl border border-neu-800 flex flex-col drop-shadow-2xl">

            {/* Head */}
            <div class="mt-4 flex items-center space-x-3 mx-4 mb-8">
                <img src={logoSVG} class="w-18 h-18" />
                <div class="flex-1 font-nunito font-medium text-white text-3xl mt-2 leading-none">
                    Unblink
                </div>
            </div>


            <div class="mx-4 space-y-1 mb-4">
                <For each={tabs}>
                    {_tab => <button
                        onClick={() => setTab({
                            type: _tab.id as 'home' | 'search' | 'moments' | 'history'
                        })}
                        data-active={
                            tab().type === _tab.id
                        }
                        class="w-full flex items-center space-x-3 px-4 py-2 rounded-xl text-neu-400 hover:bg-neu-800 data-[active=true]:bg-neu-800 data-[active=true]:text-white">
                        <_tab.icon class="w-4 h-4" />
                        <div>{_tab.name}</div>

                    </button>}
                </For>
            </div>


            <div class="mx-4 mb-4">
                <AddCameraButton />
            </div>

            <div class="flex-1 space-y-2 overflow-y-auto">
                <div class="flex items-center space-x-1 mx-4">
                    <div class="flex items-center space-x-2 ">
                        <div class="font-sm font-medium text-neu-500">Cameras</div>
                    </div>
                </div>
                <Show when={!camerasLoading()} fallback={
                    <div class="text-sm text-neu-500 p-4">Loading cameras...</div>
                }>
                    <Show when={cameras().length > 0} fallback={
                        <div class="text-sm text-neu-500 p-4">No cameras available</div>
                    }>
                        <div class="space-y-1">
                            <For each={cameraGroups()}>
                                {(group) => <MediaGroup group={group} />}
                            </For>
                        </div>
                    </Show>
                </Show>
            </div>



            <div class="flex-none mx-4 py-4 space-y-4">
                <div
                    onClick={() => window.open('https://github.com/tri2820/unblink', '_blank')}
                    class="flex items-center transition hover:text-white text-neu-500 hover:cursor-pointer">
                    <BsGithub class="w-5 h-5" />
                    <div class="ml-2 ">GitHub</div>
                </div>
                <div onClick={() => setTab({
                    type: 'settings'
                })} class="flex items-center transition hover:text-white text-neu-500 hover:cursor-pointer">
                    <FiSettings class="w-5 h-5" />
                    <div class="ml-2 ">Settings</div>
                </div>

            </div>

        </div>
    </div>
}