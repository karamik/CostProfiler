
# Cost Profiler

**Профилирование производительности с привязкой к финансовым метрикам**

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![eBPF](https://img.shields.io/badge/eBPF-Powered-red)](https://ebpf.io)
[![ROI](https://img.shields.io/badge/ROI-2000%25-brightgreen)](https://github.com)

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
git clone https://github.com/x5-labs/cost-profiler
cd cost-profiler
./run_demo.sh
```

Демонстрация запускается за 30 секунд и показывает пример анализа затрат на логистические алгоритмы.

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

- **ОС:** Linux 5.7+ (Astra Linux, Ubuntu, RHEL)
- **Ядро:** поддержка eBPF (CONFIG_BPF, CONFIG_BPF_SYSCALL)
- **Привилегии:** CAP_BPF или root для загрузки программ eBPF
- **Зависимости:** libc, libbpf (для сборки из исходников)

---

## Установка

### Статический бинарник (рекомендуется)

```bash
wget https://github.com/x5-labs/cost-profiler/releases/latest/download/cost-profiler
chmod +x cost-profiler
sudo ./cost-profiler --pid $(pgrep your-service)
```

### Сборка из исходников

```bash
git clone https://github.com/x5-labs/cost-profiler
cd cost-profiler
make build
sudo ./cost-profiler --config config.yaml
```

---

## Конфигурация

```yaml
# config.yaml
agent:
  sampling_rate: 100          # Семплинг: каждый N-й вызов
  target_pids: []             # PID процессов для мониторинга
  functions:                  # Список функций для трассировки
    - name: "SlowRouteOptimization"
      binary: "/opt/app/service"
      
finops:
  cost_per_vcpu_hour: 2.0     # ₽/vCPU·ч
  scale_factor: 100000000     # Количество вызовов в сутки
  
storage:
  clickhouse:
    host: "localhost:8123"
    database: "cost_profiler"
```

---

## Использование

### Базовый запуск

```bash
# Мониторинг конкретного процесса
sudo cost-profiler --pid 27182 --functions SlowRoute,FastRoute

# Автообнаружение функций в бинарнике
sudo cost-profiler --binary /opt/app/service --auto-discover
```

### Пример вывода

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
      - name: Run Cost Profiler
        uses: x5-labs/cost-profiler-action@v1
        with:
          binary: ./build/service
          threshold: 1000000  # Максимальная допустимая стоимость (₽/year)
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

---

## Лицензия

- **Open Source ядро:** Apache 2.0
- **Enterprise-модули:** коммерческая лицензия с поддержкой

---

**Контакты:** totalprotocol@proton.me
```
