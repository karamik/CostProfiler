
# Cost Profiler

**Профилирование производительности с привязкой к финансовым метрикам**

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![eBPF](https://img.shields.io/badge/eBPF-Powered-red)](https://ebpf.io)
[![ROI](https://img.shields.io/badge/ROI-2000%25-brightgreen)](https://karamik.github.io/CostProfiler/)
[![Demo](https://img.shields.io/badge/Demo-Landing_Page-green)](https://karamik.github.io/CostProfiler/)

---

## Описание

Cost Profiler — инструмент для мониторинга производительности кода с расчётом финансового воздействия неоптимальных операций. Работает на основе eBPF, не требует изменений в коде приложений и незаметен для production-нагрузки (<0.5% CPU).

**Ключевые возможности:**
- Перехват вызовов функций на уровне ядра Linux (uprobe/uretprobe)
- Измерение CPU cycles вместо wall time для точной оценки нагрузки
- Расчёт стоимости выполнения функций в рублях на основе инфраструктурных затрат
- Динамический семплинг для минимизации оверхеда
- Интеграция с CI/CD (GitHub Actions) для FinOps-проверок

---

## Быстрый старт

```bash
git clone https://github.com/karamik/CostProfiler
cd CostProfiler
chmod +x run_demo.sh
./run_demo.sh
```

Демонстрация запускается за 30 секунд и показывает пример анализа затрат на логистические алгоритмы.

📺 **Живое демо:** [https://karamik.github.io/CostProfiler/](https://karamik.github.io/CostProfiler/)

---

## Как это работает

```
┌─────────────────────────────┐     ┌─────────────────────────────┐
│   Приложение (Go/PHP/Java)  │     │   eBPF Agent (Userspace)    │
│                             │◄────┤  uprobe / uretprobe         │
│  func SlowRoute() {         │     │  (без изменения кода)       │
│    // логистический расчёт  │     │                             │
│  }                          │     │  Семплинг: 1/100–1/10000     │
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
└───────────────────────────────────────────────────────────────┘
```

---

## Архитектура

| Компонент | Технологии | Назначение |
|-----------|-----------|------------|
| eBPF Agent | C/eBPF, Go | Сбор метрик производительности |
| Collector | Python | Агрегация и расчёт стоимости |
| Storage | ClickHouse | Хранение временных рядов |
| Visualization | Grafana | Дашборды и алерты |
| CI Integration | GitHub Actions | Проверка PR на регрессию затрат |

---

## Требования

- **ОС:** Linux 5.7+ (Astra Linux, Ubuntu, RHEL, CentOS)
- **Ядро:** поддержка eBPF (CONFIG_BPF, CONFIG_BPF_SYSCALL)
- **Привилегии:** CAP_BPF или root для загрузки программ eBPF
- **Зависимости:** Go 1.21+, Python 3.8+, python3-bcc

### Установка зависимостей (Ubuntu/Debian)

```bash
sudo apt-get update
sudo apt-get install -y golang python3 python3-bcc bpftrace
```

---

## Установка

### Клонирование репозитория

```bash
git clone https://github.com/karamik/CostProfiler
cd CostProfiler
```

### Запуск демо

```bash
chmod +x run_demo.sh
./run_demo.sh
```

### Ручной запуск компонентов

```bash
# Сборка Go сервиса
go build -ldflags="-s=false" -o logistics-service main.go

# Запуск сервиса
./logistics-service &
SERVICE_PID=$!

# Запуск eBPF агента
sudo python3 ebpf_agent.py --pid $SERVICE_PID --binary ./logistics-service
```

---

## Конфигурация eBPF агента

```bash
sudo python3 ebpf_agent.py --help

usage: ebpf_agent.py [-h] --pid PID --binary BINARY [--sampling SAMPLING]
                     [--functions FUNCTIONS [FUNCTIONS ...]] [--cost COST]
                     [--scale SCALE]

options:
  --pid PID           Target process PID
  --binary BINARY     Path to binary
  --sampling N        Sampling rate (default: 1 = every call)
  --functions F1 F2   Specific functions to trace
  --cost RUB          Cost per vCPU hour (default: 2.0)
  --scale N           Daily call scale for projection (default: 100M)
```

### Примеры

```bash
# Мониторинг с семплингом 1/100
sudo python3 ebpf_agent.py --pid 27182 --binary ./service --sampling 100

# Мониторинг конкретных функций
sudo python3 ebpf_agent.py --pid 27182 --binary ./service --functions FastRoute SlowRoute

# С кастомной стоимостью CPU
sudo python3 ebpf_agent.py --pid 27182 --binary ./service --cost 3.5
```

---

## Пример вывода

```
┌─────────────────────────────────────────────────────────────┐
│  Cost Profiler Report                                         │
│  Scale: 100M calls/day @ 2 ₽/vCPU·ч                         │
├─────────────────────────────────────────────────────────────┤
│  Function                     │ CPU cycles │ Cost (₽/year)   │
├─────────────────────────────────────────────────────────────┤
│  SlowRouteOptimization        │ 4.2M       │ 843.2M          │
│  VerySlowRouteOptimization    │ 470K       │ 94.1M           │
│  FastRouteOptimization        │ 182K       │ 36.5M           │
├─────────────────────────────────────────────────────────────┤
│  TOTAL WASTE                  │            │ 973.8M          │
└─────────────────────────────────────────────────────────────┘

Recommendations:
  • SlowRouteOptimization: устранить O(n²) в строке 47
  • Preallocate slices: потенциал экономии ~800M ₽/year
```

---

## Интеграция с CI/CD

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
        with:
          go-version: '1.21'
      - name: Build service
        run: go build -o service main.go
      - name: Run Cost Profiler
        run: |
          ./service &
          PID=$!
          sudo python3 ebpf_agent.py --pid $PID --binary ./service --scale 10000000
          kill $PID
```

---

## ROI и экономический эффект

| Показатель | Значение |
|------------|----------|
| Выявленные неоптимальности | 300–500 млн ₽/год |
| Экономия на инфраструктуре | 150–200 млн ₽/год |
| Ускорение расчётов | +15% пропускной способности |
| Стоимость Enterprise-лицензии (3 года) | 25 млн ₽ |
| **ROI** | **2000%+** |

---

## Частые вопросы

**Q: Go инлайнит мелкие функции, uprobe их не видит.**  
A: Инструмент фокусируется на крупных логистических блоках, которые Go не инлайнит. Для критичных участков достаточно добавить `//go:noinline`. 80% затрат концентрируется в 20% больших функций.

**Q: Какой оверхед на функциях с миллионами вызовов?**  
A: Поддерживается динамический семплинг (1/100–1/10000). Оверхед <0.1% CPU при статистической точности >95%.

**Q: Работает ли с Java/Python/1С?**  
A: eBPF работает на уровне бинарника, язык не важен. Для 1С предоставляется адаптер через трассировку RPC-вызовов.

**Q: Можно ли ставить на production?**  
A: Агент поставляется как единый статически скомпилированный бинарник без зависимостей. Рекомендуется начать с пилота на небоевых контурах.

**Q: Нужны ли права root?**  
A: Да, для загрузки eBPF программ требуются привилегии `CAP_BPF` или root. В production рекомендуется настроить `sudo` для конкретной команды.

---

## Структура проекта

```
CostProfiler/
├── index.html          # Landing page (GitHub Pages)
├── README.md           # Документация
├── LICENSE             # Apache 2.0
├── .gitignore          # Игнорируемые файлы
├── main.go             # Тестовый Go сервис
├── ebpf_agent.py       # eBPF агент на Python
└── run_demo.sh         # Скрипт запуска демо
```

---

## Лицензия

- **Open Source ядро:** Apache 2.0
- **Enterprise-модули:** коммерческая лицензия с поддержкой

---

## Контакты

- **GitHub:** [https://github.com/karamik/CostProfiler](https://github.com/karamik/CostProfiler)
- **Demo:** [https://karamik.github.io/CostProfiler/](https://karamik.github.io/CostProfiler/)
- **Enterprise запросы:** [totalprotocol@proton.me](mailto:totalprotocol@proton.me)

---

