#!/bin/bash
set -e

cd $(dirname $0)

echo "Building test service..."
go build -o test-service ../main.go

echo "Starting service without profiler..."
./test-service &
SERVICE_PID=$!
sleep 2

echo "Collecting baseline CPU usage (10 seconds)..."
pidstat -p $SERVICE_PID 1 10 > baseline_cpu.txt

kill $SERVICE_PID
sleep 2

echo "Starting service WITH profiler..."
./test-service &
SERVICE_PID=$!
sleep 2

echo "Running cost-profiler for 10 seconds..."
sudo ../cost-profiler --pid $SERVICE_PID --binary ./test-service --duration 10 &
PROFILER_PID=$!
sleep 2

echo "Collecting CPU usage with profiler..."
pidstat -p $SERVICE_PID 1 10 > profiler_cpu.txt

kill $SERVICE_PID
wait $PROFILER_PID 2>/dev/null

echo "=== Baseline average CPU% ==="
grep -E "Average|^[0-9]" baseline_cpu.txt | tail -n1
echo "=== With Profiler average CPU% ==="
grep -E "Average|^[0-9]" profiler_cpu.txt | tail -n1
