import { createEffect, onCleanup } from "solid-js";
import { liveUpdateClient } from "@/src/lib/rpc";
import { setNodeOnlineStatus } from "@/src/shared";

interface UseLiveUpdatesOptions {
  enabled?: () => boolean;
  nodeIds: () => string[];
}

export function useLiveUpdates(options: UseLiveUpdatesOptions) {
  let abortController: AbortController | null = null;
  let reconnectTimer: number | null = null;
  let generation = 0;

  const clearReconnectTimer = () => {
    if (reconnectTimer !== null) {
      window.clearTimeout(reconnectTimer);
      reconnectTimer = null;
    }
  };

  const stopStream = () => {
    generation += 1;
    clearReconnectTimer();
    abortController?.abort();
    abortController = null;
  };

  const scheduleReconnect = (currentGeneration: number) => {
    clearReconnectTimer();
    reconnectTimer = window.setTimeout(() => {
      if (generation !== currentGeneration) {
        return;
      }
      void startStream(currentGeneration);
    }, 3000);
  };

  const startStream = async (currentGeneration: number) => {
    if (generation !== currentGeneration) {
      return;
    }

    abortController = new AbortController();

    try {
      const stream = liveUpdateClient.streamLiveUpdates(
        {
          nodeIds: options.nodeIds(),
        },
        { signal: abortController.signal }
      );

      for await (const response of stream) {
        if (generation !== currentGeneration) {
          break;
        }

        if (response.payload.case === "nodeStatusChanged") {
          const update = response.payload.value;
          setNodeOnlineStatus(update.nodeId, update.online);
        }
      }
    } catch (err) {
      if ((err as Error).name !== "AbortError" && generation === currentGeneration) {
        console.error("[useLiveUpdates] Stream error:", err);
      }
    } finally {
      if (abortController?.signal.aborted) {
        abortController = null;
        return;
      }

      abortController = null;
      if (generation === currentGeneration) {
        scheduleReconnect(currentGeneration);
      }
    }
  };

  createEffect(() => {
    const enabled = options.enabled ? options.enabled() : true;
    const nodeIds = options.nodeIds();

    stopStream();

    if (!enabled || nodeIds.length === 0) {
      return;
    }

    const currentGeneration = generation;
    void startStream(currentGeneration);
    onCleanup(stopStream);
  });
}
