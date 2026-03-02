package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/zapdos-labs/unblink/node"
)

var configPath string
var discover bool
var discoverHTTP bool
var discoverDebug bool
var discoverTimeout time.Duration
var discoverIfaces csvFlag

type csvFlag []string

func (c *csvFlag) String() string {
	return fmt.Sprintf("%v", []string(*c))
}

func (c *csvFlag) Set(value string) error {
	if value == "" {
		return nil
	}
	*c = append(*c, value)
	return nil
}

func init() {
	flag.StringVar(&configPath, "config", "", "Path to config file")
	flag.BoolVar(&discover, "discover", false, "Discover cameras on local network and exit")
	flag.BoolVar(&discoverHTTP, "discover-http", true, "Include HTTP and MJPEG probing during discovery")
	flag.BoolVar(&discoverDebug, "discover-debug", false, "Enable verbose stage-level discovery metadata")
	flag.DurationVar(&discoverTimeout, "discover-timeout", 0, "Optional overall discovery timeout (0 disables)")
	flag.Var(&discoverIfaces, "discover-iface", "Limit discovery to interface name (repeatable)")
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

	if discover {
		runDiscovery()
		return
	}

	// Default: run node
	configFile, err := node.Load(configPath)
	if err != nil {
		log.Fatalf("[Node] Failed to load config: %v", err)
	}

	// Log startup info
	configAbsPath, err := filepath.Abs(configFile.Path)
	if err != nil {
		configAbsPath = configFile.Path
	}
	log.Printf("[Node] Starting node...")
	log.Printf("[Node] Config: %s", configAbsPath)
	log.Printf("[Node] Server: %s", configFile.Config.RelayAddress)

	runNode(configFile)
}

func runDiscovery() {
	log.Printf("[Node] Starting discovery...")
	report, err := node.DiscoverCameras(context.Background(), node.DiscoveryOptions{
		Timeout:            discoverTimeout,
		IncludeHTTP:        discoverHTTP,
		Debug:              discoverDebug,
		InterfaceWhitelist: []string(discoverIfaces),
	})
	if err != nil {
		log.Fatalf("[Node] Discovery failed: %v", err)
	}
	fmt.Println(node.DiscoveryReportJSON(report))
}

func runNode(configFile *node.ConfigFile) {
	reconnectCfg := configFile.Config.Reconnect
	if reconnectCfg.Enabled {
		log.Printf("[Node] Auto-reconnect enabled (infinite retries)")
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
		fmt.Println("Usage: unblink-node config <command>")
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
	fmt.Println("Usage: unblink-node [command] [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  config show    Show the config file path and contents")
	fmt.Println("  config delete  Delete the config file")
	fmt.Println("  logout         Remove saved token")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -config <path>            Use custom config file (default: ~/.unblink/config.json)")
	fmt.Println("  -discover                 Discover cameras on local network and exit")
	fmt.Println("  -discover-http            Include HTTP and MJPEG probing during discovery")
	fmt.Println("  -discover-timeout <dur>   Optional overall discovery timeout (default: disabled)")
	fmt.Println("  -discover-iface <name>    Restrict to interface (repeatable)")
	fmt.Println("  -discover-debug           Enable verbose discovery metadata")
	fmt.Println("  -h, --help                Show this help message")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  unblink-node                    # Start node (default config)")
	fmt.Println("  unblink-node -discover          # Scan local network for camera endpoints")
	fmt.Println("  unblink-node -discover -discover-iface eth0")
	fmt.Println("  unblink-node config show         # Show config")
	fmt.Println("  unblink-node config delete       # Delete config (will regenerate with UUID)")
	fmt.Println("  unblink-node -config ./my-config # Use custom config file")
	fmt.Println()
}
