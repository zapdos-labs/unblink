import { createEffect, createSignal, onCleanup } from "solid-js";
import { eventClient } from "@/src/lib/rpc";
import type { Event } from "@/gen/service/v1/event_pb";

interface UseEventsOptions {
  nodeId: string;
  serviceId?: string;
  enabled?: boolean;
}

export function useEvents(options: UseEventsOptions) {
  const [events, setEvents] = createSignal<Event[]>([]);
  const [isConnected, setIsConnected] = createSignal(false);
  const [isLoading, setIsLoading] = createSignal(false);
  const [error, setError] = createSignal<Error | null>(null);

  let abortController: AbortController | null = null;

  // Fetch historical events
  const fetchHistoricalEvents = async () => {
    setIsLoading(true);
    setError(null);
    try {
      const response = await eventClient.listEventsByNodeId({
        nodeId: options.nodeId,
        pageSize: 10,
        pageOffset: 0,
      });
      setEvents(response.events || []);
    } catch (err) {
      console.error("[useEvents] Failed to fetch historical events:", err);
      setError(err as Error);
    } finally {
      setIsLoading(false);
    }
  };

  // Start streaming events
  const startStream = async () => {
    abortController = new AbortController();
    setIsConnected(false);
    setError(null);

    try {
      const stream = eventClient.streamEventsByNodeId(
        {
          nodeId: options.nodeId,
          serviceId: options.serviceId ?? "",
          sinceTimestamp: BigInt(0),
        },
        { signal: abortController.signal }
      );

      setIsConnected(true);

      for await (const response of stream) {
        if (response.payload.case === "event") {
          const event = response.payload.value;
          // Add new event to the beginning of the list
          setEvents((prev) => [event, ...prev].slice(0, 10));
        } else if (response.payload.case === "heartbeat") {
          console.log("[useEvents] Heartbeat:", response.payload.value);
        }
      }
    } catch (err) {
      if ((err as Error).name !== "AbortError") {
        setError(err as Error);
        console.error("[useEvents] Stream error:", err);
      }
    } finally {
      setIsConnected(false);
      abortController = null;
    }
  };

  // Stop streaming
  const stopStream = () => {
    abortController?.abort();
  };

  // Initialize: fetch historical events and start streaming
  createEffect(() => {
    if (options.enabled === false) {
      stopStream();
      return;
    }

    const _ = options.nodeId;
    stopStream();

    // Fetch historical events first
    fetchHistoricalEvents();

    // Then start streaming
    startStream();

    onCleanup(stopStream);
  });

  return {
    events,
    isConnected,
    isLoading,
    error,
    fetchHistoricalEvents,
    startStream,
    stopStream,
  };
}
