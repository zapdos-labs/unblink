import { createEffect, createSignal, onCleanup, untrack } from "solid-js";
import { eventClient } from "@/src/lib/rpc";
import type { Event } from "@/gen/service/v1/event_pb";

interface UseEventsOptions {
  nodeId: string;
  serviceId?: string;
  enabled?: boolean;
}

export function useEvents(options: UseEventsOptions) {
  const [events, setEvents] = createSignal<Event[]>([]);
  const [isLoading, setIsLoading] = createSignal(false);
  const [error, setError] = createSignal<Error | null>(null);

  let abortController: AbortController | null = null;
  let streamActive = false;

  // Stop streaming
  const stopStream = () => {
    abortController?.abort();
    abortController = null;
    streamActive = false;
  };

  // Initialize: fetch historical events and start streaming
  createEffect(() => {
    // Check enabled state without tracking it
    const enabled = untrack(() => options.enabled);
    if (enabled === false) {
      stopStream();
      return;
    }

    // Capture current values without tracking
    const nodeId = untrack(() => options.nodeId);
    const serviceId = untrack(() => options.serviceId);

    // Stop any existing stream before starting a new one
    stopStream();

    // Mark stream as active to prevent race conditions
    streamActive = true;
    const currentAbortController = new AbortController();
    abortController = currentAbortController;

    // Fetch historical events
    const fetchAndStream = async () => {
      setIsLoading(true);
      setError(null);

      try {
        const response = await eventClient.listEventsByNodeId({
          nodeId,
          pageSize: 10,
          pageOffset: 0,
        });

        // Check if we're still the active stream
        if (abortController !== currentAbortController) {
          return; // This effect was cleaned up, abort
        }

        setEvents(response.events || []);
      } catch (err) {
        // Only set error if we're still the active stream
        if (abortController === currentAbortController) {
          console.error("[useEvents] Failed to fetch historical events:", err);
          setError(err as Error);
        }
        return;
      } finally {
        if (abortController === currentAbortController) {
          setIsLoading(false);
        }
      }

      // Then start streaming
      try {
        const stream = eventClient.streamEventsByNodeId(
          {
            nodeId,
            serviceId: serviceId ?? "",
            sinceTimestamp: BigInt(0),
          },
          { signal: currentAbortController.signal }
        );

        for await (const response of stream) {
          // Double-check we're still the active stream
          if (abortController !== currentAbortController) {
            break; // This effect was cleaned up, abort
          }

          if (response.payload.case === "event") {
            const event = response.payload.value;
            // Add new event to the beginning of the list
            setEvents((prev) => [event, ...prev].slice(0, 10));
          } else if (response.payload.case === "heartbeat") {
            console.log("[useEvents] Heartbeat:", response.payload.value);
          }
        }
      } catch (err) {
        // Only set error if this wasn't an abort
        if ((err as Error).name !== "AbortError" && abortController === currentAbortController) {
          setError(err as Error);
          console.error("[useEvents] Stream error:", err);
        }
      } finally {
        if (abortController === currentAbortController) {
          streamActive = false;
          abortController = null;
        }
      }
    };

    fetchAndStream();

    onCleanup(stopStream);
  });

  return {
    events,
    isLoading,
    error,
  };
}
