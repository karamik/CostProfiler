package main

import (
	"fmt"
	"math/rand"
	"runtime"
	"time"
)

// ANSI color codes for terminal output
const (
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorReset  = "\033[0m"
)

// FastRouteOptimization - быстрая версия (эталон)
//export FastRouteOptimization
func FastRouteOptimization() {
	sum := 0
	for i := 0; i < 1000; i++ {
		sum += i
	}
	_ = sum
}

// SlowRouteOptimization - медленная версия с O(n²) и лишними аллокациями
//export SlowRouteOptimization
func SlowRouteOptimization() {
	var data []int
	for i := 0; i < 10000; i++ {
		data = append(data, rand.Intn(100))
		// Вложенный цикл - O(n²)
		for j := 0; j < 100; j++ {
			_ = data[j%len(data)] + rand.Intn(10)
		}
	}
}

// VerySlowRouteOptimization - очень медленная с большими аллокациями
//export VerySlowRouteOptimization
func VerySlowRouteOptimization() {
	for iter := 0; iter < 5; iter++ {
		_ = make([]byte, 10*1024*1024) // 10 MB на итерацию
		time.Sleep(50 * time.Millisecond)
	}
}

func main() {
	fmt.Printf("%s🚀 X5 Group — eBPF Cost-Aware Profiler%s\n", colorCyan, colorReset)
	fmt.Printf("PID: %d\n\n", runtime.GOMAXPROCS(0))

	// Имитация production-нагрузки
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	counter := 0
	for range ticker.C {
		counter++
		switch counter % 3 {
		case 0:
			FastRouteOptimization()
		case 1:
			SlowRouteOptimization()
		case 2:
			VerySlowRouteOptimization()
		}

		// Каждые 10 секунд показываем статус
		if counter%100 == 0 {
			fmt.Printf("%s✅ %d вызовов обработано (агент eBPF активен)%s\n",
				colorGreen, counter, colorReset)
		}
	}
}
