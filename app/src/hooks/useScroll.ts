import { createEffect } from "solid-js";
import { chatInputState, activeConversationId, uiBlocks } from "../signals/chatSignals";

export function useScroll(scrollContainerRef: () => HTMLDivElement | undefined) {
  let previousConvId: string | null = null;
  let scrollPending = false;
  let seenEmpty = false;

  // Scroll to bottom instantly ONLY when conversation changes (not on every message)
  createEffect(() => {
    const convId = activeConversationId();
    const blocks = uiBlocks();
    const container = scrollContainerRef();

    console.log('[useScroll] activeConversationId or uiBlocks changed:', convId, 'blocks:', blocks.length, 'scrollPending:', scrollPending, 'seenEmpty:', seenEmpty);

    // Conversation changed - mark scroll as pending and wait for empty state
    if (convId !== previousConvId) {
      previousConvId = convId;
      scrollPending = true;
      seenEmpty = false;
      console.log('[useScroll] conversation changed, scroll pending, waiting for empty');
      return;
    }

    // If scroll is pending, wait for blocks to be empty first
    if (scrollPending && blocks.length === 0) {
      seenEmpty = true;
      console.log('[useScroll] blocks cleared, waiting for new blocks to load');
      return;
    }

    // Only scroll when we've seen empty AND now have blocks
    if (scrollPending && seenEmpty && container && blocks.length > 0 && convId) {
      scrollPending = false;
      seenEmpty = false;
      console.log('[useScroll] new blocks loaded, scheduling scroll');

      // Poll for content to be rendered by checking scrollHeight changes
      let lastHeight = 0;
      let stableCount = 0;
      const checkAndScroll = () => {
        const currentHeight = container.scrollHeight;
        console.log('[useScroll] checking scrollHeight:', currentHeight, 'lastHeight:', lastHeight);

        if (currentHeight === lastHeight) {
          stableCount++;
          if (stableCount >= 2) {
            // Height is stable, scroll now
            console.log('[useScroll] scrolling to bottom, final scrollHeight:', currentHeight);
            container.scrollTo({
              top: currentHeight,
              behavior: "instant"
            });
            return;
          }
        } else {
          stableCount = 0;
        }

        lastHeight = currentHeight;
        requestAnimationFrame(checkAndScroll);
      };

      requestAnimationFrame(checkAndScroll);
    }
  });

  createEffect(() => {
    const cnt = scrollContainerRef();
    if (!cnt) return;
    const chatInputStateValue = chatInputState();
    console.log('[useScroll] chatInputState changed:', chatInputStateValue);
    if (chatInputStateValue === 'idle') return;

    if (chatInputStateValue === 'user_sent') {
      setTimeout(() => {
        const container = scrollContainerRef();
        if (container) {
          console.log('[useScroll] user_sent: scrolling to bottom');
          container.scrollTo({
            top: container.scrollHeight,
            behavior: "smooth"
          });
        }
      }, 100);
      return;
    }
  });
}
