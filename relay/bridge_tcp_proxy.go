package relay

import (
	"io"
	"log"
	"net"
	"sync"

	"github.com/unblink/unblink/node"
)

// BridgeTCPProxy creates a local TCP listener that proxies traffic through the relay bridge.
// This allows go2rtc to connect to localhost:port and have traffic forwarded through the bridge.
type BridgeTCPProxy struct {
	listener net.Listener
	nodeConn *NodeConn
	bridgeID string
	service  node.Service
	shutdown chan struct{}
	wg       sync.WaitGroup
}

// NewBridgeTCPProxy creates a new bridge TCP proxy with a registered data channel
func NewBridgeTCPProxy(nc *NodeConn, bridgeID string, service node.Service) (*BridgeTCPProxy, error) {
	// Create buffered channel for receiving data
	dataChan := make(chan []byte, 1000) // Buffer to prevent blocking

	// Register the channel with NodeConn
	nc.RegisterBridgeChan(bridgeID, dataChan)

	// Listen on a random local port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		nc.UnregisterBridgeChan(bridgeID)
		return nil, err
	}

	bp := &BridgeTCPProxy{
		listener: listener,
		nodeConn: nc,
		bridgeID: bridgeID,
		service:  service,
		shutdown: make(chan struct{}),
	}

	// Start accepting connections
	bp.wg.Add(1)
	go bp.acceptLoop(dataChan)

	return bp, nil
}

// Addr returns the local address of the proxy
func (bp *BridgeTCPProxy) Addr() string {
	return bp.listener.Addr().String()
}

// Close shuts down the proxy and closes the bridge on the node
func (bp *BridgeTCPProxy) Close() {
	close(bp.shutdown)
	bp.listener.Close()
	bp.nodeConn.UnregisterBridgeChan(bp.bridgeID)
	bp.nodeConn.CloseBridge(bp.bridgeID)
	bp.wg.Wait()
}

// acceptLoop accepts connections and starts proxying
func (bp *BridgeTCPProxy) acceptLoop(dataChan <-chan []byte) {
	defer bp.wg.Done()

	for {
		conn, err := bp.listener.Accept()
		if err != nil {
			select {
			case <-bp.shutdown:
				return
			default:
				log.Printf("[BridgeTCPProxy] Accept error: %v", err)
				continue
			}
		}

		bp.wg.Add(1)
		go bp.handleConn(conn, dataChan)
	}
}

// handleConn proxies data between the local connection and the bridge
func (bp *BridgeTCPProxy) handleConn(conn net.Conn, dataChan <-chan []byte) {
	defer bp.wg.Done()
	defer conn.Close()

	log.Printf("[BridgeTCPProxy] New connection from %s", conn.RemoteAddr())

	// Goroutine to read from local conn and send to bridge
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 4096)
		for {
			n, err := conn.Read(buf)
			if err != nil {
				if err != io.EOF {
					log.Printf("[BridgeTCPProxy] Read error: %v", err)
				}
				return
			}

			if n > 0 {
				if err := bp.nodeConn.SendData(bp.bridgeID, buf[:n]); err != nil {
					log.Printf("[BridgeTCPProxy] Send to bridge error: %v", err)
					return
				}
			}
		}
	}()

	// Main loop: read from bridge channel and write to local conn
	writeCount := 0
	for {
		select {
		case <-bp.shutdown:
			log.Printf("[BridgeTCPProxy] Exiting (shutdown)")
			return
		case <-done:
			log.Printf("[BridgeTCPProxy] Exiting (done), wrote %d packets", writeCount)
			return
		case data, ok := <-dataChan:
			if !ok {
				log.Printf("[BridgeTCPProxy] Channel closed, wrote %d packets", writeCount)
				return
			}
			n, err := conn.Write(data)
			if err != nil {
				log.Printf("[BridgeTCPProxy] Write error: %v", err)
				return
			}
			writeCount++
			if writeCount <= 5 {
				log.Printf("[BridgeTCPProxy] Wrote packet #%d: %d bytes", writeCount, n)
			}
			if n != len(data) {
				log.Printf("[BridgeTCPProxy] Short write: %d/%d bytes", n, len(data))
			}
		}
	}
}
