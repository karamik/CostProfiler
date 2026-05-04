//go:build ignore

#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <linux/perf_event.h>

#define SAMPLING_MAX 10000

// Хранилище времени входа (не используется, если используем CPU cycles)
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 10240);
    __type(key, u64);   // pid_tgid
    __type(value, u64); // timestamp (наносекунды)
} start_time SEC(".maps");

// Счётчики вызовов
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 10240);
    __type(key, u64);
    __type(value, u64);
} function_calls SEC(".maps");

// Суммарное время CPU (в наносекундах) – заполняется через perf_event_read
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 10240);
    __type(key, u64);
    __type(value, u64);
} function_cycles SEC(".maps");

// Мапа для хранения файловых дескрипторов perf_event для каждого pid_tgid
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 10240);
    __type(key, u64);
    __type(value, int);
} perf_fds SEC(".maps");

// PID целевого процесса (фильтр)
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 1);
    __type(key, u32);
    __type(value, u32);
} target_pid SEC(".maps");

// Параметр семплинга (1..SAMPLING_MAX)
struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(max_entries, 1);
    __type(key, u32);
    __type(value, u32);
} sampling_rate SEC(".maps");

SEC("uprobe/cost_start")
int trace_func_entry(struct pt_regs *ctx) {
    u64 pid_tgid = bpf_get_current_pid_tgid();
    u32 pid = pid_tgid >> 32;

    // Фильтр по PID
    u32 *target = bpf_map_lookup_elem(&target_pid, &pid);
    if (!target) return 0;

    // Семплинг
    u32 zero = 0;
    u32 *rate = bpf_map_lookup_elem(&sampling_rate, &zero);
    if (rate && *rate > 1) {
        u32 rnd = bpf_get_prandom_u32();
        if (rnd % (*rate) != 0) return 0;
    }

    // Запоминаем время (wall time – для fallback, если perf_event не доступен)
    u64 ts = bpf_ktime_get_ns();
    bpf_map_update_elem(&start_time, &pid_tgid, &ts, BPF_ANY);
    return 0;
}

SEC("uretprobe/cost_end")
int trace_func_return(struct pt_regs *ctx) {
    u64 pid_tgid = bpf_get_current_pid_tgid();
    u32 pid = pid_tgid >> 32;

    u32 *target = bpf_map_lookup_elem(&target_pid, &pid);
    if (!target) return 0;

    // Семплинг (аналогично entry)
    u32 zero = 0;
    u32 *rate = bpf_map_lookup_elem(&sampling_rate, &zero);
    if (rate && *rate > 1) {
        u32 rnd = bpf_get_prandom_u32();
        if (rnd % (*rate) != 0) return 0;
    }

    // Попытка прочитать количество CPU cycles через perf_event
    int *fd = bpf_map_lookup_elem(&perf_fds, &pid_tgid);
    u64 cycles = 0;
    if (fd) {
        // bpf_perf_event_read возвращает 0 при успехе, значение записывается в cycles
        if (bpf_perf_event_read((void *)fd, 0, &cycles, sizeof(cycles)) == 0) {
            // cycles – это 64-битное значение счётчика
        } else {
            // fallback: используем разницу во времени
            u64 *tsp = bpf_map_lookup_elem(&start_time, &pid_tgid);
            if (tsp) {
                cycles = bpf_ktime_get_ns() - *tsp;
            }
        }
    } else {
        // fallback: wall time
        u64 *tsp = bpf_map_lookup_elem(&start_time, &pid_tgid);
        if (tsp) {
            cycles = bpf_ktime_get_ns() - *tsp;
        }
    }

    if (cycles > 0) {
        // Увеличиваем счётчик вызовов
        u64 *calls = bpf_map_lookup_elem(&function_calls, &pid_tgid);
        if (calls) {
            __sync_fetch_and_add(calls, 1);
        } else {
            u64 one = 1;
            bpf_map_update_elem(&function_calls, &pid_tgid, &one, BPF_ANY);
        }

        // Добавляем циклы
        u64 *total_cycles = bpf_map_lookup_elem(&function_cycles, &pid_tgid);
        if (total_cycles) {
            __sync_fetch_and_add(total_cycles, cycles);
        } else {
            bpf_map_update_elem(&function_cycles, &pid_tgid, &cycles, BPF_ANY);
        }
    }

    bpf_map_delete_elem(&start_time, &pid_tgid);
    return 0;
}

char LICENSE[] SEC("license") = "GPL";
