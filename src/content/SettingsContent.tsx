import { FiBell, FiCpu, FiUser } from "solid-icons/fi";
import { createEffect, createSignal, For, untrack, type Accessor, type Setter, type ValidComponent } from "solid-js";
import { Dynamic } from "solid-js/web";
import { fetchSettings, settings } from "~/src/shared";
import { toaster } from "../ark/ArkToast";
import LayoutContent from "./LayoutContent";
import { useAlertsSubTab } from "./settings/useAlertsSubTab";
import { useAuthSubTab } from "./settings/useAuthSubTab";
import { useMachineLearningSubTab } from "./settings/useMachineLearningSubTab";

export type UseSubTab = (props: {
    scratchpad: Accessor<Record<string, string>>,
    setScratchpad: Setter<Record<string, string>>,
}) => {
    comp: ValidComponent,
    keys: () => {
        name: string,
        normalize?: (value: string) => string,
        validate?: (value: string) => {
            type: 'success' | 'error',
            message?: string
        }
    }[]
}


export default function SettingsContent() {
    const [scratchpad, setScratchpad] = createSignal<Record<string, string>>(
        settings()
    );

    const subtabs = [
        {
            type: 'machine_learning',
            name: 'Machine Learning',
            icon: FiCpu,
            use: useMachineLearningSubTab,
        },
        {
            type: 'alerts',
            name: 'Alerts',
            icon: FiBell,
            use: useAlertsSubTab,
        },
        {
            type: 'authentication',
            name: 'Authentication',
            icon: FiUser,
            use: useAuthSubTab,
        }
    ];

    type SubTab = {
        type: 'machine_learning' | 'alerts' | 'authentication';
    }
    const [subtabId, setSubtabId] = createSignal<SubTab>({
        type: 'machine_learning'
    });

    const [subTab, setSubTab] = createSignal<ReturnType<UseSubTab>>();

    createEffect(() => {
        const id = subtabId();
        const s = subtabs.find(t => t.type === id.type);
        if (!s) {
            setSubTab(undefined);
            return;
        }

        const t = s.use?.({ scratchpad, setScratchpad });
        setSubTab(t);
    })


    const handleSaveSettings = async () => {
        const ust = untrack(subTab);
        if (!ust) return;
        const keys = ust.keys();
        const entries: { key: string, value: string }[] = [];

        for (const key of keys) {
            let value = scratchpad()[key.name];
            // TODO: This mean deletion
            if (value === undefined) continue;
            value = key.normalize ? key.normalize(value) : value;
            if (key.validate) {
                const validation = key.validate(value);
                if (validation.type === 'error') {
                    toaster.error({
                        title: 'Invalid Value',
                        description: validation.message || 'Please check your input values.',
                    });
                    continue;
                }
            }
            entries.push({ key: key.name, value });

        }

        toaster.promise(async () => {
            console.log("Saving settings:", entries);
            const resp = await fetch("/settings", {
                method: "PUT",
                headers: {
                    "Content-Type": "application/json",
                },
                body: JSON.stringify({ entries }),
            });
            if (!resp.ok) {
                throw new Error(`Failed to save settings: ${resp.statusText}`);
            }
            const data = await resp.json();

            if (!data.success) {
                throw new Error('Failed to save settings');
            }

            await fetchSettings(); // Refresh settings after saving
        }, {
            loading: {
                title: 'Saving...',
                description: 'Your settings are being saved.',
            },
            success: {
                title: 'Success!',
                description: 'Settings have been saved successfully.',
            },
            error: {
                title: 'Failed',
                description: 'There was an error saving your settings. Please try again.',
            },
        })
    };



    return <LayoutContent title="Settings"
        head_tail={<div class="flex-1 flex items-center">
            <div class="flex-1" />
            <button
                onClick={handleSaveSettings}
                class="btn-primary flex-none">
                Save Settings
            </button>
        </div>}
    >
        <div class="flex items-stretch h-full">
            <div class="w-xs flex-none p-4 border-r border-neu-800">
                <div class="space-y-1">
                    <For each={subtabs}>
                        {_tab => <button
                            onClick={() => setSubtabId({
                                type: _tab.type as SubTab['type']
                            })}
                            data-active={
                                subtabId().type === _tab.type
                            }
                            class="w-full flex items-center space-x-3 px-4 py-2 rounded-lg text-neu-400 hover:bg-violet-400/10 data-[active=true]:bg-violet-400/10 data-[active=true]:text-violet-400 hover:text-violet-400">
                            <_tab.icon class="w-4 h-4" />
                            <div>{_tab.name}</div>

                        </button>}
                    </For>
                </div>
            </div>
            <div class="flex-1 p-4">
                <Dynamic component={subTab()?.comp} />
            </div>
        </div>
    </LayoutContent>
}