package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"minitunnel/internal/config"
	"minitunnel/internal/protocol"

	"github.com/google/uuid"
	"github.com/quic-go/quic-go"
)

type Server struct {
	config  *config.ServerConfig
	clients sync.Map // map[clientID]*ClientInfo
	mu      sync.RWMutex
}

type ClientInfo struct {
	stream quic.Stream
	mu     sync.Mutex // Protects stream read/write operations
}

func NewServer(cfg *config.ServerConfig) *Server {
	return &Server{
		config: cfg,
	}
}

func (s *Server) Start() error {
	// Load TLS certificates
	cert, err := tls.LoadX509KeyPair(s.config.CertFile, s.config.KeyFile)
	if err != nil {
		return fmt.Errorf("failed to load TLS certificates: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"minitunnel"},
	}

	// Start QUIC listener for agent connections
	addr := fmt.Sprintf(":%d", s.config.Port)
	listener, err := quic.ListenAddr(addr, tlsConfig, nil)
	if err != nil {
		return fmt.Errorf("failed to start QUIC listener: %w", err)
	}

	log.Printf("Server listening on %s", addr)
	log.Printf("Waiting for agent connections...")

	// Start HTTP server for incoming requests
	go s.startHTTPServer()

	// Accept agent connections
	for {
		conn, err := listener.Accept(context.Background())
		if err != nil {
			log.Printf("Error accepting connection: %v", err)
			continue
		}
		go s.handleAgentConnection(conn)
	}
}

func (s *Server) handleAgentConnection(conn quic.Connection) {
	log.Printf("New connection from %s, waiting for stream...", conn.RemoteAddr())

	// Accept stream opened by the agent with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := conn.AcceptStream(ctx)
	if err != nil {
		log.Printf("Error accepting stream: %v", err)
		log.Printf("This might be a QUIC handshake issue. Connection state: %v", conn.Context().Err())
		return
	}
	defer stream.Close()

	log.Printf("Stream accepted from %s", conn.RemoteAddr())

	// Read hello message from agent
	helloMsg, err := protocol.ReadMessage(stream)
	if err != nil {
		log.Printf("Error reading hello message: %v", err)
		return
	}

	if helloMsg.Type != protocol.MsgTypeHello {
		log.Printf("Expected hello message, got %s", helloMsg.Type)
		return
	}

	log.Printf("Received hello from agent")

	// Generate client ID
	clientID := uuid.New().String()
	tunnelURL := fmt.Sprintf("http://localhost:%d/%s", s.config.Port+1, clientID)

	// Store client connection
	clientInfo := &ClientInfo{
		stream: stream,
	}
	s.clients.Store(clientID, clientInfo)
	defer s.clients.Delete(clientID)

	log.Printf("New agent connected: %s", clientID)
	log.Printf("Tunnel URL: %s", tunnelURL)

	// Send welcome message
	welcomeMsg, err := protocol.NewWelcomeMessage(clientID, tunnelURL)
	if err != nil {
		log.Printf("Error creating welcome message: %v", err)
		return
	}

	if err := protocol.WriteMessage(stream, welcomeMsg); err != nil {
		log.Printf("Error sending welcome message: %v", err)
		return
	}

	log.Printf("Welcome message sent to %s", clientID)

	// Keep connection alive - just wait for disconnection
	// Note: We don't read messages here to avoid conflicts with HTTP handler
	// The HTTP handler will read responses, and heartbeats are fire-and-forget from agent
	<-conn.Context().Done()
	log.Printf("Agent disconnected: %s", clientID)
}

func (s *Server) startHTTPServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleHTTPRequest)

	addr := fmt.Sprintf(":%d", s.config.Port+1) // Use port+1 for HTTP to avoid conflict
	log.Printf("HTTP server listening on %s", addr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("HTTP server error: %v", err)
	}
}

func (s *Server) handleHTTPRequest(w http.ResponseWriter, r *http.Request) {
	// Extract client ID from path
	path := strings.TrimPrefix(r.URL.Path, "/")
	parts := strings.SplitN(path, "/", 2)

	var clientID string
	var requestPath string

	// Check if first part looks like a UUID (contains hyphens and is ~36 chars)
	if len(parts) > 0 && len(parts[0]) > 30 && strings.Contains(parts[0], "-") {
		// Path has UUID prefix: /uuid/path
		clientID = parts[0]
		requestPath = "/"
		if len(parts) > 1 && parts[1] != "" {
			requestPath = "/" + parts[1]
		}
	} else {
		// No UUID prefix - try to route to the only connected agent
		// This handles Next.js assets like /_next/static/...
		var foundClientID string
		count := 0
		s.clients.Range(func(key, value interface{}) bool {
			foundClientID = key.(string)
			count++
			return true
		})

		if count == 0 {
			http.Error(w, "No agents connected", http.StatusServiceUnavailable)
			return
		} else if count > 1 {
			http.Error(w, "Multiple agents connected - please use full tunnel URL: http://server:port/<client-id>/path", http.StatusBadRequest)
			return
		}

		clientID = foundClientID
		requestPath = r.URL.Path
	}

	// Preserve query string
	if r.URL.RawQuery != "" {
		requestPath += "?" + r.URL.RawQuery
	}

	// Find the agent connection
	val, ok := s.clients.Load(clientID)
	if !ok {
		http.Error(w, "Tunnel not found", http.StatusNotFound)
		return
	}

	clientInfo := val.(*ClientInfo)
	stream := clientInfo.stream

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}

	// Create HTTP request message
	httpReq := protocol.HTTPRequest{
		Method:  r.Method,
		Path:    requestPath,
		Headers: r.Header,
		Body:    body,
	}

	reqMsg, err := protocol.NewRequestMessage(httpReq)
	if err != nil {
		http.Error(w, "Error creating request message", http.StatusInternalServerError)
		return
	}

	// Send request to agent (with mutex protection)
	clientInfo.mu.Lock()
	if err := protocol.WriteMessage(stream, reqMsg); err != nil {
		clientInfo.mu.Unlock()
		http.Error(w, "Error forwarding request to agent", http.StatusBadGateway)
		return
	}

	// Wait for response from agent
	respMsg, err := protocol.ReadMessage(stream)
	clientInfo.mu.Unlock()

	if err != nil {
		http.Error(w, "Error reading response from agent", http.StatusBadGateway)
		return
	}

	if respMsg.Type != protocol.MsgTypeResponse {
		http.Error(w, "Invalid response from agent", http.StatusBadGateway)
		return
	}

	// Parse response
	var httpResp protocol.HTTPResponse
	if err := json.Unmarshal(respMsg.Payload, &httpResp); err != nil {
		http.Error(w, "Error parsing response from agent", http.StatusBadGateway)
		return
	}

	// If this is an HTML response, inject a <base> tag to fix relative URLs
	contentType := ""
	if headers, ok := httpResp.Headers["Content-Type"]; ok && len(headers) > 0 {
		contentType = headers[0]
	}

	if strings.Contains(contentType, "text/html") {
		// Inject <base href="/clientID/"> into the HTML
		baseTag := fmt.Sprintf(`<base href="/%s/">`, clientID)
		bodyStr := string(httpResp.Body)

		// Try to inject after <head> tag
		if strings.Contains(bodyStr, "<head>") {
			bodyStr = strings.Replace(bodyStr, "<head>", "<head>"+baseTag, 1)
			httpResp.Body = []byte(bodyStr)
		} else if strings.Contains(bodyStr, "<HEAD>") {
			bodyStr = strings.Replace(bodyStr, "<HEAD>", "<HEAD>"+baseTag, 1)
			httpResp.Body = []byte(bodyStr)
		}
	}

	// Remove Content-Length header as we may have modified the body
	// Go will set it automatically
	delete(httpResp.Headers, "Content-Length")

	// Write response headers
	for key, values := range httpResp.Headers {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Write response
	w.WriteHeader(httpResp.StatusCode)
	w.Write(httpResp.Body)
}

func main() {
	cfg := config.ParseServerConfig()

	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	server := NewServer(cfg)
	if err := server.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
