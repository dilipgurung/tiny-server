package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/skip2/go-qrcode"
)

type WebSocketHub struct {
	connections map[*websocket.Conn]bool
	mu          sync.Mutex
}

func NewWebSocketHub() *WebSocketHub {
	return &WebSocketHub{
		connections: make(map[*websocket.Conn]bool),
	}
}

func (h *WebSocketHub) AddConnection(conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.connections[conn] = true
}

func (h *WebSocketHub) RemoveConnection(conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.connections, conn)
}

// Broadcast sends a message to all connected WebSocket clients
func (h *WebSocketHub) Broadcast(message string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for conn := range h.connections {
		if err := conn.WriteMessage(websocket.TextMessage, []byte(message)); err != nil {
			log.Printf("WebSocket write error: %v", err)
			conn.Close()
			delete(h.connections, conn)
		}
	}
}

func WebSocketHandler(hub *WebSocketHub, upgrader websocket.Upgrader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("WebSocket upgrade error: %v", err)
			return
		}
		defer func() {
			hub.RemoveConnection(conn)
			if err := conn.Close(); err != nil {
				log.Printf("Error closing WebSocket connection: %v", err)
			}
		}()

		hub.AddConnection(conn)

		// Keep the connection alive and handle errors
		for {
			_, _, err := conn.NextReader()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("WebSocket error: %v", err)
				}
				return
			}
		}
	}
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
			if ip != nil && ip.To4() != nil {
				addr := fmt.Sprintf("http://%s:%s", ip.String(), port)
				addresses = append(addresses, addr)
				if !ip.IsLoopback() && host == "" {
					host = addr
				}
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
	hub        *WebSocketHub
	watcher    *Watcher
}

func NewServer(port, dir string) (*Server, error) {
	absPath, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("error getting absolute path: %w", err)
	}

	hub := NewWebSocketHub()
	mux := http.NewServeMux()

	fs := http.FileServer(http.Dir(absPath))
	wrappedHandler := logRequest(liveReload(fs, port))
	mux.Handle("/", wrappedHandler)

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	mux.HandleFunc("/.live-reload", WebSocketHandler(hub, upgrader))

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
		hub:     hub,
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
