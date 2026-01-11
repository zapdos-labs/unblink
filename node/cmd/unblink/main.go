package main

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/unblink/unblink/node"
)

//go:embed default_config.jsonc
var defaultConfig []byte

func main() {
	// Check for help
	if len(os.Args) > 1 && (os.Args[1] == "-h" || os.Args[1] == "--help" || os.Args[1] == "help") {
		printUsage()
		return
	}

	// Handle subcommands
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "show-config":
			showConfig()
			return
		case "login":
			doLogin()
			return
		case "logout":
			logout()
			return
		}
	}

	// Load config (LoadConfig creates it with generated node ID if missing)
	config, err := node.LoadConfigWithDefault(defaultConfig)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Validate relay_addr is set
	if config.RelayAddr == "" {
		log.Fatalf("[Node] relay_addr is not set in config. Please edit %s and add \"relay_addr\": \"<your-relay-address>\"", mustConfigPath())
	}

	// Log node ID
	log.Printf("[Node] Node ID: %s", config.NodeID)

	if len(config.Services) == 0 {
		log.Fatalf("[Node] No services configured. Edit %s to add services.", mustConfigPath())
	}

	// Run node (will authorize if needed, then continue running)
	runNode(config)
}

func doLogin() {
	config, err := node.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Validate relay_addr is set
	if config.RelayAddr == "" {
		log.Fatalf("[Node] relay_addr is not set in config. Please edit %s and add \"relay_addr\": \"<your-relay-address>\"", mustConfigPath())
	}

	if len(config.Services) == 0 {
		log.Fatalf("[Node] No services configured. Edit %s to add services.", mustConfigPath())
	}

	if config.Token != "" {
		log.Println("[Node] Already logged in. Use 'unblink logout' first if you want to re-authorize.")
		return
	}

	log.Printf("[Node] Starting authorization with relay=%s, services=%d", config.RelayAddr, len(config.Services))

	runNode(config)
}

func runNode(config *node.Config) {
	if config.Token == "" {
		log.Printf("[Node] Not authorized. Starting authorization flow...")
	} else {
		log.Printf("[Node] Starting with relay=%s, services=%d", config.RelayAddr, len(config.Services))
		log.Printf("[Node] Using saved credentials for node: %s", config.NodeID)
	}

	// Create client
	client := node.NewNodeClient(config.RelayAddr, config.Services, config.NodeID, config.Token)

	// Handle connection ready
	client.OnConnectionReady = func(nodeID, dashboardURL string) {
		if dashboardURL != "" {
			log.Println("========================================")
			log.Printf("AUTHORIZATION REQUIRED")
			log.Printf("Open this URL in your browser:")
			log.Printf("%s", dashboardURL)
			log.Println("========================================")
		} else {
			log.Printf("[Node] Connected and authorized: %s", nodeID)
		}
	}

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("[Node] Shutting down...")
		client.Close()
	}()

	if err := client.Run(config); err != nil {
		log.Fatalf("%v", err)
	}
}

func showConfig() {
	path, err := node.ConfigPath()
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	// LoadConfig creates the file if it doesn't exist
	config, err := node.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if len(config.Services) == 0 {
		fmt.Println("Config file created with default services template.")
		fmt.Println("Edit the file to add your services:")
		fmt.Println(path)
	} else {
		fmt.Println(path)
	}
}

func logout() {
	config, err := node.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if config.Token == "" {
		log.Println("[Node] Not logged in.")
		return
	}

	config.Token = ""
	if err := node.SaveConfig(config); err != nil {
		log.Fatalf("Failed to save config: %v", err)
	}

	log.Println("[Node] Logged out successfully. Run 'unblink' to re-authorize.")
}

func mustConfigPath() string {
	path, err := node.ConfigPath()
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	return path
}

func printUsage() {
	fmt.Println("Usage: unblink [command]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  show-config  Show the config file path")
	fmt.Println("  login        Authorize with the relay server")
	fmt.Println("  logout       Remove saved credentials")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -h      Show this help message")
	fmt.Println()
	fmt.Println("Configuration:")
	fmt.Println("  Relay address is configured in the config file (relay_addr)")
}
