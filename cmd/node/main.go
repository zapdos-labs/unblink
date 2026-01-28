package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"unblink/node"
)

var configPath string

func init() {
	flag.StringVar(&configPath, "config", "", "Path to config file")
}

func main() {
	// Parse flags first (so -h works)
	flag.Parse()

	// Check for help
	if len(os.Args) > 1 && (os.Args[1] == "-h" || os.Args[1] == "--help" || os.Args[1] == "help") {
		printUsage()
		return
	}

	// Handle subcommands
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "config":
			handleConfigCommand()
			return
		case "logout":
			configFile, err := node.Load(configPath)
			if err != nil {
				log.Fatalf("Failed to load config: %v", err)
			}
			if err := configFile.Logout(); err != nil {
				log.Fatalf("Failed to logout: %v", err)
			}
			return
		}
	}

	// Default: run node
	configFile, err := node.Load(configPath)
	if err != nil {
		log.Fatalf("[Node] Failed to load config: %v", err)
	}

	// Log startup info
	log.Printf("[Node] Starting node...")
	log.Printf("[Node] Server: %s", configFile.Config.RelayAddress)
	log.Printf("[Node] Node ID: %s", configFile.Config.NodeID)

	runNode(configFile)
}

func runNode(configFile *node.ConfigFile) {
	reconnectCfg := configFile.Config.Reconnect
	if reconnectCfg.Enabled {
		log.Printf("[Node] Auto-reconnect enabled (max_num_attempts=%d)", reconnectCfg.MaxNumAttempts)
	} else {
		log.Printf("[Node] Auto-reconnect is DISABLED")
	}

	// Connection factory function for reconnector
	var currentConn *node.Conn
	createConn := func() *node.Conn {
		conn := node.NewConn(configFile)
		currentConn = conn
		return conn
	}

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	if reconnectCfg.Enabled {
		reconnector := node.NewReconnector(configFile)
		// Signal handler for cleanup
		go func() {
			<-sigChan
			log.Println("[Node] Shutting down...")
			if currentConn != nil {
				currentConn.Close()
			}
			reconnector.Close()
		}()
		reconnector.Run(createConn)
	} else {
		// Single connection mode
		go func() {
			<-sigChan
			log.Println("[Node] Shutting down...")
			if currentConn != nil {
				currentConn.Close()
			}
		}()
		conn := createConn()
		if err := conn.Run(); err != nil {
			log.Fatalf("[Node] Failed to run: %v", err)
		}
	}
}

func handleConfigCommand() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: node config <command>")
		fmt.Println("Commands:")
		fmt.Println("  show    Show the config file path")
		fmt.Println("  delete  Delete the config file")
		os.Exit(1)
	}

	configFile, err := node.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	switch os.Args[2] {
	case "show":
		err = configFile.Show()
	case "delete":
		err = configFile.Delete()
	default:
		fmt.Printf("Unknown config command: %s\n", os.Args[2])
		fmt.Println("Available commands: show, delete")
		os.Exit(1)
	}

	if err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func printUsage() {
	fmt.Println("Usage: node [command] [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  config show    Show the config file path and contents")
	fmt.Println("  config delete  Delete the config file")
	fmt.Println("  logout         Remove saved token")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -config <path>  Use custom config file (default: ~/.unblink/config.json)")
	fmt.Println("  -h, --help     Show this help message")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  node                    # Start node (default config)")
	fmt.Println("  node config show         # Show config")
	fmt.Println("  node config delete       # Delete config (will regenerate with UUID)")
	fmt.Println("  node -config ./my-config # Use custom config file")
	fmt.Println()
}
