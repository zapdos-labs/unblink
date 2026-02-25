import { onMount, onCleanup } from "solid-js";

export default function WelcomeScreen() {
  let textRef: HTMLSpanElement | undefined;
  let cursorRef: HTMLSpanElement | undefined;
  const text = "How can I help you today?";
  let interval: ReturnType<typeof setInterval>;

  onMount(() => {
    if (!textRef || !cursorRef) return;

    let index = 0;
    textRef.textContent = "";

    interval = setInterval(() => {
      if (index < text.length) {
        textRef.textContent += text.charAt(index);
        index++;
        updateCursorPosition();
      } else {
        clearInterval(interval);
        setTimeout(() => {
          if (cursorRef) cursorRef.style.display = "none";
        }, 1000);
      }
    }, 50);
  });

  const updateCursorPosition = () => {
    if (textRef && cursorRef) {
      cursorRef.style.left = `${textRef.offsetWidth}px`;
    }
  };

  onCleanup(() => {
    if (interval) clearInterval(interval);
  });

  return (
    <div class="flex items-center justify-center h-full px-4">
      <div class="flex flex-col items-center empty-state-container">
        <div class="relative mb-4">
          <video
            src="/default.mp4"
            autoplay
            loop
            muted
            playsinline
            class="w-60 h-60 rounded-lg border border-neutral-800 object-cover hover:scale-105 hover:border-neutral-600 transition-transform duration-500"
          />
          <div class="absolute inset-0 rounded-lg bg-linear-to-t from-neutral-900/20 to-transparent pointer-events-none" />
        </div>
        <div class="text-lg font-bold text-neutral-100 empty-state-name">
          AI Assistant
        </div>
        <div class="text-neutral-400 mt-1 empty-state-subtitle relative inline-flex">
          <span ref={textRef}></span>
          <span ref={cursorRef} class="typing-cursor"></span>
        </div>
      </div>
    </div>
  );
}
