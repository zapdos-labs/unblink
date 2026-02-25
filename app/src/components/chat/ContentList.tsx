import { isLoading, uiBlocks } from "../../signals/chatSignals";
import UIBlockList from "./UIBlockList";
import { useScroll } from "../../hooks/useScroll";
import { Show, onMount, onCleanup } from "solid-js";
import WelcomeScreen from "./WelcomeScreen";

export default function ContentList() {
  let scrollContainerRef: HTMLDivElement | undefined;
  useScroll(() => scrollContainerRef);

  const handleScroll = (e: CustomEvent<string>) => {
    if (!scrollContainerRef) return;
    if (e.detail === "bottom") {
      scrollContainerRef.scrollTo({
        top: scrollContainerRef.scrollHeight,
        behavior: "smooth"
      });
    } else if (e.detail === "top") {
      scrollContainerRef.scrollTo({
        top: 0,
        behavior: "smooth"
      });
    }
  };

  onMount(() => {
    window.addEventListener("chat-scroll", handleScroll as EventListener);
  });

  onCleanup(() => {
    window.removeEventListener("chat-scroll", handleScroll as EventListener);
  });

  return (
    <div
      ref={(el) => scrollContainerRef = el}
      class="flex-1 overflow-y-auto min-h-0 pb-32"
      style={{ "overflow-anchor": "none" }}
    >
      <Show
        when={uiBlocks().length > 0}
        fallback={<WelcomeScreen />}
      >
        <UIBlockList
          blocks={uiBlocks()}
          showLoading={isLoading()}
        />
      </Show>
    </div>
  );
}
