package config

import (
	"flag"
	"fmt"
)

// ServerConfig holds server configuration
type ServerConfig struct {
	Port     int
	CertFile string
	KeyFile  string
}

// AgentConfig holds agent configuration
type AgentConfig struct {
	ServerAddr string
	LocalAddr  string
	Insecure   bool // Skip TLS verification for self-signed certs
}

// ParseServerConfig parses server configuration from command line flags
func ParseServerConfig() *ServerConfig {
	cfg := &ServerConfig{}
	flag.IntVar(&cfg.Port, "port", 8080, "Port to listen on")
	flag.StringVar(&cfg.CertFile, "cert", "certs/server.crt", "TLS certificate file")
	flag.StringVar(&cfg.KeyFile, "key", "certs/server.key", "TLS key file")
	flag.Parse()
	return cfg
}

// ParseAgentConfig parses agent configuration from command line flags
func ParseAgentConfig() *AgentConfig {
	cfg := &AgentConfig{}
	flag.StringVar(&cfg.ServerAddr, "server", "localhost:8080", "Server address (host:port)")
	flag.StringVar(&cfg.LocalAddr, "local", "localhost:3000", "Local service address to forward to")
	flag.BoolVar(&cfg.Insecure, "insecure", true, "Skip TLS certificate verification")
	flag.Parse()
	return cfg
}

// Validate validates server configuration
func (c *ServerConfig) Validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("invalid port: %d", c.Port)
	}
	return nil
}

// Validate validates agent configuration
func (c *AgentConfig) Validate() error {
	if c.ServerAddr == "" {
		return fmt.Errorf("server address is required")
	}
	if c.LocalAddr == "" {
		return fmt.Errorf("local address is required")
	}
	return nil
}
