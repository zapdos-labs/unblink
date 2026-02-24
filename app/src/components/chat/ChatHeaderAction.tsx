import { FiPlus } from 'solid-icons/fi';
import { useChat } from '../../hooks/useChat';
import HistoryMenu from './HistoryMenu';

export default function ChatHeaderAction() {
  const { handleNewChat } = useChat();

  return (
    <div class="flex items-center gap-3">
      <HistoryMenu />
      <button
        onClick={handleNewChat}
        class="p-2 text-neu-400 hover:text-neu-200 hover:bg-neu-800 rounded-lg transition-colors duration-150"
        aria-label="New Chat"
        title="Start a new chat"
      >
        <FiPlus />
      </button>
    </div>
  );
}
