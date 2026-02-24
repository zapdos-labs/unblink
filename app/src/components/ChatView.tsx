import MessageList from './chat/MessageList';
import ChatInput from './chat/ChatInput';
import ChatHeaderAction from './chat/ChatHeaderAction';

export default function ChatView() {
  return (
    <div class="relative flex flex-col h-full bg-neu-900">
      {/* Header */}
      <div class="flex items-center justify-between px-4 py-3 border-b border-neu-900">
        <ChatHeaderAction />
      </div>

      {/* Message List */}
      <MessageList />

      {/* Chat Input - Floating at bottom */}
      <div class="absolute bottom-0 left-0 right-0 z-10 bg-gradient-to-t from-neu-900 via-neu-900 to-transparent">
        <ChatInput />
      </div>
    </div>
  );
}
