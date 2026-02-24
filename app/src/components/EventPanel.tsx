import { FiChevronLeft, FiChevronRight, FiClock } from 'solid-icons/fi';
import { For, Show, createSignal, type Accessor } from 'solid-js';
import { useEvents } from '../hooks/useEvents';
import { services } from '../shared';
import { formatDistance } from 'date-fns';
import { Button } from './Button';

interface EventPanelProps {
  nodeId: string;
  serviceId?: Accessor<string | undefined>;
}

export function useEventPanel(props: EventPanelProps) {
  const [showEventPanel, setShowEventPanel] = createSignal(true);

  // Get all services
  const allServices = () => services();

  // Get service name by ID
  const getServiceName = (serviceId: string): string => {
    const service = allServices().find((s) => s.id === serviceId);
    if (service) {
      return service.name || serviceId;
    }
    return serviceId;
  };

  // Use events hook
  const { events, isLoading } = useEvents({
    nodeId: props.nodeId,
    serviceId: props.serviceId?.(),
    enabled: showEventPanel(),
  });

  // Filter events by serviceId if provided
  const filteredEvents = () => {
    const sid = props.serviceId?.();
    if (!sid) {
      return events();
    }
    return events().filter((event) => event.serviceId === sid);
  };

  const Toggle = () => (
    <Button onClick={() => setShowEventPanel((prev) => !prev)}>
      <Show when={showEventPanel()} fallback={<>
        <FiChevronLeft class="w-4 h-4" />
        <span>Events</span>
      </>}>
        <FiChevronRight class="w-4 h-4" />
      </Show>
    </Button>
  );

  // Component to display event data
  const EventDisplay = (props: { data: unknown }) => (
    <div class="mt-2 max-h-48 overflow-y-auto">
      <pre class="text-xs text-neu-300 whitespace-pre-wrap break-words font-mono">
        {JSON.stringify(props.data, null, 2)}
      </pre>
    </div>
  );

  return {
    showEventPanel,
    setShowEventPanel,
    Toggle,
    Comp: () => (
      <div
        data-show={showEventPanel()}
        classList={{ "flex-none w-[400px] h-screen overflow-hidden flex flex-col": showEventPanel(), "hidden": !showEventPanel() }}
      >
        <div class="border-l border-neu-800 bg-neu-900 flex-1 flex flex-col h-full overflow-hidden">
          {/* Panel Header - only visible when panel is open */}
          <Show when={showEventPanel()}>
            <div class="h-14 flex items-center gap-2 p-2 border-b border-neu-800">
              <div class="flex-1" />
              <Button onClick={() => setShowEventPanel(false)}>
                <FiChevronRight class="w-4 h-4" />
              </Button>
            </div>
          </Show>

          {/* Events List */}
          <Show when={showEventPanel()}>
            <div class="flex-1 p-2 overflow-y-auto space-y-4">
              <Show when={isLoading()}>
                <div class="text-center text-neu-500 py-8">Loading events...</div>
              </Show>
              <Show when={!isLoading()}>
                <Show when={filteredEvents().length > 0} fallback={
                  <div class="text-center text-neu-500 py-8">
                    Waiting for events...
                  </div>
                }>
                  <For each={filteredEvents()}>
                    {(event) => (
                      <div class="animate-push-down p-4 bg-neu-850 space-y-2 rounded-lg">
                        <div class="font-semibold text-white">{getServiceName(event.serviceId)}</div>
                        <div class="text-neu-400 text-sm flex items-center gap-1">
                          <FiClock class="w-3 h-3" />
                          {event.createdAt ? formatDistance(new Date(Number(event.createdAt.seconds) * 1000), new Date(), {
                            addSuffix: true,
                            includeSeconds: true,
                          }) : 'Unknown time'}
                        </div>
                        <EventDisplay data={event.payload} />
                      </div>
                    )}
                  </For>
                </Show>
              </Show>
            </div>
          </Show>
        </div>
      </div>
    ),
  };
}
