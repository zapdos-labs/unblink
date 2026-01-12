package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/unblink/unblink/relay"
)

func main() {
	// Load .env file if it exists (optional, won't error if missing)
	if err := godotenv.Load(); err != nil {
		log.Printf("[Main] No .env file found or error loading it (this is optional): %v", err)
	} else {
		log.Println("[Main] Loaded .env file")
	}

	// Load config to get ports from environment
	config, err := relay.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Parse subcommands
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "database":
			handleDatabaseCommand()
			return
		case "-h", "--help", "help":
			printUsage()
			return
		}
	}

	// Run relay server
	relayAddr := ":" + config.RelayPort // WebSocket server for node connections (port 9020)
	apiAddr := ":" + config.APIPort     // HTTP API for browser clients (port 8020)

	r := relay.NewRelay()

	// Start HTTP API server
	apiServer, err := relay.StartHTTPAPIServer(r, apiAddr, config)
	if err != nil {
		log.Fatalf("Failed to start HTTP API: %v", err)
	}

	// Start WebSocket server
	wsServer, err := relay.StartWebSocketServerAsync(r, relayAddr)
	if err != nil {
		log.Fatalf("Failed to start WebSocket server: %v", err)
	}

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("[Main] Shutting down...")

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Shutdown HTTP API server
	if err := apiServer.Shutdown(ctx); err != nil {
		log.Printf("[Main] HTTP API shutdown error: %v", err)
	}

	// Shutdown WebSocket server
	if err := wsServer.Shutdown(ctx); err != nil {
		log.Printf("[Main] WebSocket server shutdown error: %v", err)
	}

	// Shutdown relay (close node connections)
	r.Shutdown()

	log.Println("[Main] Shutdown complete")
}

func handleDatabaseCommand() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: relay database <command>")
		fmt.Println("Commands:")
		fmt.Println("  delete  Delete the database directory")
		os.Exit(1)
	}

	switch os.Args[2] {
	case "delete":
		deleteDatabase()
	default:
		fmt.Printf("Unknown database command: %s\n", os.Args[2])
		fmt.Println("Available commands: delete")
		os.Exit(1)
	}
}

func deleteDatabase() {
	// Load config to get database path
	config, err := relay.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Delete the entire database directory (containing relay.db and -wal files)
	dbDir := config.AppDir + "/database"

	// Check if directory exists
	if _, err := os.Stat(dbDir); os.IsNotExist(err) {
		log.Printf("Database directory does not exist: %s", dbDir)
		return
	}

	// Confirm deletion
	fmt.Printf("Are you sure you want to delete the database directory at %s? (yes/no): ", dbDir)
	var response string
	fmt.Scanln(&response)

	if response != "yes" {
		fmt.Println("Deletion cancelled")
		return
	}

	// Delete the entire directory
	if err := os.RemoveAll(dbDir); err != nil {
		log.Fatalf("Failed to delete database directory: %v", err)
	}

	log.Printf("Database directory deleted: %s", dbDir)
}

func printUsage() {
	fmt.Println("Usage: relay [command]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  database delete  Delete the database file")
	fmt.Println("  help, -h         Show this help message")
}
