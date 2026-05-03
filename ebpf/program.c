//go:build ignore

#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>

// Хранилище времени входа (ключ: pid_tgid)
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 10240);
    __type(key, u64);
    __type(value, u64);
} start_time SEC(".maps");

// Счётчики вызовов функций
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 10240);
    __type(key, u64);
    __type(value, u64);
} function_calls SEC(".maps");

// Суммарное время CPU (в наносекундах)
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 10240);
    __type(key, u64);
    __type(value, u64);
} function_cycles SEC(".maps");

// Привязка к PID процесса (фильтрация)
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 1);
    __type(key, u32);
    __type(value, u32);
} target_pid SEC(".maps");

SEC("uprobe/cost_start")
int trace_func_entry(struct pt_regs *ctx) {
    u64 pid_tgid = bpf_get_current_pid_tgid();
    u32 pid = pid_tgid >> 32;
    
    // Проверяем, тот ли это процесс
    u32 *target = bpf_map_lookup_elem(&target_pid, &pid);
    if (!target) {
        return 0;
    }
    
    u64 ts = bpf_ktime_get_ns();
    bpf_map_update_elem(&start_time, &pid_tgid, &ts, BPF_ANY);
    return 0;
}

SEC("uretprobe/cost_end")
int trace_func_return(struct pt_regs *ctx) {
    u64 pid_tgid = bpf_get_current_pid_tgid();
    u32 pid = pid_tgid >> 32;
    
    u32 *target = bpf_map_lookup_elem(&target_pid, &pid);
    if (!target) {
        return 0;
    }
    
    u64 *tsp = bpf_map_lookup_elem(&start_time, &pid_tgid);
    if (tsp) {
        u64 duration = bpf_ktime_get_ns() - *tsp;
        
        // Увеличиваем счётчик вызовов
        u64 *calls = bpf_map_lookup_elem(&function_calls, &pid_tgid);
        if (calls) {
            __sync_fetch_and_add(calls, 1);
        } else {
            u64 one = 1;
            bpf_map_update_elem(&function_calls, &pid_tgid, &one, BPF_ANY);
        }
        
        // Добавляем время выполнения
        u64 *cycles = bpf_map_lookup_elem(&function_cycles, &pid_tgid);
        if (cycles) {
            __sync_fetch_and_add(cycles, duration);
        } else {
            bpf_map_update_elem(&function_cycles, &pid_tgid, &duration, BPF_ANY);
        }
        
        bpf_map_delete_elem(&start_time, &pid_tgid);
    }
    return 0;
}

char LICENSE[] SEC("license") = "GPL";
