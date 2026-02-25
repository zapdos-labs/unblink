package node

import (
	"log"
	"os"

	qrterminal "github.com/mdp/qrterminal/v3"
)

func printDashboardOutput(url string) {
	log.Printf("[Node] Dashboard URL: %s", url)
	qrterminal.GenerateHalfBlock(url, qrterminal.L, os.Stdout)
}
