package server

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

// BridgeConn implements net.Conn for bridge connections
// This allows RTSP clients and other protocols to work seamlessly over bridges
type BridgeConn struct {
	nodeConn  *NodeConn
	bridgeID  string
	dataChan  chan []byte
	closeOnce sync.Once
	closed    bool
	closeMu   sync.Mutex

	// Read buffer for partial reads
	readBuf       []byte
	readBufMu     sync.Mutex
	readDeadline  time.Time
	writeDeadline time.Time
	deadlineMu    sync.RWMutex

	// Idle detection
	lastDataTime time.Time
	idleMu       sync.RWMutex
}

// NewBridgeConn creates a new BridgeConn
func NewBridgeConn(nodeConn *NodeConn, bridgeID string, dataChan chan []byte) *BridgeConn {
	return &BridgeConn{
		nodeConn:     nodeConn,
		bridgeID:     bridgeID,
		dataChan:     dataChan,
		lastDataTime: time.Now(), // Initialize with current time
	}
}

// Read implements net.Conn.Read
func (bc *BridgeConn) Read(p []byte) (int, error) {
	bc.closeMu.Lock()
	if bc.closed {
		bc.closeMu.Unlock()
		return 0, io.EOF
	}
	bc.closeMu.Unlock()

	bc.readBufMu.Lock()
	defer bc.readBufMu.Unlock()

	// If we have buffered data from a previous read, use it first
	if len(bc.readBuf) > 0 {
		n := copy(p, bc.readBuf)
		bc.readBuf = bc.readBuf[n:]
		return n, nil
	}

	// Read new data from channel
	bc.deadlineMu.RLock()
	deadline := bc.readDeadline
	bc.deadlineMu.RUnlock()

	var timer *time.Timer
	var timerCh <-chan time.Time

	if !deadline.IsZero() {
		timeout := time.Until(deadline)
		if timeout <= 0 {
			return 0, fmt.Errorf("read deadline exceeded")
		}
		timer = time.NewTimer(timeout)
		timerCh = timer.C
		defer timer.Stop()
	}

	select {
	case data, ok := <-bc.dataChan:
		if !ok {
			return 0, io.EOF
		}

		// Update last data time
		bc.idleMu.Lock()
		bc.lastDataTime = time.Now()
		bc.idleMu.Unlock()

		// Copy what we can into p
		n := copy(p, data)

		// Save any remaining data for next read
		if n < len(data) {
			bc.readBuf = data[n:]
		}

		return n, nil

	case <-timerCh:
		return 0, fmt.Errorf("read deadline exceeded")
	}
}

// Write implements net.Conn.Write
func (bc *BridgeConn) Write(p []byte) (int, error) {
	bc.closeMu.Lock()
	if bc.closed {
		bc.closeMu.Unlock()
		return 0, fmt.Errorf("connection closed")
	}
	bc.closeMu.Unlock()

	bc.deadlineMu.RLock()
	deadline := bc.writeDeadline
	bc.deadlineMu.RUnlock()

	if !deadline.IsZero() && time.Now().After(deadline) {
		return 0, fmt.Errorf("write deadline exceeded")
	}

	// Send data through the bridge
	if err := bc.nodeConn.SendData(bc.bridgeID, p); err != nil {
		return 0, err
	}

	return len(p), nil
}

// Close implements net.Conn.Close
func (bc *BridgeConn) Close() error {
	var err error
	bc.closeOnce.Do(func() {
		bc.closeMu.Lock()
		bc.closed = true
		bc.closeMu.Unlock()

		log.Printf("[BridgeConn] Closing bridge %s", bc.bridgeID)
		// The data channel will be closed by NodeConn.CloseBridge
	})
	return err
}

// LocalAddr implements net.Conn.LocalAddr
func (bc *BridgeConn) LocalAddr() net.Addr {
	return &bridgeAddr{bridgeID: bc.bridgeID, local: true}
}

// RemoteAddr implements net.Conn.RemoteAddr
func (bc *BridgeConn) RemoteAddr() net.Addr {
	return &bridgeAddr{bridgeID: bc.bridgeID, local: false}
}

// SetDeadline implements net.Conn.SetDeadline
func (bc *BridgeConn) SetDeadline(t time.Time) error {
	bc.deadlineMu.Lock()
	defer bc.deadlineMu.Unlock()
	bc.readDeadline = t
	bc.writeDeadline = t
	return nil
}

// SetReadDeadline implements net.Conn.SetReadDeadline
func (bc *BridgeConn) SetReadDeadline(t time.Time) error {
	bc.deadlineMu.Lock()
	defer bc.deadlineMu.Unlock()
	bc.readDeadline = t
	return nil
}

// SetWriteDeadline implements net.Conn.SetWriteDeadline
func (bc *BridgeConn) SetWriteDeadline(t time.Time) error {
	bc.deadlineMu.Lock()
	defer bc.deadlineMu.Unlock()
	bc.writeDeadline = t
	return nil
}

// LastDataTime returns the last time data was received
func (bc *BridgeConn) LastDataTime() time.Time {
	bc.idleMu.RLock()
	defer bc.idleMu.RUnlock()
	return bc.lastDataTime
}

// IdleDuration returns how long the connection has been idle
func (bc *BridgeConn) IdleDuration() time.Duration {
	bc.idleMu.RLock()
	defer bc.idleMu.RUnlock()
	return time.Since(bc.lastDataTime)
}

// bridgeAddr implements net.Addr for bridge connections
type bridgeAddr struct {
	bridgeID string
	local    bool
}

func (a *bridgeAddr) Network() string {
	return "bridge"
}

func (a *bridgeAddr) String() string {
	if a.local {
		return fmt.Sprintf("bridge-local:%s", a.bridgeID)
	}
	return fmt.Sprintf("bridge-remote:%s", a.bridgeID)
}
