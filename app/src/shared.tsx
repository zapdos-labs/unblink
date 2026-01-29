import { createSignal } from 'solid-js'
import { toaster } from './ark/ArkToast'
import { serviceClient } from './lib/rpc'
import { setAuthScreen } from './signals/authSignals'

export interface Service {
  id: string
  name: string
  nodeId: string
  serviceUrl: string
  description?: string
}

export type Tab =
  | { type: 'chat' }
  | { type: 'view'; nodeId: string; serviceId: string; name: string }
  | { type: 'settings' }
  | { type: 'events' }
  | { type: 'camera' }

// Services state
export const [services, setServices] = createSignal<Service[]>([])

// Active tab state - default to chat
export const [activeTab, setActiveTab] = createSignal<Tab>({ type: 'chat' })

// Permission state
export type PermissionState = 'idle' | 'loading' | 'ok' | 'denied'
export const [permissionState, setPermissionState] = createSignal<PermissionState>('idle')

// Fetch services from server
export async function fetchServices(nodeId: string) {
  setPermissionState('loading')

  try {
    const res = await serviceClient.listServicesByNodeId({ nodeId })
    if (res.services) {
      const loadedServices: Service[] = res.services.map(s => ({
        id: s.id,
        name: s.name || s.id,
        nodeId: s.nodeId,
        serviceUrl: s.url,
      }))
      setServices(loadedServices)
    }
    setPermissionState('ok')
  } catch (error) {
    console.error('Failed to fetch services:', error)
    const errorMessage = error instanceof Error ? error.message : String(error)

    // Check if it's a permission denied error
    if (errorMessage.includes('permission_denied') || errorMessage.includes("you don't have access to this node")) {
      setPermissionState('denied')
      return
    }

    setPermissionState('idle')
    // Show toast for other errors
    toaster.create({
      title: 'Failed to load services',
      description: errorMessage,
      type: 'error',
    })
  }
}
