package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"unblink/database"
	"unblink/server"
	"unblink/server/auth"
	"unblink/server/chat"
	"unblink/server/gen/chat/v1/auth/authv1connect"
	"unblink/server/gen/chat/v1/chatv1connect"
	"unblink/server/gen/service/v1/servicev1connect"
	"unblink/server/gen/webrtc/v1/webrtcv1connect"
	"unblink/server/models"
	"unblink/server/service"
	"unblink/server/webrtc"

	"connectrpc.com/connect"
	"github.com/go-gst/go-gst/gst"
)

func main() {
	// Define flags
	configPath := flag.String("config", "", "Path to config file (default: ~/.unblink/server.config.json)")

	// Parse flags
	flag.Parse()

	// Initialize GStreamer
	gst.Init(nil)

	// Load configuration from file (all fields required)
	config, err := server.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database
	dbClient, err := database.NewClient(database.Config{DatabaseURL: config.DatabaseURL})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer dbClient.Close()

	// Create schema if needed
	if err := dbClient.CreateSchema(); err != nil {
		log.Fatalf("Failed to create schema: %v", err)
	}

	// Build model configs for shared registry (chat, fast, vlm)
	modelConfigs := []models.ModelConfig{
		{ModelID: config.ChatOpenAIModel, BaseURL: config.ChatOpenAIBaseURL, APIKey: config.ChatOpenAIAPIKey},
		{ModelID: config.FastOpenAIModel, BaseURL: config.FastOpenAIBaseURL, APIKey: config.FastOpenAIAPIKey},
		{ModelID: config.VLMOpenAIModel, BaseURL: config.VLMOpenAIBaseURL, APIKey: config.VLMOpenAIAPIKey},
	}

	// Create shared model registry (fetches all model info and probes dimensions in parallel)
	modelRegistry := models.NewRegistry(modelConfigs)

	// Initialize chat service
	chatCfg := &chat.Config{
		ChatOpenAIModel:         config.ChatOpenAIModel,
		ChatOpenAIBaseURL:       config.ChatOpenAIBaseURL,
		ChatOpenAIAPIKey:        config.ChatOpenAIAPIKey,
		FastOpenAIModel:         config.FastOpenAIModel,
		FastOpenAIBaseURL:       config.FastOpenAIBaseURL,
		FastOpenAIAPIKey:        config.FastOpenAIAPIKey,
		ContentTrimSafetyMargin: config.ContentTrimSafetyMargin,
	}

	// Default fast model to main model if not configured
	if chatCfg.FastOpenAIModel == "" {
		chatCfg.FastOpenAIModel = chatCfg.ChatOpenAIModel
	}
	if chatCfg.FastOpenAIBaseURL == "" {
		chatCfg.FastOpenAIBaseURL = chatCfg.ChatOpenAIBaseURL
	}
	if chatCfg.FastOpenAIAPIKey == "" {
		chatCfg.FastOpenAIAPIKey = chatCfg.ChatOpenAIAPIKey
	}

	// Default safety margin to 10% if not set
	if chatCfg.ContentTrimSafetyMargin == 0 {
		chatCfg.ContentTrimSafetyMargin = 10
	}
	chatService := chat.NewService(dbClient, chatCfg, modelRegistry)

	// Initialize JWT manager and auth interceptor
	jwtManager := server.NewJWTManager(config.JWTSecret)
	authInterceptor := server.NewAuthInterceptor(jwtManager, dbClient)
	log.Printf("Initialized auth interceptor")

	// Create storage for frames
	storage := webrtc.NewStorage(config.FramesBaseDir())
	log.Printf("[Main] Initialized storage: baseDir=%s", config.FramesBaseDir())

	// Create event service BEFORE batch manager (batch manager needs the broadcaster)
	eventService := service.NewEventService(dbClient)

	// Create VLM frame client and batch manager
	var batchManager *webrtc.BatchManager
	if config.VLMOpenAIBaseURL != "" {
		vlmTimeout := time.Duration(config.VLMTimeoutSec) * time.Second
		frameClient := webrtc.NewFrameClient(config.VLMOpenAIBaseURL, config.VLMOpenAIModel, config.VLMOpenAIAPIKey, vlmTimeout, "Summarize the video", modelRegistry)
		batchManager = webrtc.NewBatchManager(frameClient, config.FrameBatchSize, storage, dbClient, eventService.GetBroadcaster())
		log.Printf("[Main] Initialized VLM frame client: url=%s, model=%s, batchSize=%d, timeout=%vs", config.VLMOpenAIBaseURL, config.VLMOpenAIModel, config.FrameBatchSize, config.VLMTimeoutSec)
	} else {
		log.Printf("[Main] VLM not configured, frame summaries disabled")
	}

	// Create service registry for managing services
	frameInterval := time.Duration(config.FrameIntervalSeconds * float64(time.Second))
	idleTimeout := time.Duration(config.BridgeIdleTimeoutSec) * time.Second
	serviceRegistry := service.NewServiceRegistry(
		dbClient,
		frameInterval,
		storage,
		nil, // will be set after nodeServer is created
		batchManager,
		idleTimeout,
		config.BridgeMaxRetries,
	)

	serviceService := service.NewService(dbClient, serviceRegistry)

	// Create storage service
	storageService := service.NewStorageService(dbClient, &service.StorageConfig{
		StorageBaseDir: config.FramesBaseDir(),
	})

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

	// Initialize node server for WebSocket connections
	nodeServer := server.NewServer(config)

	// Set server reference in service registry
	serviceRegistry.SetServer(nodeServer)

	// Wire up node event callbacks
	nodeServer.OnNodeReady(serviceRegistry.SetNodeOnline)
	nodeServer.OnNodeOffline(serviceRegistry.SetNodeOffline)

	// Load existing services from database
	if err := serviceRegistry.LoadServices(); err != nil {
		log.Printf("Failed to load services: %v", err)
	}

	// Mount WebRTCService with auth interceptor
	webrtcService := webrtc.NewService(nodeServer, dbClient)
	webrtcPath, webrtcHandler := webrtcv1connect.NewWebRTCServiceHandler(
		webrtcService,
		connect.WithInterceptors(authInterceptor),
	)
	mux.Handle(webrtcPath, webrtcHandler)
	log.Printf("Mounted WebRTCService at %s (with auth)", webrtcPath)

	// Add CORS middleware
	handler := withCORS(mux)

	// Get node server handler (includes /node/connect endpoint)
	nodeHandler := nodeServer.GetHTTPHandler()

	// Mount node endpoints on main mux
	mux.Handle("/node/", nodeHandler)
	mux.Handle("/health", nodeHandler)

	// Register HTTP handlers for serving JPEG frames
	storageService.RegisterHTTPHandlers(mux)

	// Start server
	log.Printf("Server starting on %s", config.ListenAddr)
	log.Printf("  - Auth RPC: /auth.v1.AuthService/*")
	log.Printf("  - Chat RPC: /chat.v1.ChatService/*")
	log.Printf("  - Service RPC: /service.v1.ServiceService/*")
	log.Printf("  - Storage RPC: /service.v1.StorageService/*")
	log.Printf("  - Event RPC: /service.v1.EventService/*")
	log.Printf("  - WebRTC RPC: /webrtc.v1.WebRTCService/*")
	log.Printf("  - Node WebSocket: /node/connect")
	log.Printf("  - Storage HTTP: /storage/{itemID}")

	// Use h2c for HTTP/2 without TLS
	h2s := &http2.Server{}
	srv := &http.Server{
		Addr:    config.ListenAddr,
		Handler: h2c.NewHandler(handler, h2s),
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
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, connect-protocol-version, connect-timeout-header, connect-accept-Encoding")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
