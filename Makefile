.PHONY: generate build clean install run demo help

# Путь к сгенерированным файлам
BPF_SRC=ebpf/program.c
BPF_OUT=ebpf/program_bpfel.go

generate:
	@echo "🔧 Генерация eBPF Go-обёртки..."
	go generate ./...

build: generate
	@echo "🔨 Сборка cost-profiler..."
	go build -o cost-profiler ./cmd/cost-profiler

clean:
	@echo "🧹 Очистка..."
	rm -f cost-profiler
	rm -f ebpf/*_bpfel.go ebpf/*_bpfeb.go ebpf/*.o

install: build
	@echo "📦 Установка в /usr/local/bin..."
	sudo cp cost-profiler /usr/local/bin/

run: build
	@echo "🚀 Запуск (требуется запущенный тестовый сервис)..."
	sudo ./cost-profiler --pid $$(pgrep -f "logistics-service") --binary ./logistics-service

demo:
	@echo "🚀 Запуск полной демонстрации..."
	@go build -ldflags="-s=false" -o logistics-service main.go 2>/dev/null || true
	@./logistics-service & \
	SERVICE_PID=$$!; \
	sleep 2; \
	sudo ./cost-profiler --pid $$SERVICE_PID --binary ./logistics-service --duration 15 || true; \
	kill $$SERVICE_PID 2>/dev/null

help:
	@echo "Доступные команды:"
	@echo "  make generate - Генерация eBPF кода"
	@echo "  make build    - Сборка бинарника"
	@echo "  make clean    - Очистка"
	@echo "  make install  - Установка в систему"
	@echo "  make run      - Запуск на тестовом сервисе"
	@echo "  make demo     - Полная демонстрация"
