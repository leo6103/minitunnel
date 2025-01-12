#!/bin/bash

echo "=== Testing Minitunnel ==="
echo ""

# Kill any existing processes
pkill -f mt_server
pkill -f mt_agent
sleep 1

# Start server in background
echo "Starting server..."
./bin/mt_server > /tmp/mt_server.log 2>&1 &
SERVER_PID=$!
echo "Server PID: $SERVER_PID"
sleep 2

# Check if server is running
if ! ps -p $SERVER_PID > /dev/null; then
    echo "❌ Server failed to start"
    cat /tmp/mt_server.log
    exit 1
fi

echo "✓ Server started"
echo ""

# Start agent
echo "Starting agent..."
timeout 10 ./bin/mt_agent 2>&1 | tee /tmp/mt_agent.log &
AGENT_PID=$!

# Wait a bit for connection
sleep 3

# Check logs
echo ""
echo "=== Server Log ==="
cat /tmp/mt_server.log
echo ""
echo "=== Agent Log ==="
cat /tmp/mt_agent.log

# Cleanup
echo ""
echo "Cleaning up..."
kill $SERVER_PID $AGENT_PID 2>/dev/null
wait $SERVER_PID $AGENT_PID 2>/dev/null

echo "Done!"
