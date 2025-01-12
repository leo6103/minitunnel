package protocol

import (
	"encoding/json"
	"io"
)

// MessageType defines the type of message being sent
type MessageType string

const (
	// Agent -> Server messages (connection init)
	MsgTypeHello MessageType = "hello" // Agent initiates connection

	// Server -> Agent messages
	MsgTypeWelcome MessageType = "welcome" // Initial connection, sends tunnel URL
	MsgTypeRequest MessageType = "request" // HTTP request to forward

	// Agent -> Server messages
	MsgTypeResponse  MessageType = "response"  // HTTP response from local service
	MsgTypeHeartbeat MessageType = "heartbeat" // Keep-alive ping
)

// Message is the base structure for all protocol messages
type Message struct {
	Type    MessageType     `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// WelcomePayload is sent by server to agent upon connection
type WelcomePayload struct {
	ClientID  string `json:"client_id"`
	TunnelURL string `json:"tunnel_url"`
}

// HTTPRequest represents an HTTP request to be forwarded
type HTTPRequest struct {
	Method  string              `json:"method"`
	Path    string              `json:"path"`
	Headers map[string][]string `json:"headers"`
	Body    []byte              `json:"body"`
}

// HTTPResponse represents an HTTP response from the local service
type HTTPResponse struct {
	StatusCode int                 `json:"status_code"`
	Headers    map[string][]string `json:"headers"`
	Body       []byte              `json:"body"`
}

// WriteMessage writes a message to the writer
func WriteMessage(w io.Writer, msg Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = w.Write(data)
	return err
}

// ReadMessage reads a message from the reader
func ReadMessage(r io.Reader) (*Message, error) {
	decoder := json.NewDecoder(r)
	var msg Message
	if err := decoder.Decode(&msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// NewWelcomeMessage creates a welcome message
func NewWelcomeMessage(clientID, tunnelURL string) (Message, error) {
	payload := WelcomePayload{
		ClientID:  clientID,
		TunnelURL: tunnelURL,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return Message{}, err
	}
	return Message{
		Type:    MsgTypeWelcome,
		Payload: data,
	}, nil
}

// NewRequestMessage creates an HTTP request message
func NewRequestMessage(req HTTPRequest) (Message, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return Message{}, err
	}
	return Message{
		Type:    MsgTypeRequest,
		Payload: data,
	}, nil
}

// NewResponseMessage creates an HTTP response message
func NewResponseMessage(resp HTTPResponse) (Message, error) {
	data, err := json.Marshal(resp)
	if err != nil {
		return Message{}, err
	}
	return Message{
		Type:    MsgTypeResponse,
		Payload: data,
	}, nil
}
