#!/bin/bash

# Create certs directory
mkdir -p certs

# Generate private key
openssl genrsa -out certs/server.key 2048

# Generate self-signed certificate
openssl req -new -x509 -sha256 -key certs/server.key -out certs/server.crt -days 365 \
  -subj "/C=US/ST=State/L=City/O=Minitunnel/CN=localhost"

echo "✓ TLS certificates generated in certs/"
echo "  - certs/server.key"
echo "  - certs/server.crt"
