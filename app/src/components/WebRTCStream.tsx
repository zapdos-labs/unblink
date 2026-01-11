import { createSignal, onMount, onCleanup, For, Show, untrack, type Component } from 'solid-js';

interface ServiceInfo {
  id: string;
  type: string;
  node_id: string;
  addr: string;
  port: number;
  path: string;
}

interface Props {
  nodeId: string;
  serviceId?: string;
}

const WebRTCStream: Component<Props> = (props) => {
  const [videoRef, setVideoRef] = createSignal<HTMLVideoElement>();
  const [status, setStatus] = createSignal('');
  const [selectedServiceId, setSelectedServiceId] = createSignal('');
  const [services, setServices] = createSignal<ServiceInfo[]>([]);
  const [pc, setPc] = createSignal<RTCPeerConnection>();
  const [statsInterval, setStatsInterval] = createSignal<number>();
  const [nodeNotFound, setNodeNotFound] = createSignal(false);
  const [loading, setLoading] = createSignal(true);

  onMount(async () => {
    // Use provided serviceId or fetch available services
    if (props.serviceId) {
      setSelectedServiceId(props.serviceId);
      setLoading(false);
      return;
    }

    try {
      const response = await fetch(`/relay/node/${props.nodeId}/services`);
      if (response.status === 404) {
        setNodeNotFound(true);
        setLoading(false);
        return;
      }
      const data = await response.json();
      setServices(data);
      if (data.length > 0) {
        setSelectedServiceId(data[0].id);
      }
    } catch (e) {
      console.error('Failed to fetch services:', e);
    }
    setLoading(false);
  });

  onCleanup(() => {
    const interval = untrack(statsInterval);
    if (interval) {
      clearInterval(interval);
    }
    const peerConnection = untrack(pc);
    if (peerConnection) {
      peerConnection.close();
    }
  });

  const startSession = async () => {
    const videoEl = untrack(videoRef);
    if (!videoEl) {
      setStatus('Error: Video element not found');
      return;
    }

    setStatus('Creating PeerConnection...');

    const newPc = new RTCPeerConnection({
      iceServers: [{
        urls: 'stun:stun.l.google.com:19302'
      }]
    });

    newPc.ontrack = (event) => {
      const track = event.track;
      console.log(`Got ${track.kind} track:`, track.label || track.id);
      videoEl.srcObject = event.streams[0];
    };

    newPc.oniceconnectionstatechange = () => {
      console.log('ICE state:', newPc.iceConnectionState);
      setStatus('ICE state: ' + newPc.iceConnectionState);
    };

    // Add transceivers for video and audio
    newPc.addTransceiver('video', { direction: 'recvonly' });
    newPc.addTransceiver('audio', { direction: 'recvonly' });

    const offer = await newPc.createOffer();
    await newPc.setLocalDescription(offer);

    setStatus('Sending offer...');

    try {
      const response = await fetch(`/relay/node/${props.nodeId}/offer`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          sdp: newPc.localDescription?.sdp,
          serviceId: untrack(selectedServiceId)
        })
      });

      const answer = await response.json();
      setStatus('Received answer. Setting remote description...');
      await newPc.setRemoteDescription(new RTCSessionDescription({
        type: 'answer',
        sdp: answer.sdp,
      }));
      setPc(newPc);

      // Start logging stats
      const interval = setInterval(async () => {
        const stats = await newPc.getStats();
        stats.forEach(report => {
          if (report.type === 'inbound-rtp' && report.kind === 'video') {
            console.log(`[Stats] At: ${new Date().toISOString()}, Jitter: ${report.jitter}, Packets: ${report.packetsReceived}, Frames Decoded: ${report.framesDecoded}, KeyFrames: ${report.keyFramesDecoded}`);
          }
        });
        // Debug video element state
        const v = untrack(videoRef);
        if (v) {
          console.log(`[Video] paused=${v.paused}, readyState=${v.readyState}, videoWidth=${v.videoWidth}x${v.videoHeight}, currentTime=${v.currentTime.toFixed(2)}`);
        }
      }, 1000);
      setStatsInterval(interval as any);

    } catch (e: any) {
      console.error(e);
      setStatus('Error: ' + e.message);
    }
  };

  return (
    <Show
      when={nodeNotFound()}
      fallback={
        <Show
          when={!loading()}
          fallback={
            <div class="p-8">
              <p>Loading node information...</p>
            </div>
          }
        >
          <div class="p-8">
            <h1 class="text-3xl font-bold mb-6">Node Dashboard</h1>

            <div class="mb-4 text-sm text-gray-500">
              Node: <code class="bg-gray-100 px-2 py-1 rounded">{props.nodeId}</code>
            </div>

            <div class="mb-4">
              <video
                ref={setVideoRef}
                autoplay
                playsinline
                controls
                class="w-full max-w-4xl bg-black"
              />
            </div>

            <div class="flex gap-4 items-center mb-4">
              <select
                value={selectedServiceId()}
                onChange={(e) => setSelectedServiceId(e.currentTarget.value)}
                class="px-3 py-2 border rounded min-w-48"
              >
                <For each={services()}>
                  {(service) => (
                    <option value={service.id}>
                      {service.id} ({service.type})
                    </option>
                  )}
                </For>
              </select>
              <button
                onClick={startSession}
                class="px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600"
              >
                Start Stream
              </button>
            </div>

            <div id="status" class="text-lg">
              {status()}
            </div>

            <div class="mt-4 text-sm text-gray-500">
              Available services: {services().length}
            </div>
          </div>
        </Show>
      }
    >
      <div class="p-8">
        <h1 class="text-3xl font-bold mb-6 text-red-600">Node Not Found</h1>
        <p class="text-gray-700 mb-4">
          No such node: <code class="bg-gray-100 px-2 py-1 rounded">{props.nodeId}</code>
        </p>
        <p class="text-gray-600">
          Please check the dashboard link again or verify the node is registered.
        </p>
        <a href="/" class="inline-block mt-4 px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600">
          Back to Dashboard
        </a>
      </div>
    </Show>
  );
};

export default WebRTCStream;
