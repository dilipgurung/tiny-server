package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"sync"

	"github.com/skip2/go-qrcode"
)

type SSEHub struct {
	clients map[chan string]struct{}
	mu      sync.Mutex
}

func NewSSEHub() *SSEHub {
	return &SSEHub{
		clients: make(map[chan string]struct{}),
	}
}

func (h *SSEHub) addClient(ch chan string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[ch] = struct{}{}
}

func (h *SSEHub) removeClient(ch chan string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, ch)
}

// Broadcast sends a message to all connected SSE clients
func (h *SSEHub) Broadcast(message string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for ch := range h.clients {
		select {
		case ch <- message:
		default:
			// client too slow; skip rather than block
		}
	}
}

func SSEHandler(hub *SSEHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		// Send an initial comment to establish the connection
		fmt.Fprint(w, ": connected\n\n")
		flusher.Flush()

		ch := make(chan string, 1)
		hub.addClient(ch)
		defer hub.removeClient(ch)

		for {
			select {
			case msg := <-ch:
				fmt.Fprintf(w, "data: %s\n\n", msg)
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	}
}

// formatAddress renders a listen address for an IP, bracketing IPv6 literals.
func formatAddress(ip net.IP, port string) string {
	if v4 := ip.To4(); v4 != nil {
		return fmt.Sprintf("http://%s:%s", v4.String(), port)
	}
	return fmt.Sprintf("http://[%s]:%s", ip.To16().String(), port)
}

func GetNetworkAddresses(port string) (string, []string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", nil, err
	}

	var host string
	var addresses []string

	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsUnspecified() {
				continue
			}

			formatted := formatAddress(ip, port)
			addresses = append(addresses, formatted)
			if !ip.IsLoopback() && host == "" {
				host = formatted
			}
		}
	}

	if host == "" && len(addresses) > 0 {
		host = addresses[0]
	}

	return host, addresses, nil
}

type Server struct {
	httpServer *http.Server
	watcher    *Watcher
}

func NewServer(port, dir string) (*Server, error) {
	absPath, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("error getting absolute path: %w", err)
	}

	hub := NewSSEHub()
	mux := http.NewServeMux()

	fs := http.FileServer(http.Dir(absPath))
	wrappedHandler := logRequest(blockDotfiles(liveReload(fs)))
	mux.Handle("/", wrappedHandler)

	mux.HandleFunc("/.live-reload", SSEHandler(hub))

	watcher, err := NewWatcher(hub)
	if err != nil {
		return nil, fmt.Errorf("error creating watcher: %w", err)
	}

	if err := watcher.WatchDirectory(absPath); err != nil {
		return nil, fmt.Errorf("error watching directory: %w", err)
	}
	watcher.Start()

	return &Server{
		httpServer: &http.Server{
			Addr:    ":" + port,
			Handler: mux,
		},
		watcher: watcher,
	}, nil
}

func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	_ = s.watcher.Close()
	return s.httpServer.Shutdown(ctx)
}

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
