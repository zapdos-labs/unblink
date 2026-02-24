import { isLoading, uiBlocks } from "../../signals/chatSignals";
import UIBlockList from "./UIBlockList";
import LoadingDots from "./LoadingDots";
import { useScroll } from "../../hooks/useScroll";
import { Show } from "solid-js";

export default function MessageList() {
  let scrollContainerRef: HTMLDivElement | undefined;
  useScroll(() => scrollContainerRef);

  return (
    <div
      ref={(el) => scrollContainerRef = el}
      class="flex-1 overflow-y-auto min-h-0 pb-32"
      style={{ "overflow-anchor": "none" }}
    >
      <Show
        when={uiBlocks().length > 0}
        fallback={
          <div class="flex items-center justify-center h-full">
            <div class="flex flex-col items-center gap-4">
              <h1 class="text-6xl font-bold text-neutral-100">
                Chat
              </h1>
              <h2 class="text-neutral-400">
                Start a conversation
              </h2>
            </div>
          </div>
        }
      >
        <UIBlockList
          blocks={uiBlocks()}
          showLoading={isLoading()}
        />
      </Show>
    </div>
  );
}
