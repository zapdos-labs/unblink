package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/zapdos-labs/unblink/relay"
)

func main() {
	// Load .env file if it exists (optional, won't error if missing)
	if err := godotenv.Load(); err != nil {
		log.Printf("[Main] No .env file found or error loading it (this is optional): %v", err)
	} else {
		log.Println("[Main] Loaded .env file")
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

	// Default: run relay server
	addr := flag.String("addr", ":9000", "Listen address for node connections")
	httpAddr := flag.String("http", ":8080", "Listen address for HTTP API")
	flag.Parse()

	r := relay.NewRelay()

	if err := r.Listen(*addr); err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	// Start HTTP API
	go relay.StartHTTPAPI(r, *httpAddr)

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("[Main] Shutting down...")
		r.Shutdown()
	}()

	if err := r.Serve(); err != nil {
		log.Fatalf("Relay error: %v", err)
	}
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
	fmt.Println("Usage: relay [command] [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  database delete  Delete the database file")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -addr  Listen address for node connections (default: :9000)")
	fmt.Println("  -http  Listen address for HTTP API (default: :8080)")
	fmt.Println("  -h     Show this help message")
}
