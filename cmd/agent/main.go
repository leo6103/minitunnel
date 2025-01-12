package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"minitunnel/internal/config"
	"minitunnel/internal/protocol"

	"github.com/quic-go/quic-go"
)

type Agent struct {
	config    *config.AgentConfig
	clientID  string
	tunnelURL string
}

func NewAgent(cfg *config.AgentConfig) *Agent {
	return &Agent{
		config: cfg,
	}
}

func (a *Agent) Start() error {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: a.config.Insecure,
		NextProtos:         []string{"minitunnel"},
	}

	log.Printf("Connecting to server at %s...", a.config.ServerAddr)

	// Connect to server
	conn, err := quic.DialAddr(context.Background(), a.config.ServerAddr, tlsConfig, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer conn.CloseWithError(0, "")

	// Open stream
	log.Printf("Opening stream to server...")
	stream, err := conn.OpenStreamSync(context.Background())
	if err != nil {
		return fmt.Errorf("failed to open stream: %w", err)
	}
	defer stream.Close()

	log.Printf("Stream opened successfully")

	// Send hello message to establish the stream
	helloMsg := protocol.Message{
		Type:    protocol.MsgTypeHello,
		Payload: json.RawMessage("{}"),
	}
	if err := protocol.WriteMessage(stream, helloMsg); err != nil {
		return fmt.Errorf("failed to send hello message: %w", err)
	}

	log.Printf("Waiting for welcome message...")

	// Wait for welcome message
	msg, err := protocol.ReadMessage(stream)
	if err != nil {
		return fmt.Errorf("failed to read welcome message: %w", err)
	}

	log.Printf("Received message type: %s", msg.Type)

	if msg.Type != protocol.MsgTypeWelcome {
		return fmt.Errorf("expected welcome message, got %s", msg.Type)
	}

	// Parse welcome payload
	var welcome protocol.WelcomePayload
	if err := json.Unmarshal(msg.Payload, &welcome); err != nil {
		return fmt.Errorf("failed to parse welcome message: %w", err)
	}

	a.clientID = welcome.ClientID
	a.tunnelURL = welcome.TunnelURL

	log.Printf("✓ Tunnel established!")
	log.Printf("Client ID: %s", a.clientID)
	log.Printf("Tunnel URL: %s", a.tunnelURL)
	log.Printf("Forwarding to: %s", a.config.LocalAddr)
	log.Printf("\nPress Ctrl+C to stop...")

	// Start heartbeat
	go a.sendHeartbeats(stream)

	// Handle incoming requests
	return a.handleRequests(stream)
}

func (a *Agent) sendHeartbeats(stream quic.Stream) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		msg := protocol.Message{
			Type:    protocol.MsgTypeHeartbeat,
			Payload: json.RawMessage("{}"),
		}
		if err := protocol.WriteMessage(stream, msg); err != nil {
			log.Printf("Error sending heartbeat: %v", err)
			return
		}
	}
}

func (a *Agent) handleRequests(stream quic.Stream) error {
	for {
		// Read request from server
		msg, err := protocol.ReadMessage(stream)
		if err != nil {
			if err == io.EOF {
				log.Printf("Server disconnected")
				return nil
			}
			return fmt.Errorf("error reading request: %w", err)
		}

		if msg.Type != protocol.MsgTypeRequest {
			log.Printf("Unexpected message type: %s", msg.Type)
			continue
		}

		// Parse HTTP request
		var httpReq protocol.HTTPRequest
		if err := json.Unmarshal(msg.Payload, &httpReq); err != nil {
			log.Printf("Error parsing request: %v", err)
			continue
		}

		log.Printf("→ %s %s", httpReq.Method, httpReq.Path)

		// Forward to local service
		resp, err := a.forwardToLocal(httpReq)
		if err != nil {
			log.Printf("Error forwarding request: %v", err)
			// Send error response
			resp = protocol.HTTPResponse{
				StatusCode: http.StatusBadGateway,
				Headers:    make(map[string][]string),
				Body:       []byte(fmt.Sprintf("Error: %v", err)),
			}
		}

		log.Printf("← %d", resp.StatusCode)

		// Send response back to server
		respMsg, err := protocol.NewResponseMessage(resp)
		if err != nil {
			log.Printf("Error creating response message: %v", err)
			continue
		}

		if err := protocol.WriteMessage(stream, respMsg); err != nil {
			log.Printf("Error sending response: %v", err)
			return err
		}
	}
}

func (a *Agent) forwardToLocal(httpReq protocol.HTTPRequest) (protocol.HTTPResponse, error) {
	// Create HTTP request to local service
	url := fmt.Sprintf("http://%s%s", a.config.LocalAddr, httpReq.Path)

	// Create request with body if present
	var bodyReader io.Reader
	if len(httpReq.Body) > 0 {
		bodyReader = bytes.NewReader(httpReq.Body)
	}

	req, err := http.NewRequest(httpReq.Method, url, bodyReader)
	if err != nil {
		return protocol.HTTPResponse{}, err
	}

	// Copy headers, but rewrite Host header to local address
	// This prevents the local service from generating absolute URLs with the tunnel domain
	for key, values := range httpReq.Headers {
		// Skip Host header - we'll set it to the local address
		if key == "Host" {
			continue
		}
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// Set Host header to local address so the app thinks it's being accessed directly
	req.Host = a.config.LocalAddr
	req.Header.Set("Host", a.config.LocalAddr)

	// Send request
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return protocol.HTTPResponse{}, err
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return protocol.HTTPResponse{}, err
	}

	// Create response
	return protocol.HTTPResponse{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Body:       body,
	}, nil
}

func main() {
	// Check for simple syntax: mt_agent http <port>
	if len(os.Args) == 3 && os.Args[1] == "http" {
		port := os.Args[2]
		cfg := &config.AgentConfig{
			ServerAddr: "localhost:8080",
			LocalAddr:  fmt.Sprintf("localhost:%s", port),
			Insecure:   true,
		}

		agent := NewAgent(cfg)
		if err := agent.Start(); err != nil {
			log.Fatalf("Agent error: %v", err)
		}
		return
	}

	// Otherwise use flag-based configuration
	cfg := config.ParseAgentConfig()

	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	agent := NewAgent(cfg)
	if err := agent.Start(); err != nil {
		log.Fatalf("Agent error: %v", err)
	}
}
