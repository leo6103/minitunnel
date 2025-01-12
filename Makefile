.PHONY: all build server agent clean certs run-server run-agent help

# Binary names
SERVER_BIN = mt_server
AGENT_BIN = mt_agent

# Build directories
BUILD_DIR = bin
SERVER_SRC = ./cmd/server
AGENT_SRC = ./cmd/agent

# Default target
all: build

# Build both binaries
build: server agent

# Build server binary
server:
	@echo "Building mt_server..."
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(SERVER_BIN) $(SERVER_SRC)
	@echo "✓ Built $(BUILD_DIR)/$(SERVER_BIN)"

# Build agent binary
agent:
	@echo "Building mt_agent..."
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(AGENT_BIN) $(AGENT_SRC)
	@echo "✓ Built $(BUILD_DIR)/$(AGENT_BIN)"

# Generate TLS certificates
certs:
	@echo "Generating TLS certificates..."
	@chmod +x scripts/generate-certs.sh
	@./scripts/generate-certs.sh

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@echo "✓ Cleaned"

# Run server (builds first if needed)
run-server: server certs
	@echo "Starting mt_server..."
	@./$(BUILD_DIR)/$(SERVER_BIN)

# Run agent (builds first if needed)
run-agent: agent
	@echo "Starting mt_agent..."
	@./$(BUILD_DIR)/$(AGENT_BIN)

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy
	@echo "✓ Dependencies ready"

# Run tests
test:
	@echo "Running tests..."
	@go test ./internal/...

# Help
help:
	@echo "Minitunnel - Build System"
	@echo ""
	@echo "Usage:"
	@echo "  make build       - Build both mt_server and mt_agent"
	@echo "  make server      - Build only mt_server"
	@echo "  make agent       - Build only mt_agent"
	@echo "  make certs       - Generate TLS certificates"
	@echo "  make clean       - Remove build artifacts"
	@echo "  make run-server  - Build and run server"
	@echo "  make run-agent   - Build and run agent"
	@echo "  make deps        - Download Go dependencies"
	@echo "  make test        - Run tests"
	@echo "  make help        - Show this help message"
