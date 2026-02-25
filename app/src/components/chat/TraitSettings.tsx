import { FiSettings } from 'solid-icons/fi';
import { For, createEffect, createSignal, onMount } from 'solid-js';

import { ArkSheet } from '@/src/ark/ArkSheet';
import { chatClient } from '@/src/lib/rpc';
import { activeConversationId } from '@/src/signals/chatSignals';

interface Trait {
  id: string;
  name: string;
  description: string;
}

const [traits, setTraits] = createSignal<Trait[]>([]);
const [selectedTrait, setSelectedTrait] = createSignal<string>();

export { selectedTrait, setSelectedTrait };

export default function TraitSettings() {
  createEffect(() => {
    const trait = selectedTrait();
    const conversationId = activeConversationId();
    if (!trait || !conversationId) return;

    chatClient.updateConversation({
      conversationId,
      trait,
    }).catch((error) => {
      console.error('Failed to update trait:', error);
    });
  });

  onMount(async () => {
    try {
      const res = await chatClient.getInfo({});
      const traitList = res.traits.map((t) => ({
        id: t.id,
        name: t.id.charAt(0).toUpperCase() + t.id.slice(1),
        description: t.description || '',
      }));

      setTraits(traitList);
      if (!selectedTrait()) {
        setSelectedTrait(res.defaultTrait || traitList[0]?.id);
      }
    } catch (error) {
      console.error('Failed to fetch traits:', error);
    }
  });

  const traitName = () => {
    const trait = traits().find((t) => t.id === selectedTrait());
    return trait?.name || 'Not Available';
  };

  return (
    <ArkSheet
      title="Personality Trait"
      trigger={(open, setOpen) => (
        <button
          onClick={() => setOpen(!open)}
          class="flex items-center gap-1.5 px-2 py-1.5 text-neu-400 hover:text-neu-200 hover:bg-neu-800 rounded-lg transition-colors duration-150 text-sm font-medium"
          aria-label="Trait Settings"
          title="Personality Trait (for new conversations)"
        >
          <FiSettings />
          <span>{traitName()}</span>
        </button>
      )}
    >
      {(setOpen) => (
        <div class="flex flex-col gap-2 p-2">
          <For each={traits()}>
            {(trait) => (
              <button
                onClick={() => {
                  setSelectedTrait(trait.id);
                  setOpen(false);
                }}
                class={`w-full text-left px-4 py-3 rounded-xl transition-colors outline-none ${
                  selectedTrait() === trait.id
                    ? 'bg-neu-800 text-white'
                    : 'text-neu-300 hover:bg-neu-800/50'
                }`}
              >
                <div class="font-semibold">{trait.name}</div>
                <div class="text-xs text-neu-500">{trait.description}</div>
              </button>
            )}
          </For>
        </div>
      )}
    </ArkSheet>
  );
}
