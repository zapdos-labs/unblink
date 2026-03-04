import { services, activeTab } from '../shared'
import VideoTile from './VideoTile'
import { useEventPanel } from './EventPanel'
import { createMemo, Show } from 'solid-js'

export default function CameraView() {
  const currentViewTab = createMemo(() => {
    const tab = activeTab()
    return tab.type === 'view' ? tab : null
  })

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
              <Show when={!eventPanel.showEventPanel()}>
                <eventPanel.Toggle />
              </Show>
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
