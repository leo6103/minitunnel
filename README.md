# Minitunnel

A lightweight HTTP tunneling tool similar to ngrok, built with Go and QUIC protocol.

## Features

- Fast and reliable tunneling using QUIC protocol
- TLS encryption for secure connections
- Unique tunnel URLs for each agent
- Automatic connection keep-alive
- HTTP request/response forwarding
- Works with modern web frameworks (Next.js, React, etc.)

## Architecture

- **mt_server**: Server component that accepts agent connections and forwards HTTP requests
- **mt_agent**: Agent component that connects to the server and forwards requests to local services

## Quick Start

### 1. Install Dependencies

```bash
make deps
```

### 2. Generate TLS Certificates

```bash
make certs
```

### 3. Build Binaries

```bash
make build
```

### 4. Run Server

```bash
./bin/mt_server
```

### 5. Run Agent (in another terminal)

Simple syntax:
```bash
./bin/mt_agent http 3000
```

Or with flags for more control:
```bash
./bin/mt_agent -server localhost:8080 -local localhost:3000
```

The agent will display a tunnel URL like: `http://localhost:8081/<uuid>`

### 6. Test the Tunnel

Send requests to the tunnel URL:
```bash
curl http://localhost:8081/<uuid>/
```

## Configuration

### Server Options

```bash
./bin/mt_server [options]
```

- `-port`: Port to listen on (default: 8080)
- `-cert`: TLS certificate file (default: certs/server.crt)
- `-key`: TLS key file (default: certs/server.key)

### Agent Options

Simple syntax:
```bash
./bin/mt_agent http <port>
```
Connects to `localhost:8080` and forwards to `localhost:<port>`

Advanced syntax:
```bash
./bin/mt_agent [options]
```

- `-server`: Server address (default: localhost:8080)
- `-local`: Local service address to forward to (default: localhost:3000)
- `-insecure`: Skip TLS verification for self-signed certs (default: true)

Examples:
```bash
# Forward local port 3000
./bin/mt_agent http 3000

# Forward local port 8000
./bin/mt_agent http 8000

# Connect to remote server
./bin/mt_agent -server example.com:8080 -local localhost:3000
```

## Development

### Build Commands

```bash
make build       # Build both binaries
make server      # Build only server
make agent       # Build only agent
make clean       # Remove build artifacts
make test        # Run tests
```

## How It Works

1. Agent connects to server via QUIC
2. Agent sends hello message to establish stream
3. Server assigns a unique UUID and tunnel URL
4. HTTP requests to the tunnel URL are forwarded to the agent
5. Agent forwards requests to the local service
6. Responses are sent back through the tunnel

## Troubleshooting

### UDP Buffer Size Warning

You may see this warning when starting the server or agent:

```
failed to sufficiently increase receive buffer size (was: 208 kiB, wanted: 7168 kiB, got: 416 kiB)
```

This is safe to ignore for development and testing. It's a performance optimization notice, not an error. QUIC uses UDP and prefers larger buffers for optimal performance, but it will work fine with smaller buffers.

To increase the buffer size on Linux (optional):
```bash
sudo sysctl -w net.core.rmem_max=7500000
sudo sysctl -w net.core.wmem_max=7500000
```

## License

MIT
