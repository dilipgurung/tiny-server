package server

import (
	"bufio"
	"fmt"
	"log"
	"os"

	"github.com/skip2/go-qrcode"
)

func (s *Server) PrintInfo(port, dir string) {
	host, addresses, err := GetNetworkAddresses(port)
	if err != nil {
		log.Printf("Error getting network addresses: %v", err)
		return
	}

	// os.Stdout is unbuffered; writing the QR code a cell at a time cost
	// one syscall per module (over a thousand for a typical code).
	out := bufio.NewWriter(os.Stdout)
	defer func() { _ = out.Flush() }()

	if host != "" {
		qr, err := qrcode.New(host, qrcode.Medium)
		if err != nil {
			log.Printf("Error generating QR code: %v", err)
		} else {
			for _, row := range qr.Bitmap() {
				for _, cell := range row {
					if cell {
						_, _ = out.WriteString("██")
					} else {
						_, _ = out.WriteString("  ")
					}
				}
				_ = out.WriteByte('\n')
			}
		}
	}

	_, _ = fmt.Fprintf(out, "Starting tiny-server ...\n")
	_, _ = fmt.Fprintf(out, "Serving %s through http\n", dir)
	_, _ = fmt.Fprintf(out, "Available on:\n")
	for _, addr := range addresses {
		_, _ = fmt.Fprintf(out, "  %s\n", addr)
	}
	_, _ = fmt.Fprintln(out, "Press CTRL+C to stop the server.")
}
