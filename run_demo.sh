#!/bin/bash

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Cleanup function
cleanup() {
    echo -e "\n${YELLOW}🛑 Shutting down...${NC}"
    
    # Kill the Go service if running
    if [ ! -z "$SERVICE_PID" ] && kill -0 $SERVICE_PID 2>/dev/null; then
        echo -e "${YELLOW}Stopping logistics service (PID: $SERVICE_PID)...${NC}"
        kill $SERVICE_PID 2>/dev/null
        wait $SERVICE_PID 2>/dev/null
    fi
    
    # Kill any remaining eBPF agent processes
    sudo pkill -f "ebpf_agent.py" 2>/dev/null
    
    echo -e "${GREEN}✅ Cleanup complete${NC}"
    exit 0
}

# Set trap for Ctrl+C
trap cleanup SIGINT SIGTERM

# Check if main.go exists
if [ ! -f "main.go" ]; then
    echo -e "${RED}❌ Error: main.go not found in current directory${NC}"
    echo -e "${YELLOW}Make sure you're in the CostProfiler directory${NC}"
    exit 1
fi

# Check if ebpf_agent.py exists
if [ ! -f "ebpf_agent.py" ]; then
    echo -e "${RED}❌ Error: ebpf_agent.py not found${NC}"
    exit 1
fi

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo -e "${RED}❌ Error: Go is not installed${NC}"
    echo -e "${YELLOW}Install Go: https://golang.org/dl/${NC}"
    exit 1
fi

# Check if python3 is installed
if ! command -v python3 &> /dev/null; then
    echo -e "${RED}❌ Error: Python3 is not installed${NC}"
    exit 1
fi

echo -e "${CYAN}🚀 Building Go service...${NC}"
go build -ldflags="-s=false" -o logistics-service main.go

if [ $? -ne 0 ]; then
    echo -e "${RED}❌ Build failed${NC}"
    exit 1
fi

echo -e "${CYAN}🚀 Starting logistics service...${NC}"
./logistics-service &
SERVICE_PID=$!
echo -e "${GREEN}✅ Service PID: $SERVICE_PID${NC}"
sleep 2

# Check if service is still running
if ! kill -0 $SERVICE_PID 2>/dev/null; then
    echo -e "${RED}❌ Service crashed immediately${NC}"
    cleanup
    exit 1
fi

echo -e "${CYAN}🔬 Attaching eBPF agent...${NC}"
echo -e "${YELLOW}⚠️  eBPF requires root privileges (sudo)${NC}"
echo -e "${YELLOW}⚠️  If this is your first time, enter your password${NC}"
echo ""

sudo python3 ebpf_agent.py --pid $SERVICE_PID --binary ./logistics-service

# If we get here, eBPF agent exited (user pressed Ctrl+C in agent)
cleanup
