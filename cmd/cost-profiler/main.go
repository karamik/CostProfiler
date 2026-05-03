package main

import (
	"bufio"
	"context"
	"debug/dwarf"
	"debug/elf"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/rlimit"
	"github.com/common-nighthawk/go-figure"
	"github.com/fatih/color"
	"github.com/fsnotify/fsnotify"
	"github.com/google/pprof/profile"
	"github.com/olekukonko/tablewriter"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	bpf "github.com/karamik/CostProfiler/ebpf"
)

// Symbol represents an ELF symbol or Java method address
type Symbol struct {
	Name    string
	Addr    uint64
	Inlined bool // true для inlined функций (нельзя привязать uprobe)
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

// ---------- Kubernetes autodiscovery ----------
func getContainerPID(podName, namespace, containerName string) (int, error) {
	var config *rest.Config
	var err error
	// пробуем in-cluster, если не получилось - используем kubeconfig
	config, err = rest.InClusterConfig()
	if err != nil {
		// fallback to kubeconfig
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			kubeconfig = clientcmd.RecommendedHomeFile
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return 0, fmt.Errorf("не удалось создать kubeconfig: %w", err)
		}
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return 0, fmt.Errorf("не удалось создать clientset: %w", err)
	}
	ctx := context.Background()
	pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return 0, fmt.Errorf("pod %s не найден в namespace %s: %w", podName, namespace, err)
	}
	// выбираем контейнер
	if containerName == "" && len(pod.Spec.Containers) > 0 {
		containerName = pod.Spec.Containers[0].Name
	}
	if containerName == "" {
		return 0, fmt.Errorf("не указано имя контейнера и нет контейнеров в pod")
	}
	// ищем containerID
	var containerID string
	for _, status := range pod.Status.ContainerStatuses {
		if status.Name == containerName {
			containerID = status.ContainerID
			break
		}
	}
	if containerID == "" {
		return 0, fmt.Errorf("контейнер %s не найден в pod", containerName)
	}
	// containerID имеет вид "docker://abc123" или "containerd://abc123"
	parts := strings.SplitN(containerID, "//", 2)
	if len(parts) != 2 {
		return 0, fmt.Errorf("неверный формат containerID: %s", containerID)
	}
	fullID := parts[1]
	shortID := fullID
	if len(shortID) > 12 {
		shortID = shortID[:12]
	}
	// ищем PID процесса контейнера на хосте
	// через cgroup v2 или v1
	cgroupBase := "/proc/self/mountinfo"
	data, err := ioutil.ReadFile(cgroupBase)
	if err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.Contains(line, "cgroup") {
				if strings.Contains(line, "/docker/") || strings.Contains(line, "/containerd/") {
					parts = strings.Split(line, " ")
					if len(parts) >= 5 {
						path := parts[4]
						// попробуем найти PID
						entries, _ := ioutil.ReadDir(path)
						for _, entry := range entries {
							if strings.HasPrefix(entry.Name(), shortID) || strings.Contains(entry.Name(), fullID) {
								pidPath := filepath.Join(path, entry.Name(), "cgroup.procs")
								if pidData, err := ioutil.ReadFile(pidPath); err == nil {
									pidStr := strings.TrimSpace(string(pidData))
									pid, err := strconv.Atoi(pidStr)
									if err == nil && pid > 0 {
										return pid, nil
									}
								}
							}
						}
					}
				}
			}
		}
	}
	return 0, fmt.Errorf("не удалось найти PID контейнера %s (%s)", containerName, fullID)
}

// ---------- DWARF и поддержка inlined функций ----------
func getAllSymbolsWithDWARF(binaryPath string, filter string) ([]Symbol, error) {
	f, err := elf.Open(binaryPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// сначала стандартные символы из таблицы
	var symbols []Symbol
	symTab, err := f.Symbols()
	if err == nil {
		for _, sym := range symTab {
			if elf.ST_TYPE(sym.Info) == elf.STT_FUNC && sym.Section != elf.SHN_UNDEF {
				name := sym.Name
				if len(name) > 0 && name[0] != '.' && name != "runtime.main" {
					if filter == "" || strings.HasPrefix(name, filter) {
						symbols = append(symbols, Symbol{Name: name, Addr: sym.Value, Inlined: false})
					}
				}
			}
		}
	}

	// теперь пробуем извлечь inlined функции через DWARF
	d, err := f.DWARF()
	if err != nil {
		// нет DWARF – возвращаем что есть
		return symbols, nil
	}
	reader := d.Reader()
	// ищем все подпрограммы
	for {
		entry, err := reader.Next()
		if err != nil {
			break
		}
		if entry == nil {
			break
		}
		if entry.Tag == dwarf.TagSubprogram {
			nameAttr := entry.Attr(dwarf.AttrName)
			if nameAttr == nil {
				continue
			}
			name := nameAttr.(string)
			if filter != "" && !strings.HasPrefix(name, filter) {
				continue
			}
			// адрес функции
			lowpcAttr := entry.Attr(dwarf.AttrLowpc)
			if lowpcAttr == nil {
				continue
			}
			addr, ok := lowpcAttr.(uint64)
			if !ok {
				continue
			}
			// проверяем, есть ли уже такой символ
			found := false
			for _, s := range symbols {
				if s.Addr == addr {
					found = true
					break
				}
			}
			if !found {
				// возможно, это inlined функция? У неё может быть атрибут inline
				inlineAttr := entry.Attr(dwarf.AttrInline)
				isInlined := inlineAttr != nil
				symbols = append(symbols, Symbol{Name: name, Addr: addr, Inlined: isInlined})
			}
		}
	}
	return symbols, nil
}

// parsePerfMap читает /tmp/perf-<pid>.map и возвращает map[адрес] → имя метода
func parsePerfMap(pid int) (map[uint64]string, error) {
	perfMapPath := fmt.Sprintf("/tmp/perf-%d.map", pid)
	file, err := os.Open(perfMapPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	result := make(map[uint64]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		// Формат: start length name
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}
		addr, err := strconv.ParseUint(parts[0], 16, 64)
		if err != nil {
			continue
		}
		name := parts[2]
		result[addr] = name
	}
	return result, scanner.Err()
}

// watchPerfMap следит за изменениями файла perf-map и обновляет карту символов и uprobe
func watchPerfMap(pid int, ex *link.Executable, objs *bpf.Objects, symbolMap *map[uint64]Symbol, uprobes *[]link.Link, done chan bool) {
	perfMapPath := fmt.Sprintf("/tmp/perf-%d.map", pid)
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("⚠️ Не удалось создать watcher для perf-map: %v", err)
		return
	}
	defer watcher.Close()

	err = watcher.Add(filepath.Dir(perfMapPath))
	if err != nil {
		log.Printf("⚠️ Не удалось добавить директорию в watcher: %v", err)
		return
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Name == perfMapPath && event.Op&fsnotify.Write == fsnotify.Write {
				newMap, err := parsePerfMap(pid)
				if err != nil {
					continue
				}
				for addr, name := range newMap {
					if _, exists := (*symbolMap)[addr]; !exists {
						up, err := ex.Uprobe("", objs.CostStart, &link.UprobeOptions{Address: addr})
						if err != nil {
							yellow.Printf("⚠️ Не удалось привязать Java метод %s (0x%x): %v\n", name, addr, err)
							continue
						}
						ret, err := ex.Uretprobe("", objs.CostEnd, &link.UprobeOptions{Address: addr})
						if err != nil {
							yellow.Printf("⚠️ Не удалось привязать uretprobe для %s: %v\n", name, err)
							up.Close()
							continue
						}
						*uprobes = append(*uprobes, up, ret)
						(*symbolMap)[addr] = Symbol{Name: name, Addr: addr, Inlined: false}
						green.Printf("  🔗 (JIT) Привязан: %s @ 0x%x\n", name, addr)
					}
				}
			}
		case <-done:
			return
		}
	}
}

// findSymbolsForGo находит функции в Go-бинарнике через ELF + DWARF (inlined)
func findSymbolsForGo(binaryPath string, filter string) ([]Symbol, error) {
	return getAllSymbolsWithDWARF(binaryPath, filter)
}

// findSymbolsForJava получает функции из perf-map (JIT-компилированные методы)
func findSymbolsForJava(pid int, filter string) ([]Symbol, error) {
	addrMap, err := parsePerfMap(pid)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения perf-map: %w", err)
	}
	var symbols []Symbol
	for addr, name := range addrMap {
		if filter == "" || strings.Contains(name, filter) {
			symbols = append(symbols, Symbol{Name: name, Addr: addr, Inlined: false})
		}
	}
	return symbols, nil
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
		if m.CallCount == 0 {
			continue
		}
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
		fmt.Println("  • Добавьте //go:noinline для точного профилирования (Go)")
		fmt.Println("  • Для Java используйте -XX:+PreserveFramePointer")
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
		if m.CallCount == 0 {
			continue
		}
		avgMs := float64(m.TotalNS) / float64(m.CallCount) / 1_000_000
		fmt.Printf("%s,%d,%.2f,%.8f,%.0f\n", m.Name, m.CallCount, avgMs, m.CostPerCall, m.AnnualCost)
	}
}

// exportPGOProfile создаёт профиль в формате pprof для использования с go build -pgo
func exportPGOProfile(metrics []FunctionMetric, outputPath string, costPerVCPUHour float64) error {
	p := &profile.Profile{
		SampleType: []*profile.ValueType{
			{Type: "cpu", Unit: "nanoseconds"},
		},
		Period: 1e9,
	}

	for _, m := range metrics {
		if m.CallCount == 0 || m.TotalNS == 0 {
			continue
		}
		avgNS := float64(m.TotalNS) / float64(m.CallCount)
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
		pid           int
		podName       string
		namespace     string
		containerName string
		binaryPath    string
		cost          float64
		scale         int64
		sampling      int
		filter        string
		format        string
		duration      int
		exportPGO     string
		lang          string
	)

	flag.IntVar(&pid, "pid", 0, "PID целевого процесса")
	flag.StringVar(&podName, "pod", "", "Имя Pod (вместо --pid) для автопоиска PID контейнера")
	flag.StringVar(&namespace, "namespace", "default", "Namespace Pod (по умолчанию default)")
	flag.StringVar(&containerName, "container", "", "Имя контейнера внутри Pod (если не указан, берётся первый)")
	flag.StringVar(&binaryPath, "binary", "", "Путь к бинарному файлу (для Go). Для Java необязателен")
	flag.Float64Var(&cost, "cost", 2.0, "Стоимость vCPU часа (₽)")
	flag.Int64Var(&scale, "scale", 100_000_000, "Количество вызовов в сутки")
	flag.IntVar(&sampling, "sampling", 1, "Семплинг: 1 = каждый вызов")
	flag.StringVar(&filter, "filter", "", "Фильтр функций (префикс для Go, подстрока для Java)")
	flag.StringVar(&format, "format", "table", "Формат вывода: table, json, csv")
	flag.IntVar(&duration, "duration", 30, "Длительность сбора метрик (секунд)")
	flag.StringVar(&exportPGO, "export-pgo", "", "Экспортировать профиль в формате pprof для go build -pgo")
	flag.StringVar(&lang, "lang", "go", "Язык приложения: go, java")
	flag.Parse()

	printBanner()

	// autodiscovery: если указан pod, получаем PID
	if podName != "" {
		p, err := getContainerPID(podName, namespace, containerName)
		if err != nil {
			red.Printf("❌ Ошибка autodiscovery pod: %v\n", err)
			os.Exit(1)
		}
		pid = p
		green.Printf("✅ Найден PID контейнера: %d\n", pid)
	}

	if pid == 0 {
		red.Println("❌ Ошибка: необходимо указать --pid или --pod")
		flag.Usage()
		os.Exit(1)
	}

	if lang == "go" && binaryPath == "" {
		red.Println("❌ Ошибка: для Go необходимо указать --binary")
		flag.Usage()
		os.Exit(1)
	}

	// Проверка существования процесса
	proc, err := os.FindProcess(pid)
	if err != nil || proc.Signal(syscall.Signal(0)) != nil {
		red.Printf("❌ Процесс с PID %d не найден или не отвечает\n", pid)
		os.Exit(1)
	}

	green.Printf("✅ Целевой процесс: PID %d, язык: %s\n", pid, lang)

	// Снятие ограничений на память для eBPF
	if err := rlimit.RemoveMemlock(); err != nil {
		log.Fatalf("❌ Ошибка снятия ограничений: %v", err)
	}

	var symbols []Symbol
	switch lang {
	case "go":
		symbols, err = findSymbolsForGo(binaryPath, filter)
		if err != nil {
			red.Printf("❌ Ошибка парсинга символов: %v\n", err)
			os.Exit(1)
		}
		cyan.Printf("🔍 Обнаружено Go-функций (включая inlined, если есть): %d\n", len(symbols))
	case "java":
		symbols, err = findSymbolsForJava(pid, filter)
		if err != nil {
			red.Printf("❌ Ошибка чтения Java perf-map: %v\n", err)
			yellow.Println("   Убедитесь, что JVM запущена с флагами:")
			fmt.Println("     -XX:+UnlockDiagnosticVMOptions -XX:+DebugNonSafepoints -XX:+PreserveFramePointer")
			os.Exit(1)
		}
		cyan.Printf("🔍 Обнаружено Java JIT-методов: %d\n", len(symbols))
	}

	if len(symbols) == 0 {
		yellow.Println("⚠️ Функции не найдены. Для Go: соберите с -ldflags=\"-s=false\" и включите отладочную информацию. Для Java: проверьте наличие /tmp/perf-<pid>.map")
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

	// Открытие бинарника для uprobe (для Go — бинарник, для Java — /proc/pid/exe)
	var ex *link.Executable
	if lang == "go" {
		ex, err = link.OpenExecutable(binaryPath)
	} else {
		exePath := fmt.Sprintf("/proc/%d/exe", pid)
		ex, err = link.OpenExecutable(exePath)
	}
	if err != nil {
		log.Fatalf("❌ Ошибка открытия исполняемого файла: %v", err)
	}

	// Установка uprobe только для не-inlined функций
	var uprobes []link.Link
	symbolMap := make(map[uint64]Symbol)
	for _, sym := range symbols {
		if sym.Inlined {
			yellow.Printf("⚠️ Inlined функция %s — пропускаем (невозможно привязать uprobe)\n", sym.Name)
			continue
		}
		if len(sym.Name) < 3 {
			continue
		}
		var up, ret link.Link
		if lang == "go" {
			up, err = ex.Uprobe(sym.Name, objs.CostStart, nil)
		} else {
			up, err = ex.Uprobe("", objs.CostStart, &link.UprobeOptions{Address: sym.Addr})
		}
		if err != nil {
			yellow.Printf("⚠️ Не удалось привязаться к %s (0x%x): %v\n", sym.Name, sym.Addr, err)
			continue
		}
		uprobes = append(uprobes, up)

		if lang == "go" {
			ret, err = ex.Uretprobe(sym.Name, objs.CostEnd, nil)
		} else {
			ret, err = ex.Uretprobe("", objs.CostEnd, &link.UprobeOptions{Address: sym.Addr})
		}
		if err != nil {
			yellow.Printf("⚠️ Не удалось привязать uretprobe к %s: %v\n", sym.Name, err)
			up.Close()
			continue
		}
		uprobes = append(uprobes, ret)

		symbolMap[sym.Addr] = sym
		green.Printf("  🔗 Привязан: %s (0x%x)\n", sym.Name, sym.Addr)
	}
	defer func() {
		for _, up := range uprobes {
			up.Close()
		}
	}()

	// Для Java запускаем watcher за perf-map
	var doneWatcher chan bool
	if lang == "java" {
		doneWatcher = make(chan bool)
		go watchPerfMap(pid, ex, &objs, &symbolMap, &uprobes, doneWatcher)
	}

	yellow.Printf("\n📊 Сбор метрик в течение %d секунд... (Ctrl+C для досрочного останова)\n", duration)
	fmt.Println()

	// Сбор метрик в фоне
	metrics := make(map[uint64]FunctionMetric)
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
	if doneWatcher != nil {
		close(doneWatcher)
	}

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
		if sym, ok := symbolMap[key]; ok {
			name = sym.Name
		}
		if m.CallCount == 0 {
			continue
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
