package relay

import (
	"log"
	"net/http"

	"github.com/zapdos-labs/unblink/relay/cv"
)

// StartWorkerAPIServer starts the worker API server on a separate port
func StartWorkerAPIServer(port string, registry *cv.CVWorkerRegistry, storage *cv.StorageManager) error {
	mux := http.NewServeMux()

	// WebSocket endpoint for worker connections
	mux.HandleFunc("/connect", registry.HandleWebSocket)

	// Frame download endpoint
	mux.HandleFunc("/frames/", storage.HandleFrameDownload)

	// Worker event publishing endpoint
	mux.HandleFunc("/events", registry.HandleEventAPI)

	log.Printf("[WorkerAPI] Starting worker API server on port %s", port)

	return http.ListenAndServe(":"+port, mux)
}
