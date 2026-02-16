#!/bin/bash
set -e

OLLAMA_HOST=${OLLAMA_HOST:-http://localhost:11434}

echo "🔍 Waiting for Ollama to be ready..."
for i in {1..30}; do
    if curl -s "$OLLAMA_HOST/api/tags" > /dev/null 2>&1; then
        echo "✅ Ollama is ready!"
        break
    fi
    if [ $i -eq 30 ]; then
        echo "❌ Timeout waiting for Ollama"
        exit 1
    fi
    sleep 1
done

echo ""
echo "📥 Pulling image generation model..."
ollama pull x/z-image-turbo:fp8

echo ""
echo "📥 Pulling text models..."
ollama pull gemma3:12b || echo "⚠️  gemma3:12b not available, skipping..."

echo ""
echo "✅ All models ready!"
echo ""
ollama list
