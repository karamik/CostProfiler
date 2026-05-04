package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
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

var (
	httpPort = flag.String("http", "", "HTTP порт для health check (например, :8081)")
)

// FastRouteOptimization - быстрая версия (эталон)
func FastRouteOptimization() {
	sum := 0
	for i := 0; i < 1000; i++ {
		sum += i
	}
	_ = sum
}

// SlowRouteOptimization - медленная версия с O(n²) и лишними аллокациями
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
func VerySlowRouteOptimization() {
	for iter := 0; iter < 5; iter++ {
		_ = make([]byte, 10*1024*1024) // 10 MB на итерацию
		time.Sleep(50 * time.Millisecond)
	}
}

func main() {
	flag.Parse()

	fmt.Printf("%s🚀 X5 Group — eBPF Cost-Aware Profiler (Demo Service)%s\n", colorCyan, colorReset)
	fmt.Printf("PID: %d\n", os.Getpid())
	fmt.Printf("HTTP health check: %v\n", *httpPort != "")

	// Запуск HTTP сервера для health check (если указан порт)
	if *httpPort != "" {
		go func() {
			http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("OK"))
			})
			fmt.Printf("Health check listening on http://0.0.0.0%s/health\n", *httpPort)
			if err := http.ListenAndServe(*httpPort, nil); err != nil {
				fmt.Printf("HTTP server error: %v\n", err)
			}
		}()
	}

	// Имитация production-нагрузки
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	counter := 0
	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("Нажмите Ctrl+C для остановки...")
	fmt.Println()

	for {
		select {
		case <-ticker.C:
			counter++
			switch counter % 3 {
			case 0:
				FastRouteOptimization()
			case 1:
				SlowRouteOptimization()
			case 2:
				VerySlowRouteOptimization()
			}

			// Каждые 10 секунд показываем статус (100 тиков * 100мс = 10с)
			if counter%100 == 0 {
				fmt.Printf("%s✅ %d вызовов обработано (агент eBPF может быть активен)%s\n",
					colorGreen, counter, colorReset)
			}
		case <-done:
			fmt.Printf("\n%s🛑 Остановка сервиса...%s\n", colorYellow, colorReset)
			return
		}
	}
}
