#!/bin/bash
echo "🚀 Building Go service..."
go build -ldflags="-s=false" -o logistics-service main.go

echo "🚀 Starting service..."
./logistics-service &
SERVICE_PID=$!
echo "Service PID: $SERVICE_PID"
sleep 2

echo "🔬 Attaching eBPF agent (requires sudo)..."
sudo python3 ebpf_agent.py --pid $SERVICE_PID --binary ./logistics-service

kill $SERVICE_PID 2>/dev/null
