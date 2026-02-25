package node

import (
	"log"
	"os"

	qrterminal "github.com/mdp/qrterminal/v3"
)

func printDashboardOutput(nodeID, url string) {
	log.Printf("[Node] Node ID: %s", nodeID)
	log.Printf("[Node] Dashboard URL: %s", url)
	log.Printf("[Node] Dashboard QR:")
	qrterminal.GenerateHalfBlock(url, qrterminal.L, os.Stdout)
}
