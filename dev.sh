#!/bin/bash

set -o pipefail

# Definition of colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${BLUE}🚀 Starting auleOS Development Environment...${NC}"

KERNEL_PID=""
WEB_PID=""
RELOAD_MODE=false

for arg in "$@"; do
    case "$arg" in
        --reload)
            RELOAD_MODE=true
            ;;
        *)
            echo -e "${BLUE}ℹ️ Ignoring unknown argument: $arg${NC}"
            ;;
    esac
done

find_free_port() {
    local port=$1
    while lsof -iTCP:"$port" -sTCP:LISTEN -t >/dev/null 2>&1; do
        port=$((port + 1))
    done
    echo "$port"
}

kill_port_listener() {
    local port=$1
    local pids
    pids=$(lsof -tiTCP:"$port" -sTCP:LISTEN 2>/dev/null || true)
    if [ -z "$pids" ]; then
        return
    fi

    echo -e "${BLUE}🔧 Releasing port ${port} (PIDs: ${pids})...${NC}"
    kill $pids 2>/dev/null || true
    sleep 1

    for pid in $pids; do
        if kill -0 "$pid" 2>/dev/null; then
            kill -9 "$pid" 2>/dev/null || true
        fi
    done
}

# Function to handle cleanup on exit
cleanup() {
    local exit_code=${1:-0}
    echo -e "\n${RED}🛑 Shutting down services...${NC}"
    if [ -n "$WEB_PID" ]; then
        kill "$WEB_PID" 2>/dev/null
    fi
    if [ -n "$KERNEL_PID" ]; then
        kill "$KERNEL_PID" 2>/dev/null
    fi
    exit "$exit_code"
}

# Trap signals to ensure we kill child processes
trap 'cleanup 0' SIGINT SIGTERM

if [ "$RELOAD_MODE" = true ]; then
    echo -e "${BLUE}♻️ Reload mode enabled: killing previous local processes and rebuilding from scratch...${NC}"

    kill_port_listener 8080
    for port in $(seq 5173 5190); do
        kill_port_listener "$port"
    done

    pkill -f "bin/aule-kernel" 2>/dev/null || true
    pkill -f "cmd/aule-kernel/main.go" 2>/dev/null || true
fi

# Check for Docker
if ! command -v docker &> /dev/null; then
    echo -e "${RED}❌ Docker could not be found. Please install Docker or enable WSL integration.${NC}"
    exit 1
fi

if ! docker info &> /dev/null; then
    echo -e "${RED}❌ Docker daemon is not running or not accessible. Try 'sudo service docker start' or check Docker Desktop settings.${NC}"
    exit 1
fi

# Start Ollama orchestrator if not running
if ! docker ps | grep -q aule-ollama; then
    echo -e "${BLUE}🧠 Starting Ollama orchestrator...${NC}"
    docker-compose -f docker-compose.ollama.yml up -d
    sleep 2
else
    echo -e "${GREEN}✅ Ollama orchestrator already running${NC}"
fi

# Build Kernel binary
echo -e "${GREEN}🛠️ Building Kernel binary...${NC}"
mkdir -p bin
if ! go build -o bin/aule-kernel ./cmd/aule-kernel; then
    echo -e "${RED}❌ Failed to build kernel binary.${NC}"
    cleanup 1
fi

# Start Kernel Backend
echo -e "${GREEN}📦 Starting Kernel (Backend)...${NC}"
AULE_DB_PATH="${AULE_DB_PATH:-aule-dev.db}" ./bin/aule-kernel &
KERNEL_PID=$!

# Wait a bit for backend to initialize
sleep 2

if ! kill -0 "$KERNEL_PID" 2>/dev/null; then
    echo -e "${RED}❌ Kernel failed to start. Check logs above.${NC}"
    cleanup 1
fi

# Wait for API readiness
KERNEL_READY=0
for _ in {1..20}; do
    if curl -sf http://localhost:8080/v1/jobs >/dev/null 2>&1; then
        KERNEL_READY=1
        break
    fi
    sleep 0.5
done

if [ "$KERNEL_READY" -ne 1 ]; then
    echo -e "${RED}❌ Kernel did not become ready on http://localhost:8080.${NC}"
    cleanup 1
fi

# Start Web Frontend
echo -e "${GREEN}🌐 Starting Web Interface (Frontend)...${NC}"
WEB_PORT=$(find_free_port 5173)
(cd web && npm run dev -- --port "$WEB_PORT") &
WEB_PID=$!

echo -e "${BLUE}✨ Environment is READY!${NC}"
echo -e "${BLUE}👉 Frontend: http://localhost:${WEB_PORT}${NC}"
echo -e "${BLUE}👉 Kernel:   http://localhost:8080${NC}"
echo -e "${BLUE}👉 DB Path:  ${AULE_DB_PATH:-aule-dev.db}${NC}"
echo -e "${BLUE}Press Ctrl+C to stop.${NC}"

# Wait for both processes
wait
