import { FiBell, FiCpu } from "solid-icons/fi";
import { createEffect, createSignal, For, untrack, type Accessor, type Setter, type ValidComponent } from "solid-js";
import { Dynamic } from "solid-js/web";
import ArkSwitch from "~/src/ark/ArkSwitch";
import { saveSettings, settings } from "~/src/shared";
import LayoutContent from "./LayoutContent";
import { useAlertsSubTab } from "./settings/useAlertsSubTab";

export type UseSubTab = {
    comp: ValidComponent,
    keys: () => string[]
}

export function useMachineLearningSubTab(props: {
    scratchpad: Accessor<Record<string, string>>,
    setScratchpad: Setter<Record<string, string>>,
}): UseSubTab {
    return {
        comp: () => <div>
            <div class="bg-neu-850 border border-neu-800 rounded-lg p-6">
                <div class="flex items-center justify-between">
                    <ArkSwitch
                        checked={() => props.scratchpad()['object_detection_enabled'] === 'true'}
                        onCheckedChange={(details) => props.setScratchpad((prev) => ({
                            ...prev,
                            object_detection_enabled: details.checked ? 'true' : 'false'
                        }))}
                        label="Enable Object Detection"
                    />
                </div>
            </div>
        </div>,
        keys: () => ['object_detection_enabled']
    }
}



export default function SettingsContent() {
    const [scratchpad, setScratchpad] = createSignal<Record<string, string>>(
        settings()
    );

    const subtabs = [
        {
            id: 'machine_learning',
            name: 'Machine Learning',
            icon: FiCpu,
            use: useMachineLearningSubTab,
        },
        {
            id: 'alerts',
            name: 'Alerts',
            icon: FiBell,
            use: useAlertsSubTab,
        }
    ];

    const [subtab, setSubtab] = createSignal<{
        type: 'machine_learning' | 'alerts';
    }>({
        type: 'machine_learning'
    });

    const [useSubTab, setUseSubTab] = createSignal<UseSubTab>();

    createEffect(() => {
        const _subtab = subtab();
        const s = subtabs.find(t => t.id === _subtab.type);
        if (!s) {
            setUseSubTab(undefined);
            return;
        }

        const _use = s.use({ scratchpad, setScratchpad });
        setUseSubTab(_use);
    })


    const handleSaveSettings = async () => {
        const ust = untrack(useSubTab);
        if (!ust) return;
        const keys = ust.keys();
        for (const key of keys) {
            const value = scratchpad()[key];
            if (value === undefined) continue;
            await saveSettings(key, value);
        }
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
        <div class="flex items-start">
            <div class="w-xs flex-none ml-4 my-4 mr-2">
                <div class="space-y-1">
                    <For each={subtabs}>
                        {_tab => <button
                            onClick={() => setSubtab({
                                type: _tab.id as 'machine_learning' | 'alerts'
                            })}
                            data-active={
                                subtab().type === _tab.id
                            }
                            class="w-full flex items-center space-x-3 px-4 py-2 rounded-lg text-neu-400 hover:bg-violet-400/10 data-[active=true]:bg-violet-400/10 data-[active=true]:text-violet-400 hover:text-violet-400">
                            <_tab.icon class="w-4 h-4" />
                            <div>{_tab.name}</div>

                        </button>}
                    </For>
                </div>
            </div>
            <div class="flex-1 my-4 mr-4">
                <Dynamic component={useSubTab()?.comp} />
            </div>
        </div>
    </LayoutContent>
}