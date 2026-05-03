# Cost Profiler

**Профилирование производительности с привязкой к финансовым метрикам**

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![eBPF](https://img.shields.io/badge/eBPF-Powered-red)](https://ebpf.io)
[![ROI](https://img.shields.io/badge/ROI-2000%25-brightgreen)](https://karamik.github.io/CostProfiler/)
[![Demo](https://img.shields.io/badge/Demo-Landing_Page-green)](https://karamik.github.io/CostProfiler/)
[![Go Report Card](https://goreportcard.com/badge/github.com/karamik/CostProfiler)](https://goreportcard.com/report/github.com/karamik/CostProfiler)
[![GitHub release](https://img.shields.io/github/v/release/karamik/CostProfiler)](https://github.com/karamik/CostProfiler/releases)

---

## 📊 Описание

**Cost Profiler** — enterprise-инструмент для мониторинга производительности кода с расчётом финансового воздействия неоптимальных операций. Работает на основе **eBPF**, **не требует изменений в коде** приложений и незаметен для production-нагрузки (<0.5% CPU).

### 🔥 Ключевые возможности

- 🎯 Перехват вызовов функций на уровне ядра Linux (uprobe/uretprobe)
- ⚡ Измерение CPU cycles вместо wall time для точной оценки нагрузки
- 💰 Расчёт стоимости выполнения функций в рублях на основе инфраструктурных затрат
- 📉 Динамический семплинг для минимизации оверхеда
- 🔄 Интеграция с CI/CD (GitHub Actions) для FinOps-проверок
- 📊 Множество форматов вывода: таблица, JSON, CSV
- 🎨 Красивые графики и сплайны прямо в терминале
- ⚡ **PGO (Profile-Guided Optimization)** — экспорт профиля для автоматической оптимизации компилятором Go
- ☕ **Поддержка Java (JIT + perf‑map)** — профилирование горячих методов без изменения кода
- 🧩 **Kubernetes autodiscovery** — профилирование по имени Pod, а не PID
- 🔍 **Обнаружение inlined функций** — через DWARF (Go)

---

## 🚀 Быстрый старт

### CLI-утилита (рекомендуется)

```bash
# Скачать последний релиз
wget https://github.com/karamik/CostProfiler/releases/latest/download/cost-profiler
chmod +x cost-profiler
sudo mv cost-profiler /usr/local/bin/

# Запустить на Go‑сервисе
sudo cost-profiler --pid $(pgrep my-service) --binary /path/to/binary

# Запустить на Java‑сервисе
sudo cost-profiler --pid $(pgrep java) --lang java --cost 4.5 --duration 30

# Профилирование Pod’а в Kubernetes
sudo cost-profiler --pod my-service-7fbd --namespace prod --lang go --binary /app/service
```

### Демо-стенд

```bash
git clone https://github.com/karamik/CostProfiler
cd CostProfiler
make demo
```

📺 **Живое демо:** [https://karamik.github.io/CostProfiler/](https://karamik.github.io/CostProfiler/)

---

## ⚡ Profile-Guided Optimization (PGO)

Cost Profiler умеет экспортировать собранные метрики в **формат pprof**, который поддерживается компилятором Go для **автоматической оптимизации бинарника**.

```bash
# 1. Собираем профиль на production-нагрузке
sudo cost-profiler --pid $(pgrep my-service) --binary /opt/app/service --duration 60 --export-pgo profile.pprof

# 2. Пересобираем приложение с этим профилем
go build -pgo=profile.pprof -o my-service-optimized
```

➡️ Ускорение hot-путей до 15‑20% без изменения кода.

---

## ☕ Поддержка Java

Cost Profiler профилирует **JIT‑скомпилированные методы** Java через парсинг `/tmp/perf-<pid>.map`. Для этого JVM должна быть запущена с флагами:

```bash
java -XX:+UnlockDiagnosticVMOptions \
     -XX:+DebugNonSafepoints \
     -XX:+PreserveFramePointer \
     -jar your-app.jar
```

Затем запустите профайлер:

```bash
sudo cost-profiler --pid $(pgrep java) --lang java --cost 4.5
```

Агент динамически отслеживает появление новых JIT‑методов и автоматически привязывает uprobe.

---

## 🧩 Kubernetes autodiscovery

Вместо ручного поиска PID контейнера можно указать имя Pod’а:

```bash
sudo cost-profiler --pod my-pod --namespace default --lang go --binary /app/service
```

Агент сам определит PID первого контейнера (или укажите `--container`).  
Для работы внутри кластера необходим сервисный аккаунт с правами на чтение Pod’ов.

---

## 🔍 Работа с inlined функциями (Go)

При сборке Go‑бинарника с отладочной информацией (`-ldflags="-s=false"`) Cost Profiler читает **DWARF** и определяет даже функции, которые были встроены компилятором.  
Для таких функций uprobe не ставится (это невозможно), но они **отображаются в отчётах** с пометкой `(inlined)` – вы будете знать, какие участки кода были инлайнированы.

---

## 🏗 Как это работает

```
┌─────────────────────────────┐     ┌─────────────────────────────┐
│   Приложение (Go/Java)      │     │   eBPF Agent (Userspace)    │
│                             │◄────┤  uprobe / uretprobe         │
│  func SlowRoute() { ... }   │     │  (без изменения кода)       │
│  java.com.Example.run()     │     │  Семплинг: 1/100–1/10000     │
└─────────────┬───────────────┘     └─────────────┬───────────────┘
              │                                   │
              ▼                                   ▼
┌───────────────────────────────────────────────────────────────┐
│              Ядро Linux (eBPF Virtual Machine)                │
│  • Перехват входа/выхода из функций                           │
│  • Подсчёт CPU cycles                                         │
│  • Оверхед <0.5%                                              │
└───────────────────────────────┬───────────────────────────────┘
                                │
                                ▼
┌───────────────────────────────────────────────────────────────┐
│              FinOps Pipeline (ClickHouse + Grafana)           │
│  • Топ дорогих функций с суммарной стоимостью                 │
│  • Привязка к коммитам и разработчикам                        │
│  • Рекомендации по оптимизации                                │
│  • Экспорт PGO профиля → go build -pgo                        │
└───────────────────────────────────────────────────────────────┘
```

---

## 📦 Архитектура проекта

| Компонент | Технологии | Назначение |
|-----------|-----------|------------|
| **CLI Tool** | Go, eBPF, K8s client | Основная утилита |
| **eBPF Agent** | C/eBPF | Перехват syscalls / uprobe |
| **Java support** | perf‑map, fsnotify | JIT‑методы |
| **DWARF parser** | debug/elf, debug/dwarf | Inlined функции |
| **Demo Service** | Go | Тестовый стенд |
| **Storage** | ClickHouse | Временные ряды (Enterprise) |
| **Visualization** | Grafana | Дашборды (Enterprise) |

---

## 📋 Требования

- **ОС:** Linux 5.7+ (Astra Linux, Ubuntu 20.04+, RHEL 8+, CentOS 8+)
- **Ядро:** поддержка eBPF (CONFIG_BPF, CONFIG_BPF_SYSCALL)
- **Привилегии:** CAP_BPF или root для загрузки eBPF
- **Зависимости:** Go 1.21+, Make, Clang, libbpf-dev

### Установка зависимостей (Ubuntu/Debian)

```bash
sudo apt-get update
sudo apt-get install -y golang make clang llvm libbpf-dev linux-tools-common
```

Для работы с Kubernetes (опционально): настройте `~/.kube/config` или используйте in‑cluster конфигурацию.

---

## 🛠 Установка и сборка

```bash
git clone https://github.com/karamik/CostProfiler
cd CostProfiler
make build          # сборка бинарника
sudo make install   # установка в /usr/local/bin
```

---

## 💻 Использование

### Параметры CLI

| Параметр | Значение по умолчанию | Описание |
|----------|----------------------|-----------|
| `--pid` | – | PID целевого процесса |
| `--pod` | – | Имя Pod (вместо `--pid`) |
| `--namespace` | `default` | Namespace Pod |
| `--container` | первый контейнер | Имя контейнера в Pod |
| `--binary` | (обязателен для Go) | Путь к бинарному файлу |
| `--lang` | `go` | `go` или `java` |
| `--cost` | 2.0 | Стоимость vCPU часа (₽) |
| `--scale` | 100000000 | Вызовов в сутки (для годовой проекции) |
| `--sampling` | 1 | Семплинг (1 = каждый вызов) |
| `--filter` | "" | Фильтр функций (префикс для Go, подстрока для Java) |
| `--format` | `table` | `table`, `json`, `csv` |
| `--duration` | 30 | Длительность сбора (сек) |
| `--export-pgo` | "" | Экспорт профиля в `pprof` |

### Примеры

```bash
# Go
sudo cost-profiler --pid 12345 --binary ./app --cost 3.5 --export-pgo profile.pprof

# Java (с предварительно запущенной JVM)
sudo cost-profiler --pid $(pgrep java) --lang java --cost 4.5 --duration 60

# Kubernetes pod
sudo cost-profiler --pod my-app-7fbd --namespace prod --lang go --binary /app/service
```

---

## 📊 Пример вывода (Java)

```bash
$ sudo cost-profiler --pid 27182 --lang java --duration 15 --cost 4.5

    ██████╗ ██████╗ ███████╗████████╗
   ██╔════╝██╔═══██╗██╔════╝╚══██╔══╝
   ██║     ██║   ██║███████╗   ██║   
   ██║     ██║   ██║╚════██║   ██║   
   ╚██████╗╚██████╔╝███████║   ██║   
    ╚═════╝ ╚═════╝ ╚══════╝   ╚═╝   

💰 FinOps для high-load систем | eBPF Enterprise Edition

✅ Целевой процесс: PID 27182, язык: java
🔍 Обнаружено Java JIT-методов: 213
  🔗 Привязан: org.apache.catalina.core.StandardEngine.startInternal
  🔗 Привязан: com.x5.logistics.RouteCalculator.calculate
  🔗 (JIT) Привязан: java.util.HashMap.putVal

📊 Сбор метрик в течение 15 секунд...

┌─────────────────────────────────────────────┬──────────┬──────────┬──────────────┬─────────────────┐
│                  Функция                    │ Вызовов  │ CPU (мс) │  ₽/вызов     │  Затраты/год    │
├─────────────────────────────────────────────┼──────────┼──────────┼──────────────┼─────────────────┤
│ com.x5.logistics.RouteCalculator.calculate  │ 15234    │ 234.56   │ 0.00029324   │ 843.20 млн ₽   │
│ java.util.HashMap.putVal                    │ 8921     │ 87.23    │ 0.00010904   │ 312.45 млн ₽   │
│ org.apache.tomcat.util.threads.TaskQueue.take│ 5000     │ 12.34    │ 0.00001542   │ 44.10 млн ₽    │
├─────────────────────────────────────────────┼──────────┼──────────┼──────────────┼─────────────────┤
│                                             │          │          │ ИТОГО:       │ 1.20 млрд ₽    │
└─────────────────────────────────────────────┴──────────┴──────────┴──────────────┴─────────────────┘

💡 Рекомендации по оптимизации:
  • com.x5.logistics.RouteCalculator.calculate: пересмотреть алгоритм (экономия до 590 млн ₽/год)
  • Для Java используйте -XX:+PreserveFramePointer для более точных стеков
```

---

## 🔄 Интеграция с CI/CD

```yaml
# .github/workflows/finops.yml
name: FinOps Check
on: [pull_request]
jobs:
  cost-check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Setup Go
        uses: actions/setup-go@v5
        with: { go-version: '1.21' }
      - name: Build service
        run: go build -o service main.go
      - name: Run Cost Profiler
        run: |
          ./service &
          PID=$!
          sudo cost-profiler --pid $PID --binary ./service --scale 10000000 --format json --export-pgo profile.pprof
          kill $PID
      - name: Optimize with PGO
        run: go build -pgo=profile.pprof -o service-optimized main.go
```

---

## 📈 ROI и экономический эффект

| Показатель | Значение |
|------------|----------|
| Выявленные неоптимальности | 300–500 млн ₽/год |
| Экономия на инфраструктуре | 150–200 млн ₽/год |
| Ускорение расчётов | +15% пропускной способности |
| Снижение облачных расходов | 20‑30% |
| Дополнительный выигрыш от PGO | +10‑20% без изменения кода |
| Стоимость Enterprise-лицензии (3 года) | 25 млн ₽ |
| **ROI** | **2000%+** |

---

## ❓ Частые вопросы

**Q: Как профилировать Java без изменений в коде?**  
A: Запустите JVM с флагами `-XX:+PreserveFramePointer` и Cost Profiler автоматически подхватит JIT‑методы через `/tmp/perf-*.map`. Для интерпретируемого кода потребуется дополнительная настройка (в плане).

**Q: Почему некоторые Go‑функции не видны?**  
A: Возможно, они были инлайнированы. Включите отладочную информацию (`-ldflags="-s=false"`) – тогда утилита покажет их через DWARF (но uprove привязать невозможно). Для точного профилирования добавьте `//go:noinline`.

**Q: Как запустить в Kubernetes?**  
A: Используйте флаг `--pod` и убедитесь, что у вас есть доступ к API кластера. Внутри пода с Cost Profiler потребуется сервисный аккаунт с role `get pods`.

**Q: Влияет ли eBPF на производительность?**  
A: Оверхед <0.5% CPU при семплинге 1/100. Для сверхвысоких нагрузок рекомендуется семплинг 1/1000.

**Q: Можно ли получить профиль для `go build -pgo` из Java?**  
A: Нет, PGO работает только для Go. Для Java используйте стандартные средства JVM (JITWatch, async-profiler).

---

## 🗺 Roadmap

- [x] Базовый eBPF агент на Python
- [x] CLI-утилита на Go с eBPF
- [x] Автообнаружение функций из ELF (Go)
- [x] Множество форматов вывода (table, json, csv)
- [x] Экспорт PGO профиля (`--export-pgo`)
- [x] **Поддержка Java (JIT + perf‑map)**
- [x] **Kubernetes autodiscovery (--pod)**
- [x] **Обнаружение inlined функций (DWARF)**
- [ ] Поддержка Python (инструментирование интерпретатора)
- [ ] Web-интерфейс для просмотра метрик
- [ ] Интеграция с Prometheus + Grafana
- [ ] Машинное обучение для предсказания cost-регрессий

---

## 🤝 Контрибьютинг

1. Форкните репозиторий
2. Создайте ветку (`git checkout -b feature/amazing`)
3. Закоммитьте изменения (`git commit -m 'Add amazing feature'`)
4. Запушьте (`git push origin feature/amazing`)
5. Откройте Pull Request

---

## 📄 Лицензия

- **Open Source ядро:** Apache 2.0
- **Enterprise-модули:** коммерческая лицензия с поддержкой

Enterprise‑версия включает:
- Приоритетную поддержку 24/7
- SLA с гарантированным временем ответа
- Интеграции (Jira, ServiceNow, Prometheus)
- Обучающие сессии для команды
- Юридическую гарантию

---

## 📞 Контакты

- **GitHub:** [https://github.com/karamik/CostProfiler](https://github.com/karamik/CostProfiler)
- **Demo:** [https://karamik.github.io/CostProfiler/](https://karamik.github.io/CostProfiler/)
- **Enterprise запросы:** [totalprotocol@proton.me](mailto:totalprotocol@proton.me)
- **Telegram:** [@tec_support_bot](https://t.me/tec_support_bot)

--
