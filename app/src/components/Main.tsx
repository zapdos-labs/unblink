import { onMount, Show, Switch, Match } from 'solid-js'
import { activeTab, fetchServices, permissionState, services } from '../shared'
import { setAuthScreen } from '../signals/authSignals'
import CameraView from './CameraView'
import SideBar from './SideBar'
import ChatView from './ChatView'
import EventsView from './EventsView'
import SettingsView from './SettingsView'

interface MainProps {
  nodeId: string
}

export default function Main(props: MainProps) {
  // Fetch services on mount - only runs after auth is complete
  onMount(() => {
    fetchServices(props.nodeId)
  })

  return (
    <Show
      when={props.nodeId}
      fallback={
        <div class="flex h-full items-center justify-center">
          <div class="text-center">
            <p class="text-gray-400">No node ID in URL</p>
            <p class="text-sm text-gray-500 mt-2">Navigate to /node/YOUR_NODE_ID</p>
          </div>
        </div>
      }
    >
      {(() => {
        const state = permissionState()
        if (state === 'idle') return null
        if (state === 'loading') {
          return (
            <div class="flex items-center justify-center h-screen">
              <p class="text-white">Loading...</p>
            </div>
          )
        }
        if (state === 'denied') {
          return (
            <div class="flex items-center justify-center h-screen bg-neu-950">
              <div class="bg-neu-900 p-8 rounded-lg shadow-lg w-96 border border-neu-800 space-y-4">
                <div class="space-y-2">
                  <div class="flex justify-center">
                    <img src="/logo.svg" class="w-18 h-18" alt="Logo" />
                  </div>
                  <h2 class="text-2xl font-semibold text-white text-center">Authentication Required</h2>
                  <p class="text-sm text-neu-400 text-center">You need to login to access this node.</p>
                </div>
                <button
                  onClick={() => setAuthScreen("login")}
                  class="w-full px-4 py-2 rounded-xl border border-neu-750 bg-neu-800 hover:bg-neu-850 flex items-center justify-center space-x-2"
                >
                  <span class="text-white">Login</span>
                </button>
              </div>
            </div>
          )
        }
        // state === 'ok' - show dashboard
        return <div class="flex items-start h-full">
          <SideBar nodeId={props.nodeId} />

          {/* Main Content Area */}
          <div class="flex-1 h-full">
            <Switch fallback={<ChatView />}>
              <Match when={activeTab().type === 'chat'}>
                <ChatView />
              </Match>
              <Match when={activeTab().type === 'view'}>
                <CameraView />
              </Match>
              <Match when={activeTab().type === 'events'}>
                <EventsView nodeId={props.nodeId} />
              </Match>
              <Match when={activeTab().type === 'settings'}>
                <SettingsView nodeId={props.nodeId} />
              </Match>
            </Switch>
          </div>
        </div>
      })()}
    </Show>
  )
}
