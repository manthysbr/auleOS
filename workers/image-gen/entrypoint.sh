#!/bin/bash
set -e

echo "Starting Ollama daemon..."
ollama serve &
OLLAMA_PID=$!

# Wait for Ollama to be ready
echo "Waiting for Ollama to start..."
for i in {1..30}; do
    if curl -s http://localhost:11434/api/tags > /dev/null 2>&1; then
        echo "Ollama ready!"
        break
    fi
    sleep 1
done

# Pull z-image-turbo model on first run (cached on subsequent runs via volume)
echo "Checking for z-image-turbo model..."
if ! ollama list | grep -q "z-image-turbo"; then
    echo "Pulling z-image-turbo model (this may take 1-2 minutes on first run)..."
    ollama pull z-image-turbo
else
    echo "z-image-turbo model ready!"
fi

# Start watchdog
echo "Starting watchdog server..."
exec /usr/local/bin/watchdog
