import { FiClock } from 'solid-icons/fi';
import { Index, onMount, Show } from 'solid-js';
import { useChat } from '../../hooks/useChat';
import { activeConversationId } from '../../signals/chatSignals';
import type { Conversation } from '../../../gen/chat/v1/chat_pb';
import { ArkSheet } from '../../ark/ArkSheet';

interface HistoryMenuProps {
  class?: string;
}

export const HistoryMenu = (props: HistoryMenuProps) => {
  const { handleSelectConversation, conversations, listConversations } = useChat();

  onMount(() => {
    listConversations();
  });

  return (
    <ArkSheet
      title="Chat History"
      trigger={(open, setOpen) => (
        <button
          onClick={() => setOpen(!open)}
          class={`p-2 text-neu-400 hover:text-neu-200 hover:bg-neu-800 rounded-lg transition-colors duration-150 ${props.class || ''}`}
          aria-label="Chat History"
          title="Chat History"
        >
          <FiClock />
        </button>
      )}
    >
      {(setOpen) => (
        <div class="flex flex-col">
        <Show
          when={conversations().length > 0}
          fallback={<div class="text-sm text-neu-500 text-center py-8">No recent chats</div>}
        >
          <Index each={conversations()}>
            {(conv) => (
              <Show when={conv()} fallback={null}>
                {(c) => {
                  const isActive = () => c().id === activeConversationId();
                  return (
                    <button
                      onClick={() => {
                        handleSelectConversation(c().id);
                        setOpen(false);
                      }}
                      class={`w-full text-left px-4 py-3 rounded-xl flex items-start gap-3 transition-colors outline-none ${
                        isActive()
                          ? 'bg-neu-800 text-white'
                          : 'text-neu-300 hover:bg-neu-800/50'
                      }`}
                    >
                      <FiClock class="w-5 h-5 flex-shrink-0 mt-0.5" />
                      <div class="flex-1 min-w-0">
                        <div class="font-semibold truncate">{c().title || 'Untitled Chat'}</div>
                        {c().updatedAt && (
                          <div class="mt-0.5 text-neu-500 text-xs truncate">
                            {new Date(Number((c().updatedAt?.seconds ?? 0n)) * 1000).toLocaleString()}
                          </div>
                        )}
                      </div>
                    </button>
                  );
                }}
              </Show>
            )}
          </Index>
        </Show>
        </div>
      )}
    </ArkSheet>
  );
};

export default HistoryMenu;
