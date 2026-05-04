# Этап 1: сборка бинарника
FROM golang:1.21-alpine AS builder

# Устанавливаем необходимые пакеты для сборки eBPF и Go
RUN apk add --no-cache make gcc musl-dev clang llvm libbpf-dev linux-headers

WORKDIR /build

# Копируем go.mod и go.sum для кеширования зависимостей
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходный код
COPY . .

# Генерируем eBPF обёртки и собираем статический бинарник
RUN make generate && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
        -ldflags="-s -w -extldflags '-static'" \
        -o cost-profiler ./cmd/cost-profiler

# Этап 2: минимальный финальный образ
FROM alpine:latest

# Устанавливаем ca-certificates и базовые утилиты (опционально)
RUN apk add --no-cache ca-certificates

# Копируем бинарник из builder
COPY --from=builder /build/cost-profiler /usr/local/bin/cost-profiler

# Команда по умолчанию (показываем help)
ENTRYPOINT ["cost-profiler"]
CMD ["--help"]
