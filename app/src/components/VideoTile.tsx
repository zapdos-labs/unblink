import { createSignal, onCleanup, Show, onMount } from 'solid-js'
import { webrtcClient } from '@/src/lib/rpc'

interface Props {
  nodeId: string
  serviceId: string
  serviceUrl: string
  name?: string
}

export default function VideoTile(props: Props) {
  let videoRef: HTMLVideoElement | undefined

  const [pc, setPc] = createSignal<RTCPeerConnection | null>(null)
  const [loading, setLoading] = createSignal(true)
  const [error, setError] = createSignal<string | null>(null)
  const [connected, setConnected] = createSignal(false)

  const connect = async () => {
    // Prevent multiple simultaneous connections
    if (connected()) {
      console.log('[VideoTile] Already connected, skipping')
      return
    }

    console.log('[VideoTile] Connecting to:', {
      nodeId: props.nodeId,
      serviceId: props.serviceId,
      serviceUrl: props.serviceUrl,
    })

    setConnected(true)
    setLoading(true)
    setError(null)

    // Close existing connection
    const existingPc = pc()
    if (existingPc) {
      existingPc.close()
      setPc(null)
    }

    try {
      // Create new peer connection
      const newPc = new RTCPeerConnection({
        iceServers: [{ urls: 'stun:stun.l.google.com:19302' }],
      })

      setPc(newPc)

      // Handle incoming tracks (both video and audio)
      newPc.ontrack = (event) => {
        console.log(`[VideoTile] Got ${event.track.kind} track:`, event.track.label || event.track.id)

        if (!videoRef) {
          return
        }

        // Add track to the video element's MediaStream
        if (event.streams && event.streams[0]) {
          console.log('[VideoTile] Using existing stream', event.streams[0].id)
          if (videoRef.srcObject !== event.streams[0]) {
            videoRef.srcObject = event.streams[0]
          }
        } else {
          console.log('[VideoTile] No stream in event, adding track to existing or new stream')
          let stream = videoRef.srcObject as MediaStream | null
          if (!stream) {
            stream = new MediaStream()
            videoRef.srcObject = stream
          }
          stream.addTrack(event.track)
        }

        // Only trigger play and hide loading on video track
        if (event.track.kind === 'video') {
          // Explicitly play to ensure autoplay works even if policy is strict
          videoRef.play().catch(e => {
            if (e.name !== 'AbortError') {
              console.error('[VideoTile] Play error:', e)
            }
          })
          setLoading(false)
        }
      }

      // Monitor ICE connection state
      newPc.oniceconnectionstatechange = () => {
        console.log('[VideoTile] ICE connection state:', newPc.iceConnectionState)
        if (
          newPc.iceConnectionState === 'failed' ||
          newPc.iceConnectionState === 'disconnected' ||
          newPc.iceConnectionState === 'closed'
        ) {
          setError(`Connection ${newPc.iceConnectionState}`)
          setLoading(false)
          setConnected(false)
        }
      }

      // Add transceivers for receiving video and audio
      newPc.addTransceiver('video', { direction: 'recvonly' })
      newPc.addTransceiver('audio', { direction: 'recvonly' })

      // Create offer
      const offer = await newPc.createOffer()
      await newPc.setLocalDescription(offer)

      console.log('[VideoTile] Sending WebRTC session request...')

      // Request WebRTC session from server
      const response = await webrtcClient.createWebRTCSession({
        nodeId: props.nodeId,
        serviceId: props.serviceId,
        serviceUrl: props.serviceUrl,
        sdpOffer: newPc.localDescription?.sdp || '',
      })

      console.log('[VideoTile] Got session response, session ID:', response.sessionId)

      // Set remote description (answer from server)
      await newPc.setRemoteDescription(
        new RTCSessionDescription({
          type: 'answer',
          sdp: response.sdpAnswer,
        })
      )

      console.log('[VideoTile] WebRTC session established')
    } catch (err) {
      console.error('[VideoTile] Connection error:', err)
      setError(err instanceof Error ? err.message : 'Connection failed')
      setLoading(false)
      setConnected(false)
    }
  }

  // Connect only once when component mounts
  onMount(() => {
    connect()
  })

  // Cleanup on unmount
  onCleanup(() => {
    const connection = pc()
    if (connection) {
      console.log('[VideoTile] Cleaning up connection')
      connection.close()
      setPc(null)
    }
    setConnected(false)
  })

  return (
    <div class="relative w-full h-full bg-neu-900 overflow-hidden">
      {/* Video element */}
      <video
        ref={videoRef}
        class="w-full h-full object-contain"
        autoplay
        playsinline
      />

      {/* Loading spinner */}
      <Show when={loading()}>
        <div class="absolute inset-0 flex items-center justify-center bg-neu-950/50">
          <div class="flex flex-col items-center gap-3">
            <div class="w-12 h-12 border-4 border-neu-700 border-t-neu-400 rounded-full animate-spin" />
            <div class="text-neu-300 text-sm">Connecting...</div>
          </div>
        </div>
      </Show>

      {/* Error message */}
      <Show when={error()}>
        <div class="absolute inset-0 flex items-center justify-center bg-neu-950/70">
          <div class="flex flex-col items-center gap-3 px-6">
            <div class="text-neu-400 text-lg">⚠️</div>
            <div class="text-neu-300 text-sm text-center">{error()}</div>
            <button
              onClick={() => {
                setConnected(false)
                connect()
              }}
              class="px-4 py-2 bg-neu-700 hover:bg-neu-600 text-neu-100 text-sm rounded-md transition-colors"
            >
              Retry
            </button>
          </div>
        </div>
      </Show>

      {/* Service name label */}
      <Show when={props.name}>
        <div class="absolute bottom-0 left-0 right-0 px-4 py-2 bg-gradient-to-t from-neu-900/80 to-transparent">
          <div class="text-neu-100 text-sm font-medium">{props.name}</div>
        </div>
      </Show>
    </div>
  )
}
