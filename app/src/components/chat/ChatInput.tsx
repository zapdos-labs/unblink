import { FiArrowUp, FiSquare } from "solid-icons/fi";
import { onMount } from "solid-js";
import { inputValue, isLoading, setInputValue, isTextareaFocused, setIsTextareaFocused } from "../../signals/chatSignals";
import { useChat } from "../../hooks/useChat";
import { BsSquareFill } from "solid-icons/bs";

export default function ChatInput() {
  const { sendMessage, stopGeneration } = useChat();

  let textareaRef: HTMLTextAreaElement | undefined;

  onMount(() => {
    textareaRef?.focus();
  });

  const handleKeyPress = (e: KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      sendMessage();
    }
  };

  const ChatSendButton = () => {
    return <button
      onClick={() => isLoading() ? stopGeneration() : sendMessage()}
      disabled={!isLoading() && !inputValue().trim()}
      class="p-2.5 rounded-full bg-neu-200 text-neu-900 disabled:opacity-50 disabled:cursor-not-allowed hover:bg-white hover:shadow-xl hover:scale-105 transition-all duration-150 shadow-lg"
    >
      {isLoading() ? <BsSquareFill size={20} /> : <FiArrowUp size={20} />}
    </button>
  }

  return (
    <div class="px-4 pb-2">


      <div
        data-ta-state={isTextareaFocused() ? 'focused' : 'blurred'}
        class="max-w-4xl mx-auto bg-neu-850 rounded-3xl p-3 transition-all duration-150 focus-within:border-neu-750 focus-within:bg-neu-800 shadow-lg ">

        <div class="flex items-center gap-3">
          <textarea
            ref={textareaRef}
            value={inputValue()}
            onInput={(e) => {
              setInputValue(e.currentTarget.value);
              e.currentTarget.style.height = 'auto';
              e.currentTarget.style.height = e.currentTarget.scrollHeight + 'px';
            }}
            onKeyPress={handleKeyPress}
            onFocus={() => setIsTextareaFocused(true)}
            onBlur={() => setIsTextareaFocused(false)}
            placeholder={

              "Ask anything"
            }
            rows={1}
            class="w-full px-4 py-2  text-neu-100 placeholder:text-neu-500 focus:outline-none resize-none max-h-48 overflow-y-auto text-lg  block"
          />
          <ChatSendButton />
        </div>

      </div>
    </div>
  );
}
