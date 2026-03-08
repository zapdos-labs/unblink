import { createEffect, createSignal, onCleanup, Show, untrack } from 'solid-js'
import { webrtcClient } from '@/src/lib/rpc'

interface Props {
  nodeId: string
  serviceId: string
  serviceUrl: string
  name?: string
  onStatus?: (status: VideoTileStatus | null) => void
}

interface WebGLState {
  gl: WebGLRenderingContext
  program: WebGLProgram
  texture: WebGLTexture
}

export interface VideoTileStatus {
  updatedAtMs: number
  iceConnectionState: RTCIceConnectionState
  peerConnectionState: RTCPeerConnectionState
  loading: boolean
  error: string | null
  inbound: {
    framesDecoded: number
    framesDropped: number
    packetsLost: number
    jitterMs: number
    bytesReceived: number
    frameWidth: number
    frameHeight: number
  }
  renderer: {
    processorFrames: number
    renderedFrames: number
    queuedDrops: number
    lastRenderAgeMs: number | null
  }
}

const vertexShaderSource = `
  attribute vec2 a_position;
  attribute vec2 a_texCoord;
  varying vec2 v_texCoord;

  void main() {
    gl_Position = vec4(a_position, 0.0, 1.0);
    v_texCoord = a_texCoord;
  }
`

const fragmentShaderSource = `
  precision mediump float;
  varying vec2 v_texCoord;
  uniform sampler2D u_texture;

  void main() {
    gl_FragColor = texture2D(u_texture, v_texCoord);
  }
`

function compileShader(gl: WebGLRenderingContext, type: number, source: string): WebGLShader | null {
  const shader = gl.createShader(type)
  if (!shader) {
    return null
  }
  gl.shaderSource(shader, source)
  gl.compileShader(shader)
  if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
    console.error('[VideoTile] Shader compile error:', gl.getShaderInfoLog(shader))
    gl.deleteShader(shader)
    return null
  }
  return shader
}

function createProgram(gl: WebGLRenderingContext, vertexSource: string, fragmentSource: string): WebGLProgram | null {
  const vertexShader = compileShader(gl, gl.VERTEX_SHADER, vertexSource)
  const fragmentShader = compileShader(gl, gl.FRAGMENT_SHADER, fragmentSource)
  if (!vertexShader || !fragmentShader) {
    return null
  }

  const program = gl.createProgram()
  if (!program) {
    return null
  }

  gl.attachShader(program, vertexShader)
  gl.attachShader(program, fragmentShader)
  gl.linkProgram(program)

  if (!gl.getProgramParameter(program, gl.LINK_STATUS)) {
    console.error('[VideoTile] Program link error:', gl.getProgramInfoLog(program))
    gl.deleteProgram(program)
    return null
  }

  return program
}

export default function VideoTile(props: Props) {
  let containerRef: HTMLDivElement | undefined
  let canvasRef: HTMLCanvasElement | undefined
  let audioRef: HTMLAudioElement | undefined
  let connectAttempt = 0
  let renderGeneration = 0
  let frameReader: ReadableStreamDefaultReader<any> | null = null
  let pendingFrame: any | null = null
  let renderLoopId: number | null = null
  let renderLoopRunning = false
  let statusTimer: number | null = null
  let resizeObserver: ResizeObserver | null = null
  let webgl: WebGLState | null = null
  let textureWidth = 0
  let textureHeight = 0
  let processorFrameCount = 0
  let renderedFrameCount = 0
  let queuedDropCount = 0
  let lastRenderAtMs = 0

  const [pc, setPc] = createSignal<RTCPeerConnection | null>(null)
  const [loading, setLoading] = createSignal(true)
  const [error, setError] = createSignal<string | null>(null)
  const [connected, setConnected] = createSignal(false)

  const stopStatusTimer = () => {
    if (statusTimer !== null) {
      clearInterval(statusTimer)
      statusTimer = null
    }
  }

  const collectAndEmitStatus = async (connection: RTCPeerConnection | null, attempt: number) => {
    if (!connection || attempt !== connectAttempt) {
      return
    }

    const status: VideoTileStatus = {
      updatedAtMs: Date.now(),
      iceConnectionState: connection.iceConnectionState,
      peerConnectionState: connection.connectionState,
      loading: loading(),
      error: error(),
      inbound: {
        framesDecoded: 0,
        framesDropped: 0,
        packetsLost: 0,
        jitterMs: 0,
        bytesReceived: 0,
        frameWidth: 0,
        frameHeight: 0,
      },
      renderer: {
        processorFrames: processorFrameCount,
        renderedFrames: renderedFrameCount,
        queuedDrops: queuedDropCount,
        lastRenderAgeMs: lastRenderAtMs > 0 ? Math.max(0, Date.now() - lastRenderAtMs) : null,
      },
    }

    try {
      const stats = await connection.getStats()
      stats.forEach((report) => {
        const r = report as any
        if (r.type === 'inbound-rtp' && r.kind === 'video') {
          if (typeof r.framesDecoded === 'number') status.inbound.framesDecoded = r.framesDecoded
          if (typeof r.framesDropped === 'number') status.inbound.framesDropped = r.framesDropped
          if (typeof r.packetsLost === 'number') status.inbound.packetsLost = r.packetsLost
          if (typeof r.jitter === 'number') status.inbound.jitterMs = r.jitter * 1000
          if (typeof r.bytesReceived === 'number') status.inbound.bytesReceived = r.bytesReceived
        }
        if (r.type === 'track' && r.kind === 'video') {
          if (typeof r.frameWidth === 'number') status.inbound.frameWidth = r.frameWidth
          if (typeof r.frameHeight === 'number') status.inbound.frameHeight = r.frameHeight
        }
      })
    } catch {
      // keep lightweight status even when stats aren't available
    }

    props.onStatus?.(status)
  }

  const closeFrame = (frame: any | null) => {
    if (frame && typeof frame.close === 'function') {
      frame.close()
    }
  }

  const stopRenderLoop = () => {
    renderLoopRunning = false
    if (renderLoopId !== null) {
      cancelAnimationFrame(renderLoopId)
      renderLoopId = null
    }
    if (pendingFrame) {
      closeFrame(pendingFrame)
      pendingFrame = null
    }
  }

  const stopFramePump = () => {
    renderGeneration += 1
    const reader = frameReader
    frameReader = null
    stopRenderLoop()
    if (reader) {
      void reader.cancel().catch(() => {})
      try {
        reader.releaseLock()
      } catch {
        // ignore
      }
    }
  }

  const clearCanvas = () => {
    if (!canvasRef || !webgl) {
      return
    }
    const { gl } = webgl
    gl.viewport(0, 0, canvasRef.width, canvasRef.height)
    gl.clearColor(0, 0, 0, 1)
    gl.clear(gl.COLOR_BUFFER_BIT)
  }

  const resizeCanvasToContainer = () => {
    if (!containerRef || !canvasRef) {
      return
    }
    const rect = containerRef.getBoundingClientRect()
    if (rect.width <= 0 || rect.height <= 0) {
      return
    }

    const dpr = Math.max(1, window.devicePixelRatio || 1)
    const width = Math.max(1, Math.round(rect.width * dpr))
    const height = Math.max(1, Math.round(rect.height * dpr))

    if (canvasRef.width !== width || canvasRef.height !== height) {
      canvasRef.width = width
      canvasRef.height = height
      clearCanvas()
    }
  }

  const initRenderer = (): boolean => {
    if (!canvasRef) {
      return false
    }
    if (webgl) {
      return true
    }

    const gl = canvasRef.getContext('webgl', {
      alpha: false,
      antialias: false,
      preserveDrawingBuffer: false,
    })
    if (!gl) {
      setError('WebGL is not available in this browser')
      setLoading(false)
      return false
    }

    const program = createProgram(gl, vertexShaderSource, fragmentShaderSource)
    if (!program) {
      setError('Failed to initialize WebGL renderer')
      setLoading(false)
      return false
    }

    const positionBuffer = gl.createBuffer()
    const texCoordBuffer = gl.createBuffer()
    const texture = gl.createTexture()
    if (!positionBuffer || !texCoordBuffer || !texture) {
      setError('Failed to initialize WebGL buffers')
      setLoading(false)
      return false
    }

    const positionLocation = gl.getAttribLocation(program, 'a_position')
    const texCoordLocation = gl.getAttribLocation(program, 'a_texCoord')
    const textureLocation = gl.getUniformLocation(program, 'u_texture')
    if (positionLocation < 0 || texCoordLocation < 0 || !textureLocation) {
      setError('Failed to initialize WebGL shader attributes')
      setLoading(false)
      return false
    }

    const positions = new Float32Array([
      -1, -1,
       1, -1,
      -1,  1,
      -1,  1,
       1, -1,
       1,  1,
    ])
    const texCoords = new Float32Array([
      0, 1,
      1, 1,
      0, 0,
      0, 0,
      1, 1,
      1, 0,
    ])

    gl.useProgram(program)

    gl.bindBuffer(gl.ARRAY_BUFFER, positionBuffer)
    gl.bufferData(gl.ARRAY_BUFFER, positions, gl.STATIC_DRAW)
    gl.enableVertexAttribArray(positionLocation)
    gl.vertexAttribPointer(positionLocation, 2, gl.FLOAT, false, 0, 0)

    gl.bindBuffer(gl.ARRAY_BUFFER, texCoordBuffer)
    gl.bufferData(gl.ARRAY_BUFFER, texCoords, gl.STATIC_DRAW)
    gl.enableVertexAttribArray(texCoordLocation)
    gl.vertexAttribPointer(texCoordLocation, 2, gl.FLOAT, false, 0, 0)

    gl.activeTexture(gl.TEXTURE0)
    gl.bindTexture(gl.TEXTURE_2D, texture)
    gl.uniform1i(textureLocation, 0)
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)

    webgl = { gl, program, texture }
    clearCanvas()
    return true
  }

  const setContainViewport = (frameWidth: number, frameHeight: number) => {
    if (!canvasRef || !webgl) {
      return
    }
    const { gl } = webgl
    const canvasWidth = canvasRef.width
    const canvasHeight = canvasRef.height
    const frameAspect = frameWidth / frameHeight
    const canvasAspect = canvasWidth / canvasHeight

    let drawWidth = canvasWidth
    let drawHeight = canvasHeight

    if (frameAspect > canvasAspect) {
      drawHeight = Math.round(canvasWidth / frameAspect)
    } else {
      drawWidth = Math.round(canvasHeight * frameAspect)
    }

    const offsetX = Math.max(0, Math.floor((canvasWidth - drawWidth) / 2))
    const offsetY = Math.max(0, Math.floor((canvasHeight - drawHeight) / 2))
    gl.viewport(offsetX, offsetY, Math.max(1, drawWidth), Math.max(1, drawHeight))
  }

  const renderFrame = (frame: any): boolean => {
    if (!initRenderer() || !canvasRef || !webgl) {
      return false
    }

    const frameWidth = Number(frame?.displayWidth || frame?.codedWidth || frame?.width || 0)
    const frameHeight = Number(frame?.displayHeight || frame?.codedHeight || frame?.height || 0)
    if (frameWidth <= 0 || frameHeight <= 0) {
      return false
    }

    resizeCanvasToContainer()

    const { gl, texture } = webgl
    gl.useProgram(webgl.program)
    gl.bindTexture(gl.TEXTURE_2D, texture)

    const texSource = frame as TexImageSource
    if (frameWidth === textureWidth && frameHeight === textureHeight) {
      try {
        gl.texSubImage2D(gl.TEXTURE_2D, 0, 0, 0, gl.RGBA, gl.UNSIGNED_BYTE, texSource)
      } catch {
        gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, texSource)
      }
    } else {
      gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, texSource)
      textureWidth = frameWidth
      textureHeight = frameHeight
    }

    gl.viewport(0, 0, canvasRef.width, canvasRef.height)
    gl.clearColor(0, 0, 0, 1)
    gl.clear(gl.COLOR_BUFFER_BIT)
    setContainViewport(frameWidth, frameHeight)
    gl.drawArrays(gl.TRIANGLES, 0, 6)
    renderedFrameCount += 1
    lastRenderAtMs = Date.now()
    return true
  }

  const startVideoFramePump = async (track: MediaStreamTrack, attempt: number) => {
    const processorCtor = (
      window as unknown as {
        MediaStreamTrackProcessor?: new (opts: {
          track: MediaStreamTrack
        }) => { readable: ReadableStream<any> }
      }
    ).MediaStreamTrackProcessor

    if (!processorCtor) {
      setError('MediaStreamTrackProcessor is unavailable in this browser')
      setLoading(false)
      return
    }

    const generation = ++renderGeneration
    try {
      const processor = new processorCtor({ track })
      const reader = processor.readable.getReader()
      frameReader = reader

      const renderTick = () => {
        if (generation !== renderGeneration || attempt !== connectAttempt) {
          renderLoopRunning = false
          renderLoopId = null
          return
        }

        const frame = pendingFrame
        pendingFrame = null
        if (frame) {
          try {
            if (renderFrame(frame)) {
              setLoading(false)
            }
          } finally {
            closeFrame(frame)
          }
        }

        renderLoopId = requestAnimationFrame(renderTick)
      }

      stopRenderLoop()
      renderLoopRunning = true
      renderLoopId = requestAnimationFrame(renderTick)

      while (generation === renderGeneration && attempt === connectAttempt) {
        const result = await reader.read()
        if (result.done) {
          break
        }

        const frame = result.value
        processorFrameCount += 1
        if (pendingFrame) {
          closeFrame(pendingFrame)
          queuedDropCount += 1
        }
        pendingFrame = frame
      }
    } catch (err) {
      if (generation === renderGeneration && attempt === connectAttempt) {
        console.error('[VideoTile] Frame pump error:', err)
        setError(err instanceof Error ? err.message : 'Video frame pipeline failed')
        setLoading(false)
      }
    } finally {
      if (generation === renderGeneration) {
        stopRenderLoop()
      }
    }
  }

  const disconnect = () => {
    stopStatusTimer()
    const connection = pc()
    if (connection) {
      console.log('[VideoTile] Cleaning up connection')
      connection.close()
      setPc(null)
    }

    stopFramePump()
    textureWidth = 0
    textureHeight = 0
    clearCanvas()

    if (audioRef) {
      audioRef.srcObject = null
    }

    setConnected(false)
    props.onStatus?.(null)
  }

  const connect = async (nodeId: string, serviceId: string, serviceUrl: string) => {
    const attempt = ++connectAttempt

    console.log('[VideoTile] Connecting to:', {
      nodeId,
      serviceId,
      serviceUrl,
    })

    setLoading(true)
    setError(null)
    setConnected(true)
    processorFrameCount = 0
    renderedFrameCount = 0
    queuedDropCount = 0
    lastRenderAtMs = 0

    disconnect()
    setConnected(true)

    try {
      const newPc = new RTCPeerConnection({
        iceServers: [{ urls: 'stun:stun.l.google.com:19302' }],
      })
      setPc(newPc)
      stopStatusTimer()
      void collectAndEmitStatus(newPc, attempt)
      statusTimer = window.setInterval(() => {
        void collectAndEmitStatus(newPc, attempt)
      }, 1000)

      newPc.ontrack = (event) => {
        if (attempt !== connectAttempt) {
          return
        }

        console.log(`[VideoTile] Got ${event.track.kind} track:`, event.track.label || event.track.id)

        if (event.track.kind === 'video') {
          stopFramePump()
          void startVideoFramePump(event.track, attempt)
          return
        }

        if (event.track.kind === 'audio' && audioRef) {
          if (event.streams && event.streams[0]) {
            audioRef.srcObject = event.streams[0]
          } else {
            const stream = new MediaStream([event.track])
            audioRef.srcObject = stream
          }
          audioRef.play().catch(() => {})
        }
      }

      newPc.oniceconnectionstatechange = () => {
        if (attempt !== connectAttempt) {
          return
        }

        console.log('[VideoTile] ICE connection state:', newPc.iceConnectionState)
        void collectAndEmitStatus(newPc, attempt)
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

      newPc.addTransceiver('video', { direction: 'recvonly' })
      newPc.addTransceiver('audio', { direction: 'recvonly' })

      const offer = await newPc.createOffer()
      await newPc.setLocalDescription(offer)

      if (attempt !== connectAttempt) {
        newPc.close()
        return
      }

      console.log('[VideoTile] Sending WebRTC session request...')
      const response = await webrtcClient.createWebRTCSession({
        nodeId,
        serviceId,
        serviceUrl,
        sdpOffer: newPc.localDescription?.sdp || '',
      })

      if (attempt !== connectAttempt) {
        newPc.close()
        return
      }

      console.log('[VideoTile] Got session response, session ID:', response.sessionId)
      await newPc.setRemoteDescription(
        new RTCSessionDescription({
          type: 'answer',
          sdp: response.sdpAnswer,
        })
      )

      if (attempt !== connectAttempt) {
        newPc.close()
        return
      }

      console.log('[VideoTile] WebRTC session established')
      void collectAndEmitStatus(newPc, attempt)
    } catch (err) {
      if (attempt !== connectAttempt) {
        return
      }

      console.error('[VideoTile] Connection error:', err)
      setError(err instanceof Error ? err.message : 'Connection failed')
      setLoading(false)
      setConnected(false)
      const livePc = pc()
      void collectAndEmitStatus(livePc, attempt)
    }
  }

  createEffect(() => {
    if (!containerRef || !canvasRef) {
      return
    }
    resizeCanvasToContainer()
    if (resizeObserver) {
      resizeObserver.disconnect()
    }
    resizeObserver = new ResizeObserver(() => {
      resizeCanvasToContainer()
    })
    resizeObserver.observe(containerRef)
  })

  createEffect(() => {
    const nodeId = props.nodeId
    const serviceId = props.serviceId
    const serviceUrl = props.serviceUrl

    untrack(() => {
      void connect(nodeId, serviceId, serviceUrl)
    })
  })

  onCleanup(() => {
    connectAttempt += 1
    stopStatusTimer()
    disconnect()
    if (resizeObserver) {
      resizeObserver.disconnect()
      resizeObserver = null
    }
  })

  return (
    <div ref={containerRef} class="relative w-full h-full bg-neu-900 overflow-hidden">
      <canvas ref={canvasRef} class="w-full h-full block" />
      <audio ref={audioRef} autoplay class="hidden" />

      <Show when={loading()}>
        <div class="absolute inset-0 flex items-center justify-center bg-neu-950/50">
          <div class="flex flex-col items-center gap-3">
            <div class="w-12 h-12 border-4 border-neu-700 border-t-neu-400 rounded-full animate-spin" />
            <div class="text-neu-300 text-sm">Connecting...</div>
          </div>
        </div>
      </Show>

      <Show when={error()}>
        <div class="absolute inset-0 flex items-center justify-center bg-neu-950/70">
          <div class="flex flex-col items-center gap-3 px-6">
            <div class="text-neu-400 text-lg">⚠️</div>
            <div class="text-neu-300 text-sm text-center">{error()}</div>
            <button
              onClick={() => {
                setConnected(false)
                void connect(props.nodeId, props.serviceId, props.serviceUrl)
              }}
              class="px-4 py-2 bg-neu-700 hover:bg-neu-600 text-neu-100 text-sm rounded-md transition-colors"
            >
              Retry
            </button>
          </div>
        </div>
      </Show>

      <Show when={props.name}>
        <div class="absolute bottom-0 left-0 right-0 px-4 py-2 bg-gradient-to-t from-neu-900/80 to-transparent">
          <div class="text-neu-100 text-sm font-medium">{props.name}</div>
        </div>
      </Show>
    </div>
  )
}
