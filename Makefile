.PHONY: generate build build-pgo clean install run demo benchmark docker-build k8s-deploy help

# Путь к сгенерированным файлам eBPF
BPF_SRC=ebpf/program.c
BPF_OUT=ebpf/program_bpfel.go

# Флаги для сборки
GO_FLAGS=-ldflags="-s -w"
PGO_FILE=profile.pprof

# Docker image
IMAGE_NAME=karamik/cost-profiler
IMAGE_TAG?=latest

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
	rm -f benchmarks/test-service

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

# Бенчмарк оверхеда (требует установленного pidstat из sysstat)
benchmark: build
	@echo "📊 Запуск бенчмарка оверхеда..."
	@cd benchmarks && chmod +x run_benchmark.sh && ./run_benchmark.sh

# Сборка Docker образа
docker-build:
	@echo "🐳 Сборка Docker образа $(IMAGE_NAME):$(IMAGE_TAG)..."
	docker build -t $(IMAGE_NAME):$(IMAGE_TAG) -f Dockerfile .

# Деплой в Kubernetes (применяет манифесты из k8s/ после замены переменных)
k8s-deploy: docker-build
	@echo "☸️ Деплой в Kubernetes..."
	@if [ -z "$(TARGET_POD)" ]; then \
		echo "❌ Укажите TARGET_POD=имя-пода для профилирования"; \
		exit 1; \
	fi
	@sed "s/your-target-pod/$(TARGET_POD)/g" k8s/deployment.yaml | kubectl apply -f -
	@kubectl apply -f k8s/rbac.yaml
	@kubectl apply -f k8s/service.yaml
	@echo "✅ Деплой завершён. Порт пробросьте: kubectl port-forward svc/cost-profiler-service 8080:8080"

# Остановка деплоя в Kubernetes
k8s-down:
	@echo "🗑️ Удаление ресурсов Kubernetes..."
	-kubectl delete -f k8s/deployment.yaml 2>/dev/null || true
	-kubectl delete -f k8s/rbac.yaml 2>/dev/null || true
	-kubectl delete -f k8s/service.yaml 2>/dev/null || true

# Помощь
help:
	@echo "Доступные команды:"
	@echo "  make generate      - Генерация eBPF кода"
	@echo "  make build         - Сборка cost-profiler (обычная)"
	@echo "  make build-pgo     - Сборка cost-profiler с PGO (требуется profile.pprof)"
	@echo "  make clean         - Очистка сгенерированных файлов и бинарников"
	@echo "  make install       - Установка cost-profiler в /usr/local/bin"
	@echo "  make run           - Запуск на уже работающем logistics-service"
	@echo "  make demo          - Полная демонстрация (тестовый сервис + PGO экспорт)"
	@echo "  make benchmark     - Измерение оверхеда CPU (требуется sysstat)"
	@echo "  make docker-build  - Сборка Docker образа (IMAGE_TAG=...)"
	@echo "  make k8s-deploy    - Деплой в Kubernetes (TARGET_POD=имя-пода)"
	@echo "  make k8s-down      - Удалить ресурсы Kubernetes"
	@echo "  make help          - Показать эту справку"
