package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/zapdos-labs/unblink/database"
	"github.com/zapdos-labs/unblink/server"
	"github.com/zapdos-labs/unblink/server/auth"
	"github.com/zapdos-labs/unblink/server/chat"
	"github.com/zapdos-labs/unblink/server/chat/tools"
	"github.com/zapdos-labs/unblink/server/gen/chat/v1/auth/authv1connect"
	"github.com/zapdos-labs/unblink/server/gen/chat/v1/chatv1connect"
	"github.com/zapdos-labs/unblink/server/gen/service/v1/servicev1connect"
	"github.com/zapdos-labs/unblink/server/gen/webrtc/v1/webrtcv1connect"
	"github.com/zapdos-labs/unblink/server/service"
	"github.com/zapdos-labs/unblink/server/webrtc"

	"connectrpc.com/connect"
)

func main() {
	log.Printf("Loading configuration from environment variables")
	config, err := server.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config from environment: %v", err)
	}

	// Initialize database
	dbClient, err := database.NewClient(database.Config{DatabaseURL: config.DatabaseURL})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer dbClient.Close()

	// Create schema if needed (idempotent - safe to run multiple times)
	if err := dbClient.EnsureSchema(); err != nil {
		log.Fatalf("Failed to ensure schema: %v", err)
	}

	// Initialize chat service
	chatCfg := &chat.Config{
		ChatOpenAIModel:         config.ChatOpenAIModel,
		ChatOpenAIBaseURL:       config.ChatOpenAIBaseURL,
		ChatOpenAIAPIKey:        config.ChatOpenAIAPIKey,
		ChatMaxTokens:           config.ChatMaxTokens,
		VLMOpenAIModel:          config.VLMOpenAIModel,
		VLMOpenAIBaseURL:        config.VLMOpenAIBaseURL,
		VLMOpenAIAPIKey:         config.VLMOpenAIAPIKey,
		ContentTrimSafetyMargin: config.ContentTrimSafetyMargin,
	}

	// Default safety margin to 10% if not set
	if chatCfg.ContentTrimSafetyMargin == 0 {
		chatCfg.ContentTrimSafetyMargin = 10
	}
	chatService := chat.NewService(dbClient, chatCfg)

	// Register camera search tool
	cameraSearchTool := tools.NewCameraSearchTool(dbClient)
	chatService.RegisterTool(cameraSearchTool)
	chat.RegisterSetCharacterTool(chatService)

	// Initialize JWT manager and auth interceptor
	jwtManager := server.NewJWTManager(config.JWTSecret)
	authInterceptor := server.NewAuthInterceptor(jwtManager, dbClient)
	log.Printf("Initialized auth interceptor")

	// Create storage for frames
	storage := webrtc.NewStorage(config.FramesBaseDir())
	log.Printf("[Main] Initialized storage: baseDir=%s", config.FramesBaseDir())

	// Create event service BEFORE batch manager (batch manager needs the broadcaster)
	eventService := service.NewEventService(dbClient)

	// Create VLM frame client and batch manager (for VLM summarization)
	vlmTimeout := time.Duration(config.VLMTimeoutSec) * time.Second
	frameClient := webrtc.NewFrameClient(config.VLMOpenAIBaseURL, config.VLMOpenAIModel, config.VLMOpenAIAPIKey, vlmTimeout, "Summarize the video")
	batchManager := webrtc.NewBatchManager(frameClient, config.FrameBatchSize, storage, dbClient, eventService.GetBroadcaster())
	log.Printf("[Main] Initialized VLM frame client: url=%s, model=%s, batchSize=%d, timeout=%vs", config.VLMOpenAIBaseURL, config.VLMOpenAIModel, config.FrameBatchSize, config.VLMTimeoutSec)

	// Initialize node server for WebSocket connections
	nodeServer := server.NewServer(config)

	// Create service registry for managing services
	frameInterval := time.Duration(config.FrameIntervalSeconds * float64(time.Second))
	idleTimeout := time.Duration(config.BridgeIdleTimeoutSec) * time.Second
	serviceRegistry := service.NewServiceRegistry(
		dbClient,
		frameInterval,
		storage,
		nodeServer,
		batchManager,
		idleTimeout,
		config.BridgeMaxRetries,
		config.EnableIndexing,
	)
	liveUpdateService := service.NewLiveUpdateService(dbClient, serviceRegistry)

	// Wire up node event callbacks
	nodeServer.OnNodeReady(serviceRegistry.SetNodeOnline)
	nodeServer.OnNodeOffline(serviceRegistry.SetNodeOffline)
	nodeServer.OnNodeReady(func(nodeID string) {
		liveUpdateService.BroadcastNodeStatus(nodeID, true)
	})
	nodeServer.OnNodeOffline(func(nodeID string) {
		liveUpdateService.BroadcastNodeStatus(nodeID, false)
	})

	// Load existing services from database
	if err := serviceRegistry.LoadServices(); err != nil {
		log.Printf("Failed to load services: %v", err)
	}

	serviceService := service.NewService(dbClient, serviceRegistry)

	// Create storage service
	storageService := service.NewStorageService(dbClient, &service.StorageConfig{
		StorageBaseDir: config.FramesBaseDir(),
	}, jwtManager)

	// Create HTTP handler with Connect RPC
	mux := http.NewServeMux()

	// Mount AuthService with auth interceptor
	// CreateGuestUser will be skipped in the interceptor
	authService := auth.NewService(dbClient, jwtManager)
	authPath, authHandler := authv1connect.NewAuthServiceHandler(
		authService,
		connect.WithInterceptors(authInterceptor),
	)
	mux.Handle(authPath, authHandler)
	log.Printf("Mounted AuthService at %s (with auth)", authPath)

	// Mount ChatService with auth interceptor
	chatPath, chatHandler := chatv1connect.NewChatServiceHandler(
		chatService,
		connect.WithInterceptors(authInterceptor),
	)
	mux.Handle(chatPath, chatHandler)
	log.Printf("Mounted ChatService at %s (with auth)", chatPath)

	// Mount ServiceService with auth interceptor
	servicePath, serviceHandler := servicev1connect.NewServiceServiceHandler(
		serviceService,
		connect.WithInterceptors(authInterceptor),
	)
	mux.Handle(servicePath, serviceHandler)
	log.Printf("Mounted ServiceService at %s (with auth)", servicePath)

	// Mount StorageService with auth interceptor
	framesPath, framesHandler := servicev1connect.NewStorageServiceHandler(
		storageService,
		connect.WithInterceptors(authInterceptor),
	)
	mux.Handle(framesPath, framesHandler)
	log.Printf("Mounted StorageService at %s (with auth)", framesPath)

	// Mount EventService with auth interceptor
	eventPath, eventHandler := servicev1connect.NewEventServiceHandler(
		eventService,
		connect.WithInterceptors(authInterceptor),
	)
	mux.Handle(eventPath, eventHandler)
	log.Printf("Mounted EventService at %s (with auth)", eventPath)

	liveUpdatePath, liveUpdateHandler := servicev1connect.NewLiveUpdateServiceHandler(
		liveUpdateService,
		connect.WithInterceptors(authInterceptor),
	)
	mux.Handle(liveUpdatePath, liveUpdateHandler)
	log.Printf("Mounted LiveUpdateService at %s (with auth)", liveUpdatePath)

	// Mount WebRTCService with auth interceptor
	webrtcService := webrtc.NewService(nodeServer, dbClient)
	webrtcPath, webrtcHandler := webrtcv1connect.NewWebRTCServiceHandler(
		webrtcService,
		connect.WithInterceptors(authInterceptor),
	)
	mux.Handle(webrtcPath, webrtcHandler)
	log.Printf("Mounted WebRTCService at %s (with auth)", webrtcPath)

	// Add CORS middleware
	apiHandler := withCORS(mux)

	// Create root mux for routing
	rootMux := http.NewServeMux()

	// Mount API routes under /api/ (StripPrefix removes /api before passing to handler)
	rootMux.Handle("/api/", http.StripPrefix("/api", apiHandler))

	// Node WebSocket endpoint (stay at root level for external connections)
	rootMux.HandleFunc("/node/connect", nodeServer.HandleNodeConnect)

	// Health check endpoint (stay at root level)
	rootMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	})

	// Register HTTP handlers for serving JPEG frames (stay at root level for direct access)
	storageService.RegisterHTTPHandlers(rootMux)

	// Serve frontend from dist directory if configured
	if config.DistPath != "" {
		fs := http.FileServer(http.Dir(config.DistPath))
		rootMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/" || r.URL.Path == "/index.html" || r.URL.Path == "/node" || strings.HasPrefix(r.URL.Path, "/node/") {
				serveInjectedIndex(w, config.DistPath)
				return
			}
			fs.ServeHTTP(w, r)
		})
		log.Printf("Serving frontend from: %s", config.DistPath)
	}

	// Start server
	log.Printf("Server starting on %s", config.ListenAddr)
	log.Printf("  - API: /api/*")
	log.Printf("  - Node WebSocket: /node/connect")
	log.Printf("  - Health: /health")
	log.Printf("  - Storage HTTP: /storage/{itemID}")

	// Use h2c for HTTP/2 without TLS
	h2s := &http2.Server{}
	srv := &http.Server{
		Addr:    config.ListenAddr,
		Handler: h2c.NewHandler(rootMux, h2s),
	}

	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// withCORS adds CORS headers to the response
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Connect-Protocol-Version, Connect-Timeout-Ms, X-Grpc-Web, X-User-Agent, Authorization")
		w.Header().Set("Access-Control-Expose-Headers", "Grpc-Status, Grpc-Message, Content-Encoding")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func serveInjectedIndex(w http.ResponseWriter, distPath string) {
	indexPath := filepath.Join(distPath, "index.html")
	content, err := os.ReadFile(indexPath)
	if err != nil {
		http.Error(w, "index.html not found", http.StatusNotFound)
		return
	}

	script := `<script>window.SERVER_META = {"servedBy":"go"};</script>`
	html := string(content)
	if idx := strings.Index(html, "</head>"); idx != -1 {
		html = html[:idx] + script + html[idx:]
	} else if idx := strings.Index(html, "</body>"); idx != -1 {
		html = html[:idx] + script + html[idx:]
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}
