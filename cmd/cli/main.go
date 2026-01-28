package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"unblink/database"
	"unblink/server"
)

func main() {
	// Define flags
	configPath := flag.String("config", "", "Path to config file (default: ~/.unblink/server.config.json)")

	// Parse flags
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Println("Usage: cli [flags] [command]")
		fmt.Println("Flags:")
		fmt.Println("  -config string")
		fmt.Println("        Path to config file (default: ~/.unblink/server.config.json)")
		fmt.Println("Commands:")
		fmt.Println("  -delete-app-dir  Delete the application directory")
		fmt.Println("  -drop            Drop the database schema")
		os.Exit(1)
	}

	// Load configuration
	config, err := server.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database client
	dbClient, err := database.NewClient(database.Config{DatabaseURL: config.DatabaseURL})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer dbClient.Close()

	// Handle the command
	command := flag.Arg(0)
	switch command {
	case "-delete-app-dir":
		handleDeleteAppDir(config)
	case "-drop":
		handleDropSchema(dbClient)
	default:
		log.Fatalf("Unknown command: %s", command)
	}
}

// confirm prompts the user for y/n confirmation
func confirm() bool {
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}

// handleDeleteAppDir deletes the application directory after confirmation
func handleDeleteAppDir(config *server.Config) {
	if config.AppDir == "" {
		log.Fatalf("AppDir is not configured")
	}

	fmt.Printf("WARNING: This will delete the app directory: %s\n", config.AppDir)
	fmt.Print("Are you sure you want to continue? (y/n): ")

	if !confirm() {
		log.Println("Operation cancelled")
		os.Exit(0)
	}

	log.Printf("Deleting app directory: %s", config.AppDir)
	if err := os.RemoveAll(config.AppDir); err != nil {
		log.Fatalf("Failed to delete app directory: %v", err)
	}
	log.Println("App directory deleted successfully")
}

// handleDropSchema drops the database schema after confirmation
func handleDropSchema(dbClient *database.Client) {
	fmt.Println("WARNING: This will drop the database schema and delete all data")
	fmt.Print("Are you sure you want to continue? (y/n): ")

	if !confirm() {
		log.Println("Operation cancelled")
		os.Exit(0)
	}

	log.Println("Dropping schema...")
	if err := dbClient.DropSchema(); err != nil {
		log.Fatalf("Failed to drop schema: %v", err)
	}
	log.Println("Schema dropped successfully")
}
