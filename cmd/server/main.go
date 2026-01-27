package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"unb/database"
	"unb/server"
	"unb/server/auth"
	"unb/server/chat"
	"unb/server/gen/chat/v1/auth/authv1connect"
	"unb/server/models"
	"unb/server/gen/chat/v1/chatv1connect"
	"unb/server/gen/service/v1/servicev1connect"
	"unb/server/gen/webrtc/v1/webrtcv1connect"
	"unb/server/service"
	"unb/server/webrtc"

	"connectrpc.com/connect"
	"github.com/go-gst/go-gst/gst"
)

func main() {
	// Initialize GStreamer
	gst.Init(nil)

	// Load configuration from file (all fields required)
	config, err := server.LoadConfig("")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database
	dbClient, err := database.NewClient(database.Config{DatabaseURL: config.DatabaseURL})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer dbClient.Close()

	// Drop schema if requested
	if len(os.Args) > 1 && os.Args[1] == "-drop" {
		log.Println("Dropping schema...")
		if err := dbClient.DropSchema(); err != nil {
			log.Fatalf("Failed to drop schema: %v", err)
		}
		log.Println("Schema dropped successfully")
		os.Exit(0)
	}

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
	authInterceptor := server.NewAuthInterceptor(jwtManager)
	log.Printf("Initialized auth interceptor")

	// Create VLM frame client and batch manager
	var batchManager *webrtc.BatchManager
	if config.VLMOpenAIBaseURL != "" {
		vlmTimeout := 30 * time.Second
		if config.VLMTimeoutSec > 0 {
			vlmTimeout = time.Duration(config.VLMTimeoutSec) * time.Second
		}

		frameClient := webrtc.NewFrameClient(config.VLMOpenAIBaseURL, config.VLMOpenAIModel, config.VLMOpenAIAPIKey, vlmTimeout, "Summarize the video", modelRegistry)

		frameBatchSize := 2
		if config.FrameBatchSize > 0 {
			frameBatchSize = config.FrameBatchSize
		}

		batchManager = webrtc.NewBatchManager(frameClient, frameBatchSize, config.FramesBaseDir())
		log.Printf("[Main] Initialized VLM frame client: url=%s, model=%s, batchSize=%d", config.VLMOpenAIBaseURL, config.VLMOpenAIModel, frameBatchSize)
	} else {
		log.Printf("[Main] VLM not configured, frame summaries disabled")
	}

	// Create service registry for frame extraction
	frameInterval := 5.0 * time.Second
	if config.FrameIntervalSeconds > 0 {
		frameInterval = time.Duration(config.FrameIntervalSeconds * float64(time.Second))
	}
	serviceRegistry := service.NewServiceRegistry(
		dbClient,
		frameInterval,
		config.FramesBaseDir(),
		nil, // will be set after nodeServer is created
		batchManager,
	)

	serviceService := service.NewService(dbClient, serviceRegistry)

	// Create storage service
	storageService := service.NewStorageService(dbClient, &service.StorageConfig{
		FramesBaseDir: config.FramesBaseDir(),
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
	webrtcService := webrtc.NewService(nodeServer)
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
	log.Printf("  - WebRTC RPC: /webrtc.v1.WebRTCService/*")
	log.Printf("  - Node WebSocket: /node/connect")
	log.Printf("  - Frames HTTP: /frames/{serviceID}/{frameID}.jpg")

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
