package server

import (
	"fmt"
	"log"

	"github.com/skip2/go-qrcode"
)

func (s *Server) PrintInfo(port, dir string) {
	host, addresses, err := GetNetworkAddresses(port)
	if err != nil {
		log.Printf("Error getting network addresses: %v", err)
		return
	}

	qr, err := qrcode.New(host, qrcode.Medium)
	if err != nil {
		log.Printf("Error generating QR code: %v", err)
	} else {
		for _, row := range qr.Bitmap() {
			for _, cell := range row {
				if cell {
					fmt.Print("██")
				} else {
					fmt.Print("  ")
				}
			}
			fmt.Println()
		}
	}

	fmt.Printf("Starting tiny-server ...\n")
	fmt.Printf("Serving %s through http\n", dir)
	fmt.Printf("Available on:\n")
	for _, addr := range addresses {
		fmt.Printf("  %s\n", addr)
	}
	fmt.Println("Press CTRL+C to stop the server.")
}
