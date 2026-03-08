import { services, activeTab } from '../shared'
import VideoTile, { type VideoTileStatus } from './VideoTile'
import { useEventPanel } from './EventPanel'
import { For, Show, createSignal } from 'solid-js'
import { Menu } from '@ark-ui/solid/menu'
import { Portal } from 'solid-js/web'
import { FiActivity } from 'solid-icons/fi'

export default function CameraView() {
  const [videoStatus, setVideoStatus] = createSignal<VideoTileStatus | null>(null)

  const currentViewTab = () => {
    const tab = activeTab()
    return tab.type === 'view' ? tab : null
  }

  // Get current service ID for filtering events
  const serviceIdAccessor = () => currentViewTab()?.serviceId

  // Get nodeId safely
  const getNodeId = () => currentViewTab()?.nodeId ?? ''

  // Get service name
  const getServiceName = () => {
    const tab = currentViewTab()
    if (!tab) return ''
    const service = services().find((s) => s.id === tab.serviceId)
    return service?.name || tab.name || ''
  }

  // Initialize event panel
  const eventPanel = useEventPanel({
    nodeId: getNodeId(),
    serviceId: serviceIdAccessor,
  })

  const statsRows = () => {
    const status = videoStatus()
    if (!status) {
      return [
        { label: 'Status', value: 'No data yet' },
      ]
    }

    const dims = status.inbound.frameWidth > 0 && status.inbound.frameHeight > 0
      ? `${status.inbound.frameWidth}x${status.inbound.frameHeight}`
      : 'unknown'

    return [
      { label: 'ICE', value: status.iceConnectionState },
      { label: 'Peer', value: status.peerConnectionState },
      { label: 'Loading', value: status.loading ? 'yes' : 'no' },
      { label: 'Error', value: status.error ?? 'none' },
      { label: 'Frames decoded', value: String(status.inbound.framesDecoded) },
      { label: 'Frames dropped', value: String(status.inbound.framesDropped) },
      { label: 'Packets lost', value: String(status.inbound.packetsLost) },
      { label: 'Jitter', value: `${status.inbound.jitterMs.toFixed(1)} ms` },
      { label: 'Bytes received', value: String(status.inbound.bytesReceived) },
      { label: 'Video size', value: dims },
      { label: 'Processor frames', value: String(status.renderer.processorFrames) },
      { label: 'Rendered frames', value: String(status.renderer.renderedFrames) },
      { label: 'Queue drops', value: String(status.renderer.queuedDrops) },
      {
        label: 'Last render age',
        value: status.renderer.lastRenderAgeMs === null ? 'n/a' : `${status.renderer.lastRenderAgeMs} ms`,
      },
    ]
  }

  return (
    <Show when={currentViewTab()} keyed>
      {(tab) => (
        <div class="flex h-full">
          {/* Main Content Area */}
          <div class="flex-1 h-full overflow-hidden flex flex-col">
            {/* Header */}
            <div class="flex-none h-14 flex items-center px-4 bg-neu-900 border-b border-neu-800">
              <div class="text-lg font-medium">{getServiceName()}</div>
              <div class="flex-1" />
              <div class="flex items-center gap-2">
                <Menu.Root>
                  <Menu.Trigger class="focus:outline-none drop-shadow-xl px-3 py-1.5 rounded-xl border border-neu-750 bg-neu-800 hover:bg-neu-850 flex items-center space-x-2 text-sm text-white transition-colors">
                    <FiActivity class="w-4 h-4" />
                    <span>Stats</span>
                  </Menu.Trigger>
                  <Portal>
                    <Menu.Positioner>
                      <Menu.Content class="focus:outline-none  min-w-[22rem] max-h-[24rem] overflow-y-auto bg-neu-850 border border-neu-800 rounded-lg shadow-lg py-2 z-50">
                        <For each={statsRows()}>
                          {(row) => (
                            <div class="px-3 py-1.5 text-sm flex items-center justify-between gap-4">
                              <span class="text-neu-400">{row.label}</span>
                              <span class="text-white font-mono text-xs">{row.value}</span>
                            </div>
                          )}
                        </For>
                      </Menu.Content>
                    </Menu.Positioner>
                  </Portal>
                </Menu.Root>
                <Show when={!eventPanel.showEventPanel()}>
                  <eventPanel.Toggle />
                </Show>
              </div>
            </div>

            {/* Video */}
            <div class="flex-1 min-h-0">
              <div class="h-full bg-neu-900">
                <Show when={services().find((s) => s.id === tab.serviceId)} keyed>
                  {(service) => (
                    <VideoTile
                      nodeId={tab.nodeId}
                      serviceId={tab.serviceId}
                      serviceUrl={service.serviceUrl}
                      name={tab.name}
                      onStatus={setVideoStatus}
                    />
                  )}
                </Show>
              </div>
            </div>
          </div>

          {/* Events Sidebar */}
          <eventPanel.Comp />
        </div>
      )}
    </Show>
  )
}
