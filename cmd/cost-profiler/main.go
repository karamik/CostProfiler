package main

import (
	"debug/elf"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/rlimit"
	"github.com/common-nighthawk/go-figure"
	"github.com/fatih/color"
	"github.com/google/pprof/profile"
	"github.com/olekukonko/tablewriter"

	bpf "github.com/karamik/CostProfiler/ebpf"
)

// Symbol represents an ELF symbol
type Symbol struct {
	Name string
	Addr uint64
}

// FunctionMetric holds collected data for a function
type FunctionMetric struct {
	Name        string
	CallCount   uint64
	TotalNS     uint64
	CostPerCall float64
	AnnualCost  float64
}

// CostConfig holds pricing configuration
type CostConfig struct {
	CostPerVCPUHour float64
	DailyCalls      int64
	SamplingRate    int
	OutputFormat    string
}

var (
	cyan   = color.New(color.FgCyan, color.Bold)
	green  = color.New(color.FgGreen, color.Bold)
	red    = color.New(color.FgRed, color.Bold)
	yellow = color.New(color.FgYellow)
)

func printBanner() {
	banner := figure.NewFigure("COST PROFILER", "slant", true)
	banner.Print()
	fmt.Println()
	cyan.Println("💰 FinOps для high-load систем | eBPF Enterprise Edition")
	fmt.Println()
}

func findSymbols(binaryPath string, filter string) ([]Symbol, error) {
	f, err := elf.Open(binaryPath)
	if err != nil {
		return nil, fmt.Errorf("ошибка открытия ELF: %w", err)
	}
	defer f.Close()

	symbols, err := f.Symbols()
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения символов: %w", err)
	}

	var found []Symbol
	for _, sym := range symbols {
		if elf.ST_TYPE(sym.Info) == elf.STT_FUNC && sym.Section != elf.SHN_UNDEF {
			name := sym.Name
			if len(name) > 0 && name[0] != '.' && name != "runtime.main" {
				if filter == "" || (len(name) >= len(filter) && name[:len(filter)] == filter) {
					found = append(found, Symbol{
						Name: name,
						Addr: sym.Value,
					})
				}
			}
		}
	}
	return found, nil
}

func formatMoney(amount float64) string {
	if amount >= 1_000_000_000 {
		return fmt.Sprintf("%.2f млрд ₽", amount/1_000_000_000)
	} else if amount >= 1_000_000 {
		return fmt.Sprintf("%.2f млн ₽", amount/1_000_000)
	}
	return fmt.Sprintf("%.0f ₽", amount)
}

func printTable(metrics []FunctionMetric, config CostConfig) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Функция", "Вызовов", "CPU (мс)", "₽/вызов", "Затраты/год"})
	table.SetBorder(false)
	table.SetHeaderColor(
		tablewriter.Colors{tablewriter.FgCyanColor},
		tablewriter.Colors{tablewriter.FgCyanColor},
		tablewriter.Colors{tablewriter.FgCyanColor},
		tablewriter.Colors{tablewriter.FgCyanColor},
		tablewriter.Colors{tablewriter.FgCyanColor},
	)

	var totalAnnual float64
	for _, m := range metrics {
		avgMs := float64(m.TotalNS) / float64(m.CallCount) / 1_000_000
		totalAnnual += m.AnnualCost

		costColor := tablewriter.Colors{tablewriter.FgGreenColor}
		if m.AnnualCost > 50_000_000 {
			costColor = tablewriter.Colors{tablewriter.FgRedColor}
		} else if m.AnnualCost > 10_000_000 {
			costColor = tablewriter.Colors{tablewriter.FgYellowColor}
		}

		table.Rich([]string{
			m.Name,
			fmt.Sprintf("%d", m.CallCount),
			fmt.Sprintf("%.2f", avgMs),
			fmt.Sprintf("%.8f", m.CostPerCall),
			formatMoney(m.AnnualCost),
		}, []tablewriter.Colors{
			{},
			{},
			{},
			{},
			costColor,
		})
	}

	table.SetFooter([]string{"", "", "", "ИТОГО:", formatMoney(totalAnnual)})
	table.SetFooterColor(
		tablewriter.Colors{},
		tablewriter.Colors{},
		tablewriter.Colors{},
		tablewriter.Colors{tablewriter.FgYellowColor},
		tablewriter.Colors{tablewriter.FgRedColor, tablewriter.Bold},
	)
	table.Render()

	if totalAnnual > 100_000_000 {
		fmt.Println()
		yellow.Println("💡 Рекомендации по оптимизации:")
		for _, m := range metrics {
			if m.AnnualCost > 50_000_000 {
				fmt.Printf("  • %s: пересмотреть алгоритм (экономия до %s/год)\n", m.Name, formatMoney(m.AnnualCost*0.7))
			}
		}
		fmt.Println("  • Добавьте //go:noinline для точного профилирования")
		fmt.Println("  • Используйте preallocation для слайсов")
	}
}

func printJSON(metrics []FunctionMetric, config CostConfig) {
	output := map[string]interface{}{
		"timestamp":           time.Now().Format(time.RFC3339),
		"cost_per_vcpu_hour":  config.CostPerVCPUHour,
		"daily_calls":         config.DailyCalls,
		"sampling_rate":       config.SamplingRate,
		"functions":           metrics,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(output)
}

func printCSV(metrics []FunctionMetric, config CostConfig) {
	fmt.Println("Function,Calls,AvgTimeMs,CostPerCall,AnnualCostRub")
	for _, m := range metrics {
		avgMs := float64(m.TotalNS) / float64(m.CallCount) / 1_000_000
		fmt.Printf("%s,%d,%.2f,%.8f,%.0f\n", m.Name, m.CallCount, avgMs, m.CostPerCall, m.AnnualCost)
	}
}

// exportPGOProfile создаёт профиль в формате pprof для использования с go build -pgo
func exportPGOProfile(metrics []FunctionMetric, outputPath string, costPerVCPUHour float64) error {
	// Создаём профиль CPU в формате pprof
	p := &profile.Profile{
		SampleType: []*profile.ValueType{
			{Type: "cpu", Unit: "nanoseconds"},
		},
		Period: 1e9, // 1 second
	}

	// Добавляем семплы для каждой функции
	for _, m := range metrics {
		if m.CallCount == 0 || m.TotalNS == 0 {
			continue
		}
		avgNS := float64(m.TotalNS) / float64(m.CallCount)
		// В pprof семплы представляют собой накопленное время в наносекундах
		p.Sample = append(p.Sample, &profile.Sample{
			Value: []int64{int64(avgNS)},
			Location: []*profile.Location{
				{
					ID: uint64(len(p.Location) + 1),
					Line: []profile.Line{
						{
							Function: &profile.Function{
								ID:   uint64(len(p.Function) + 1),
								Name: m.Name,
							},
						},
					},
				},
			},
		})
		// Добавляем функцию в список
		p.Function = append(p.Function, &profile.Function{
			ID:   uint64(len(p.Function) + 1),
			Name: m.Name,
		})
		p.Location = append(p.Location, &profile.Location{
			ID: uint64(len(p.Location) + 1),
			Line: []profile.Line{
				{
					Function: p.Function[len(p.Function)-1],
				},
			},
		})
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := p.Write(f); err != nil {
		return err
	}
	fmt.Println()
	green.Printf("✅ PGO профиль экспортирован в %s\n", outputPath)
	yellow.Println("   Используйте для оптимизации: go build -pgo=" + outputPath + " -o optimized-binary")
	return nil
}

func main() {
	var (
		pid         int
		binaryPath  string
		cost        float64
		scale       int64
		sampling    int
		filter      string
		format      string
		duration    int
		exportPGO   string
	)

	flag.IntVar(&pid, "pid", 0, "PID целевого процесса")
	flag.StringVar(&binaryPath, "binary", "", "Путь к бинарному файлу")
	flag.Float64Var(&cost, "cost", 2.0, "Стоимость vCPU часа (₽)")
	flag.Int64Var(&scale, "scale", 100_000_000, "Количество вызовов в сутки")
	flag.IntVar(&sampling, "sampling", 1, "Семплинг: 1 = каждый вызов")
	flag.StringVar(&filter, "filter", "", "Фильтр функций (префикс)")
	flag.StringVar(&format, "format", "table", "Формат вывода: table, json, csv")
	flag.IntVar(&duration, "duration", 30, "Длительность сбора метрик (секунд)")
	flag.StringVar(&exportPGO, "export-pgo", "", "Экспортировать профиль в формате pprof для go build -pgo")
	flag.Parse()

	printBanner()

	if pid == 0 || binaryPath == "" {
		red.Println("❌ Ошибка: необходимо указать --pid и --binary")
		flag.Usage()
		os.Exit(1)
	}

	// Проверка существования процесса
	proc, err := os.FindProcess(pid)
	if err != nil || proc.Signal(syscall.Signal(0)) != nil {
		red.Printf("❌ Процесс с PID %d не найден или не отвечает\n", pid)
		os.Exit(1)
	}

	green.Printf("✅ Целевой процесс: PID %d\n", pid)

	// Снятие ограничений на память для eBPF
	if err := rlimit.RemoveMemlock(); err != nil {
		log.Fatalf("❌ Ошибка снятия ограничений: %v", err)
	}

	// Поиск функций в бинарнике
	symbols, err := findSymbols(binaryPath, filter)
	if err != nil {
		red.Printf("❌ Ошибка парсинга ELF: %v\n", err)
		os.Exit(1)
	}
	cyan.Printf("🔍 Обнаружено функций: %d\n", len(symbols))

	if len(symbols) == 0 {
		yellow.Println("⚠️ Функции не найдены. Попробуйте убрать фильтр или собрать бинарник с символами:")
		fmt.Println("   go build -ldflags=\"-s=false\" -o service main.go")
		os.Exit(1)
	}

	// Загрузка eBPF объектов
	objs := bpf.Objects{}
	if err := bpf.LoadObjects(&objs, nil); err != nil {
		log.Fatalf("❌ Ошибка загрузки eBPF: %v", err)
	}
	defer objs.Close()

	// Установка целевого PID в eBPF мап
	pidKey := uint32(0)
	pidVal := uint32(pid)
	if err := objs.TargetPid.Put(&pidKey, &pidVal); err != nil {
		log.Fatalf("❌ Ошибка установки PID в eBPF: %v", err)
	}

	// Открытие бинарника для uprobe
	ex, err := link.OpenExecutable(binaryPath)
	if err != nil {
		log.Fatalf("❌ Ошибка открытия бинарника: %v", err)
	}

	// Установка uprobe для каждой функции
	var uprobes []link.Link
	for _, sym := range symbols {
		if len(sym.Name) < 5 {
			continue
		}
		up, err := ex.Uprobe(sym.Name, objs.CostStart, nil)
		if err != nil {
			yellow.Printf("⚠️ Не удалось привязаться к %s: %v\n", sym.Name, err)
			continue
		}
		uprobes = append(uprobes, up)

		ret, err := ex.Uretprobe(sym.Name, objs.CostEnd, nil)
		if err != nil {
			yellow.Printf("⚠️ Не удалось привязать uretprobe к %s: %v\n", sym.Name, err)
			continue
		}
		uprobes = append(uprobes, ret)

		green.Printf("  🔗 Привязан: %s\n", sym.Name)
	}
	defer func() {
		for _, up := range uprobes {
			up.Close()
		}
	}()

	yellow.Printf("\n📊 Сбор метрик в течение %d секунд... (Ctrl+C для досрочного останова)\n", duration)
	fmt.Println()

	// Сбор метрик в фоне
	metrics := make(map[uint64]FunctionMetric)
	done := make(chan bool)
	ticker := time.NewTicker(500 * time.Millisecond)

	go func() {
		for range ticker.C {
			var key, calls, cycles uint64
			callIter := objs.FunctionCalls.Iterate()
			for callIter.Next(&key, &calls) {
				if err := objs.FunctionCycles.Lookup(&key, &cycles); err != nil {
					continue
				}
				if calls > 0 && cycles > 0 {
					if m, exists := metrics[key]; exists {
						m.CallCount = calls
						m.TotalNS = cycles
						metrics[key] = m
					} else {
						metrics[key] = FunctionMetric{
							CallCount: calls,
							TotalNS:   cycles,
						}
					}
				}
			}
		}
	}()

	// Ожидание завершения или сигнала
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-time.After(time.Duration(duration) * time.Second):
		fmt.Println()
		yellow.Println("⏰ Время сбора истекло, формируем отчёт...")
	case <-sigChan:
		fmt.Println()
		yellow.Println("🛑 Остановка по сигналу, формируем отчёт...")
	}
	ticker.Stop()

	// Подготовка данных для отчёта
	config := CostConfig{
		CostPerVCPUHour: cost,
		DailyCalls:      scale,
		SamplingRate:    sampling,
		OutputFormat:    format,
	}

	var metricList []FunctionMetric
	for key, m := range metrics {
		name := fmt.Sprintf("func_0x%x", key)
		for _, sym := range symbols {
			if sym.Addr == key {
				name = sym.Name
				break
			}
		}
		avgNS := float64(m.TotalNS) / float64(m.CallCount)
		costPerCall := (avgNS / 1e9 / 3600) * config.CostPerVCPUHour
		annualCost := costPerCall * float64(config.DailyCalls) * 365

		metricList = append(metricList, FunctionMetric{
			Name:        name,
			CallCount:   m.CallCount,
			TotalNS:     m.TotalNS,
			CostPerCall: costPerCall,
			AnnualCost:  annualCost,
		})
	}

	// Сортировка по затратам
	for i := 0; i < len(metricList)-1; i++ {
		for j := i + 1; j < len(metricList); j++ {
			if metricList[i].AnnualCost < metricList[j].AnnualCost {
				metricList[i], metricList[j] = metricList[j], metricList[i]
			}
		}
	}

	// Вывод результатов
	fmt.Println()
	switch format {
	case "json":
		printJSON(metricList, config)
	case "csv":
		printCSV(metricList, config)
	default:
		printTable(metricList, config)
	}

	// Экспорт PGO профиля, если указан флаг
	if exportPGO != "" {
		if err := exportPGOProfile(metricList, exportPGO, cost); err != nil {
			log.Printf("⚠️ Ошибка экспорта PGO профиля: %v", err)
		}
	}
}
