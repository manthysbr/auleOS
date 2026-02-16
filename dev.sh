#!/bin/bash

# Definition of colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${BLUE}🚀 Starting auleOS Development Environment...${NC}"

# Function to handle cleanup on exit
cleanup() {
    echo -e "\n${RED}🛑 Shutting down services...${NC}"
    kill $(jobs -p) 2>/dev/null
    exit
}

# Trap signals to ensure we kill child processes
trap cleanup SIGINT SIGTERM

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

# Start Kernel Backend
echo -e "${GREEN}📦 Starting Kernel (Backend)...${NC}"
# redirects stdout/stderr to utilize separate terminal or file if needed, 
# for now we mix them but prefix could be good. 
# keeping it simple:
go run cmd/aule-kernel/main.go &
KERNEL_PID=$!

# Wait a bit for backend to initialize
sleep 2

# Start Web Frontend
echo -e "${GREEN}🌐 Starting Web Interface (Frontend)...${NC}"
cd web && npm run dev &
WEB_PID=$!

echo -e "${BLUE}✨ Environment is READY!${NC}"
echo -e "${BLUE}👉 Frontend: http://localhost:5173${NC}"
echo -e "${BLUE}👉 Kernel:   http://localhost:8080${NC}"
echo -e "${BLUE}Press Ctrl+C to stop.${NC}"

# Wait for both processes
wait
