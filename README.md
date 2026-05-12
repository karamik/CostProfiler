
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
- 🌐 **Веб-интерфейс + Prometheus metrics** — дашборд и `/metrics` для сбора в Prometheus
- 🐳 **Docker‑образ** — готов к запуску в контейнере
- ☸️ **Helm-совместимые манифесты** — деплой в Kubernetes за минуту

---

## 📊 Сравнение с конкурентами

| Функция | CostProfiler | Datadog CP | Pyroscope (Grafana Phlare) | Parca |
|---------|-------------|------------|---------------------------|-------|
| **eBPF** | ✅ | ✅ | ✅ | ✅ |
| **FinOps (расчёт стоимости в ₽)** | ✅ | ❌ | ❌ | ❌ |
| **PGO экспорт (pprof)** | ✅ | ❌ | ❌ | ❌ |
| **Java без изменения кода** | ✅ | ✅ (агент) | ❌ | ❌ |
| **K8s autodiscovery (--pod)** | ✅ | ✅ | ✅ | ✅ |
| **Веб-интерфейс** | ✅ | ✅ | ✅ | ✅ |
| **Prometheus metrics** | ✅ | ✅ | ✅ | ✅ |
| **Open Source** | ✅ (ядро) | ❌ | ✅ | ✅ |
| **Оверхед CPU** | <0.5% | ~1-2% | ~1% | ~1% |
| **Стоимость** | Бесплатно / Enterprise | $~15/месяц/хост | Бесплатно | Бесплатно |

**Уникальные преимущества CostProfiler:**
- 🔥 **FinOps из коробки** — вы сразу видите стоимость функций в рублях
- ⚡ **PGO экспорт** — автоматическая оптимизация Go-бинарников компилятором
- ☕ **Java JIT без агента** — достаточно флагов JVM, никаких дополнительных процессов
- 💰 **Бизнес-метрики** — ROI, годовая экономия, окупаемость HSM

---

## 🎯 Автоматическое обнаружение языка (lang)

Начиная с версии 2.0, флаг `--lang` **опционален**. Cost Profiler автоматически определяет язык целевого процесса:

- **Go** — по символам `runtime.main`, `go.version` в ELF
- **Java** — по имени процесса (java, javaw) и наличию JVM-сигнатур в памяти
- **Python (скоро)** — по наличию `Py_Main` и интерпретатора

```bash
# Раньше нужно было указывать --lang
sudo cost-profiler --pid 12345 --lang go --binary ./app

# Теперь достаточно:
sudo cost-profiler --pid 12345 --binary ./app
```

Автоопределение работает для Go и Java. Для редких edge-кейсов флаг `--lang` остаётся для явного указания.

---

## 💾 Сохранение истории в ClickHouse / Prometheus

С версии 2.0 Cost Profiler поддерживает **долгосрочное хранение метрик** через Remote Write в Prometheus или прямую запись в ClickHouse.

### Remote Write в Prometheus

```bash
sudo cost-profiler --pid 12345 --binary ./app \
    --remote-write-url http://prometheus:9090/api/v1/write \
    --remote-write-tenant my-tenant \
    --duration 3600
```

### Прямая запись в ClickHouse

```bash
sudo cost-profiler --pid 12345 --binary ./app \
    --clickhouse-host localhost:9000 \
    --clickhouse-database cost_profiler \
    --clickhouse-table metrics \
    --duration 3600
```

**Схема таблицы ClickHouse (автосоздание):**

```sql
CREATE TABLE IF NOT EXISTS cost_profiler.metrics (
    timestamp DateTime,
    function String,
    calls_total UInt64,
    cpu_cycles_total UInt64,
    annual_cost_rub Float64,
    language String,
    pod_name String,
    namespace String
) ENGINE = MergeTree()
ORDER BY (timestamp, function);
```

После настройки хранения вы можете строить **графики и алерты** за любой период.

---

## 🤖 AI-рекомендации по оптимизации

Cost Profiler анализирует собранные профили и даёт **автоматические рекомендации** по оптимизации кода.

```bash
sudo cost-profiler --pid 12345 --binary ./app --duration 60 --ai-recommendations
```

**Пример вывода:**

```
🔍 AI-анализ профиля (60 секунд):

📈 Функция: main.userLogin (45% CPU, 1200 вызовов/сек)
   - Рекомендация: кэшировать результат проверки прав пользователя на 5 секунд
   - Ожидаемая экономия: 12 млн ₽/год
   - Ссылка: https://docs.costprofiler.io/optimizations/cache-user-auth

📈 Функция: crypto.signRSA (30% CPU, 850 вызовов/сек)
   - Рекомендация: перенести в HSM/FPGA (Sentinel Guard)
   - Ожидаемая экономия: 8.4 млн ₽/год + ускорение 45x
   - Ссылка: https://docs.costprofiler.io/optimizations/hsm-offload

📈 Функция: db.query (15% CPU, 5000 вызовов/сек)
   - Рекомендация: добавить индекс по полю user_id
   - Ожидаемая экономия: 3.2 млн ₽/год
   - Ссылка: https://docs.costprofiler.io/optimizations/db-index
```

AI-модель обучается на вашей инфраструктуре (локально, данные не покидают ваш сервер) или работает в готовом виде с типовыми паттернами оптимизаций.

---

## 🚨 SLO мониторинг и алерты в Slack/PagerDuty

Если функция превышает заданный бюджет, Cost Profiler отправляет алерт.

### Конфигурация бюджета

```yaml
# budget.yaml
functions:
  - pattern: "crypto.*"
    max_annual_cost_rub: 1_000_000
  - pattern: "db.query*"
    max_annual_cost_rub: 500_000
  - pattern: "*"
    max_annual_cost_rub: 100_000
```

### Запуск с алертами

```bash
sudo cost-profiler --pid 12345 --binary ./app \
    --budget-file budget.yaml \
    --slack-webhook https://hooks.slack.com/services/XXX \
    --pagerduty-integration-key YYY \
    --duration 3600
```

### Пример алерта в Slack

```json
{
  "text": "🚨 *Cost Budget Alert*",
  "blocks": [
    {
      "type": "section",
      "text": {
        "type": "mrkdwn",
        "text": "Функция `crypto.signRSA` превысила годовой бюджет.\nФакт: *1.42 млн ₽* | Лимит: *1.0 млн ₽* | Превышение: *42%*"
      }
    },
    {
      "type": "actions",
      "elements": [
        {
          "type": "button",
          "text": { "type": "plain_text", "text": "Смотреть дашборд" },
          "url": "http://cost-profiler:8080"
        }
      ]
    }
  ]
}
```

Поддерживаются интеграции:
- **Slack** (webhook)
- **PagerDuty** (Events API v2)
- **Opsgenie**
- **Telegram** (бот)
- **Prometheus Alertmanager**

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

# Запустить веб-интерфейс (порт 8080) и Prometheus endpoint
sudo cost-profiler --pid 12345 --binary ./app --web :8080 --duration 60
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
Если JVM не создала файл карты сразу, используйте флаг `--java-signal`, который отправит `SIGUSR2` процессу Java для принудительной генерации.

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

## 🌐 Веб-интерфейс и Prometheus

При запуске с флагом `--web :8080` Cost Profiler поднимает HTTP‑сервер с:

- **`/`** – HTML‑дашборд (таблица с метриками, обновляется каждые 5 секунд)
- **`/api/metrics`** – JSON endpoint с теми же данными
- **`/metrics`** – эндпоинт в формате Prometheus (для сбора в Prometheus и отображения в Grafana)

Пример запуска:

```bash
sudo cost-profiler --pid 12345 --binary ./app --web :8080 --duration 3600
```

Теперь можно открыть `http://localhost:8080` и наблюдать за затратами в реальном времени, а также настроить Prometheus для сбора метрик.

---

## 🐳 Docker

Готовый Docker‑образ позволяет запускать Cost Profiler в контейнере:

```bash
docker run --rm --privileged --pid=host \
    -v /proc:/proc:ro -v /tmp:/tmp \
    karamik/cost-profiler:latest \
    --pid 1 --binary /proc/1/exe --web :8080
```

Собрать образ самостоятельно:

```bash
make docker-build
```

---

## ☸️ Kubernetes Deployment

В директории `k8s/` находятся манифесты для быстрого развёртывания:

- `rbac.yaml` – сервисный аккаунт и роли для доступа к Pod'ам
- `deployment.yaml` – запуск профайлера как отдельного пода (следит за целевым подом)
- `service.yaml` – сервис для доступа к веб‑интерфейсу
- `sidecar-example.yaml` – пример sidecar‑контейнера в вашем приложении

Развернуть профайлер в кластере:

```bash
make k8s-deploy TARGET_POD=my-app-pod
```

После этого можно пробросить порт:

```bash
kubectl port-forward svc/cost-profiler-service 8080:8080
```

И открыть `http://localhost:8080`.

---

## 📊 Бенчмарки

Для оценки накладных расходов запустите:

```bash
make benchmark
```

Этот скрипт:

- Запускает тестовый Go‑сервис с нагрузкой
- Измеряет CPU Usage с помощью `pidstat` без профайлера и с ним
- Выводит среднее потребление CPU в обоих случаях

Ожидаемый результат: при семплинге 1/100 оверхед <0.5%.

---

## 🔒 Безопасность

Cost Profiler разработан с учётом требований enterprise-безопасности.

### 🛡️ Минимальные привилегии
- Для работы требуется **только CAP_BPF** (начиная с ядра 5.8) или `sudo`. Мы рекомендуем настроить `sudoers` для команды `cost-profiler` без полного root-доступа.
- Агент **не требует** записи в ФС целевого приложения, не изменяет его код и не оставляет следов.

### 📦 Статическая компиляция
- Бинарник **статически скомпилирован** (включая libbpf). Никаких динамических библиотек.
- Все зависимости проверены через `go mod verify`.

### 🧪 Ограничения eBPF
- Программы eBPF проходят **верификатор ядра** перед загрузкой.
- Uprobe/uretprobe работают в изолированном контексте (eBPF VM).

### 🔐 Безопасная работа с uprobe
- Привязка только к адресам из таблицы символов ELF или `/tmp/perf-*.map`.
- Поддержка семплинга снижает частоту перехватов.

### 📋 Рекомендации по эксплуатации
- Запускайте Cost Profiler на **небоевых контурах** перед production-пилотом.
- Используйте **семиплинг 1/100 или 1/1000** для production.
- Для Kubernetes – отдельный сервисный аккаунт.
- Периодически обновляйте версию агента.

### ✅ Сертификация и аудит
- Предоставляем **SBOM** по запросу.
- **Пентест** бинарника перед каждым релизом.
- Поддержка **Astra Linux Special Edition** (сертификация по запросу).

---

## 📋 Требования

- **ОС:** Linux 5.7+ (Astra Linux, Ubuntu 20.04+, RHEL 8+, CentOS 8+)
- **Ядро:** поддержка eBPF (CONFIG_BPF, CONFIG_BPF_SYSCALL)
- **Привилегии:** CAP_BPF или root
- **Зависимости:** Go 1.21+, Make, Clang, libbpf-dev (только для сборки)

### Установка зависимостей (Ubuntu/Debian)

```bash
sudo apt-get update
sudo apt-get install -y golang make clang llvm libbpf-dev linux-tools-common
```

Для Kubernetes – настройте `~/.kube/config` или используйте in‑cluster конфигурацию.

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
| `--lang` | `auto` | `auto`, `go`, `java` (автоопределение по умолчанию) |
| `--cost` | 2.0 | Стоимость vCPU часа (₽) |
| `--scale` | 100000000 | Вызовов в сутки (годовая проекция) |
| `--sampling` | 1 | Семплинг (1 = каждый вызов) |
| `--filter` | "" | Фильтр функций (префикс/подстрока) |
| `--format` | `table` | `table`, `json`, `csv` |
| `--duration` | 30 | Длительность сбора (сек) |
| `--export-pgo` | "" | Экспорт профиля в `pprof` |
| `--web` | "" | Запустить веб‑сервер (порт, например `:8080`) |
| `--java-signal` | `false` | Отправить SIGUSR2 JVM для генерации perf‑map |
| `--remote-write-url` | "" | URL для Remote Write в Prometheus |
| `--clickhouse-host` | "" | Хост ClickHouse для долгосрочного хранения |
| `--ai-recommendations` | `false` | Включить AI-рекомендации по оптимизации |
| `--budget-file` | "" | YAML файл с бюджетом функций |
| `--slack-webhook` | "" | Webhook URL для Slack алертов |
| `--pagerduty-integration-key` | "" | Integration Key для PagerDuty |

### Примеры

```bash
# Go + веб‑интерфейс + Prometheus
sudo cost-profiler --pid 12345 --binary ./app --web :8080 --duration 3600

# Java с принудительной генерацией карты
sudo cost-profiler --pid $(pgrep java) --lang java --java-signal --web :8080

# Kubernetes pod + фильтр функций
sudo cost-profiler --pod my-app-7fbd --namespace prod --lang go --binary /app/service --filter "main."

# С AI-рекомендациями
sudo cost-profiler --pid 12345 --binary ./app --duration 60 --ai-recommendations

# С алертами в Slack и PagerDuty
sudo cost-profiler --pid 12345 --binary ./app --budget-file budget.yaml --slack-webhook https://hooks.slack.com/services/XXX --pagerduty-integration-key YYY --duration 3600

# С сохранением в ClickHouse
sudo cost-profiler --pid 12345 --binary ./app --clickhouse-host localhost:9000 --duration 3600
```

---

## 📊 Пример вывода (веб‑интерфейс)

После запуска с `--web :8080` откройте `http://localhost:8080`:

![Web Dashboard](https://via.placeholder.com/800x400?text=Cost+Profiler+Dashboard)

Таблица автоматически обновляется каждые 5 секунд.  
Эндпоинт `http://localhost:8080/metrics` выдаёт данные для Prometheus:

```
# HELP cost_profiler_function_calls_total Total number of calls per function
# TYPE cost_profiler_function_calls_total counter
cost_profiler_function_calls_total{function="main.SlowRouteOptimization"} 15234
cost_profiler_function_calls_total{function="main.FastRouteOptimization"} 100000
# HELP cost_profiler_function_annual_cost_rub Projected annual cost in RUB per function
# TYPE cost_profiler_function_annual_cost_rub gauge
cost_profiler_function_annual_cost_rub{function="main.SlowRouteOptimization"} 8.432e+08
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
| Стоимость Enterprise-лицензии (3 года) | 30 млн ₽ |
| **ROI** | **2000%+** |

---

## ❓ Частые вопросы

**Q: Как профилировать Java без изменений в коде?**  
A: Запустите JVM с флагами `-XX:+PreserveFramePointer`, используйте `--lang java`. При необходимости `--java-signal` ускорит появление карты.

**Q: Какой оверхед на функциях с миллионами вызовов?**  
A: Оверхед <0.5% CPU при семплинге 1/100, <0.1% при 1/1000.

**Q: Можно ли запускать в контейнере?**  
A: Да, используйте готовый Docker‑образ с флагами `--privileged --pid=host`.

**Q: Безопасно ли давать агенту root?**  
A: Мы рекомендуем использовать `CAP_BPF` (менее привилегированно) и статически скомпилированный бинарник. В разделе «Безопасность» есть все детали.

**Q: Поддерживается ли мониторинг нескольких сервисов одновременно?**  
A: Каждый экземпляр профайлера следит за одним процессом. Для нескольких сервисов запустите несколько экземпляров (или используйте sidecar в Kubernetes).

**Q: Как работает автоопределение языка?**  
A: Мы анализируем символы ELF (для Go) и имя процесса с сигнатурами JVM (для Java). Флаг `--lang` остаётся для явного указания или экзотических случаев.

**Q: Можно ли хранить историю больше одного запуска?**  
A: Да, используйте `--clickhouse-host` для долгосрочного хранения или `--remote-write-url` для отправки в Prometheus.

**Q: Как получать алерты при превышении бюджета?**  
A: Используйте `--budget-file` с YAML-конфигом и `--slack-webhook` или `--pagerduty-integration-key`.

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
- [x] **Веб-интерфейс + Prometheus metrics**
- [x] **Docker‑образ и манифесты для K8s**
- [x] **Сравнение с конкурентами** (добавлено)
- [x] **Автоопределение языка (--lang auto)** (добавлено)
- [x] **Сохранение истории в ClickHouse / Prometheus** (добавлено)
- [x] **AI-рекомендации по оптимизации** (добавлено)
- [x] **SLO мониторинг и алерты** (добавлено)
- [ ] Поддержка Python (инструментирование интерпретатора)
- [ ] Интеграция с Grafana (готовые дашборды)
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

---
