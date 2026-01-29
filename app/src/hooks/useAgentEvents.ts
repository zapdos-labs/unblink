import { createEffect, createSignal, onCleanup } from "solid-js";
import { eventClient } from "@/src/lib/rpc";
import type { Event } from "@/gen/service/v1/event_pb";

interface UseAgentEventsOptions {
  nodeId: string;
  serviceId?: string;
  sinceTimestamp?: number;
}

export function useAgentEvents(options: UseAgentEventsOptions) {
  const [isConnected, setIsConnected] = createSignal(false);
  const [error, setError] = createSignal<Error | null>(null);

  let abortController: AbortController | null = null;

  const startStream = async () => {
    abortController = new AbortController();
    setIsConnected(false);
    setError(null);

    try {
      const stream = eventClient.streamEventsByNodeId(
        {
          nodeId: options.nodeId,
          serviceId: options.serviceId,
          sinceTimestamp: BigInt(options.sinceTimestamp ?? 0),
        },
        { signal: abortController.signal }
      );

      setIsConnected(true);

      for await (const response of stream) {
        if (response.payload.case === "event") {
          const event = response.payload.value;
          // Console log the event table row
          console.log({ type: 'event-table-row', row: event });
        } else if (response.payload.case === "heartbeat") {
          console.log("[useAgentEvents] Heartbeat:", response.payload.value);
        }
      }
    } catch (err) {
      if ((err as Error).name !== "AbortError") {
        setError(err as Error);
      }
    } finally {
      setIsConnected(false);
      abortController = null;
    }
  };

  const stopStream = () => {
    abortController?.abort();
  };

  createEffect(() => {
    const _ = options.nodeId;
    stopStream();
    startStream();
    onCleanup(stopStream);
  });

  return {
    isConnected,
    error,
    startStream,
    stopStream,
  };
}
