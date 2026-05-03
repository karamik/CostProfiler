.PHONY: generate build clean install run demo help

# Путь к сгенерированным файлам eBPF
BPF_SRC=ebpf/program.c
BPF_OUT=ebpf/program_bpfel.go

# Флаги для сборки
GO_FLAGS=-ldflags="-s -w"
PGO_FILE=profile.pprof

# Генерация eBPF Go-обёртки
generate:
	@echo "🔧 Генерация eBPF Go-обёртки..."
	go generate ./...

# Обычная сборка
build: generate
	@echo "🔨 Сборка cost-profiler..."
	go build $(GO_FLAGS) -o cost-profiler ./cmd/cost-profiler

# Сборка с PGO-оптимизацией (требует готового профиля)
build-pgo: generate
	@echo "🔨 Сборка cost-profiler с PGO (go build -pgo=$(PGO_FILE))..."
	go build -pgo=$(PGO_FILE) $(GO_FLAGS) -o cost-profiler-pgo ./cmd/cost-profiler

# Очистка
clean:
	@echo "🧹 Очистка..."
	rm -f cost-profiler cost-profiler-pgo
	rm -f ebpf/*_bpfel.go ebpf/*_bpfeb.go ebpf/*.o
	rm -f $(PGO_FILE)

# Установка в систему
install: build
	@echo "📦 Установка в /usr/local/bin..."
	sudo cp cost-profiler /usr/local/bin/

# Запуск на тестовом сервисе (требуется запущенный logistics-service)
run: build
	@echo "🚀 Запуск на тестовом сервисе..."
	@pid=$$(pgrep -f "logistics-service" || echo ""); \
	if [ -z "$$pid" ]; then \
		echo "⚠️  logistics-service не запущен. Сначала запустите: ./logistics-service &"; \
		exit 1; \
	fi; \
	sudo ./cost-profiler --pid $$pid --binary ./logistics-service

# Полная демонстрация (запускает тестовый сервис, собирает профиль и сохраняет PGO)
demo: build
	@echo "🚀 Запуск полной демонстрации..."
	@echo "→ Компиляция тестового сервиса..."
	@go build -ldflags="-s=false" -o logistics-service main.go
	@echo "→ Запуск тестового сервиса..."
	@./logistics-service & \
	SERVICE_PID=$$!; \
	echo "  PID: $$SERVICE_PID"; \
	sleep 2; \
	echo "→ Запуск cost-profiler (сбор метрик 15 секунд, экспорт PGO в $(PGO_FILE))..."; \
	sudo ./cost-profiler --pid $$SERVICE_PID --binary ./logistics-service --duration 15 --export-pgo $(PGO_FILE) || true; \
	echo "→ Остановка тестового сервиса..."; \
	kill $$SERVICE_PID 2>/dev/null; \
	echo ""; \
	if [ -f "$(PGO_FILE)" ]; then \
		echo "✅ PGO профиль сохранён в $(PGO_FILE)"; \
		echo "💡 Для оптимизации: make build-pgo"; \
	else \
		echo "⚠️  PGO профиль не создан (возможно, eBPF не поддерживается или нет прав)"; \
	fi

# Помощь
help:
	@echo "Доступные команды:"
	@echo "  make generate  - Генерация eBPF кода"
	@echo "  make build     - Сборка cost-profiler (обычная)"
	@echo "  make build-pgo - Сборка cost-profiler с PGO (требуется файл profile.pprof)"
	@echo "  make clean     - Очистка сгенерированных файлов и бинарников"
	@echo "  make install   - Установка cost-profiler в /usr/local/bin"
	@echo "  make run       - Запуск на уже работающем logistics-service"
	@echo "  make demo      - Полная демонстрация: тестовый сервис + сбор профиля + PGO экспорт"
	@echo "  make help      - Показать эту справку"
