import { Suspense } from 'solid-js'
import { Authenticated } from './components/Authenticated'
import Main from './components/Main'
import ArkToast from './ark/ArkToast'

function App() {
  // Parse node ID from URL path: /node/{nodeId}
  const getNodeIdFromPath = () => {
    const path = window.location.pathname
    const match = path.match(/^\/node\/([^/]+)/)
    return match ? match[1] : null
  }

  const nodeId = getNodeIdFromPath()

  return (
    <div class="h-[100dvh] bg-black text-white">
      <ArkToast />
      <Suspense fallback={<div class="flex h-[100dvh] items-center justify-center text-white">Loading...</div>}>
        <Authenticated>
          <Main nodeId={nodeId!} />
        </Authenticated>
      </Suspense>
    </div>
  )
}

export default App
