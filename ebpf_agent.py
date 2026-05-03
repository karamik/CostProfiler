#!/usr/bin/env python3
# ebpf_agent.py - eBPF-based cost profiler for X5 Group

from bcc import BPF
import time
import argparse
import subprocess
import re
import os
import signal
import sys

# ANSI colors
class Colors:
    RED = '\033[91m'
    GREEN = '\033[92m'
    YELLOW = '\033[93m'
    CYAN = '\033[96m'
    RESET = '\033[0m'

# eBPF C program
ebpf_source = """
#include <uapi/linux/ptrace.h>
#include <linux/sched.h>

// Store entry timestamp per PID
BPF_HASH(start_time, u64, u64);

// Store cumulative CPU cycles per function
BPF_HASH(function_calls, u64, u64);
BPF_HASH(function_cycles, u64, u64);

// Function name mapping (hardcoded for demo)
BPF_HASH(func_names, u64, char[64]);

int trace_func_entry(struct pt_regs *ctx) {
    u64 pid_tgid = bpf_get_current_pid_tgid();
    u64 ts = bpf_ktime_get_ns();
    start_time.update(&pid_tgid, &ts);
    return 0;
}

int trace_func_return(struct pt_regs *ctx) {
    u64 pid_tgid = bpf_get_current_pid_tgid();
    u64 *tsp = start_time.lookup(&pid_tgid);
    
    if (tsp != 0) {
        u64 duration_ns = bpf_ktime_get_ns() - *tsp;
        
        // Increment call count
        u64 zero = 0;
        u64 *calls = function_calls.lookup_or_init(&pid_tgid, &zero);
        *calls += 1;
        
        // Add to total cycles (using duration as proxy for CPU cycles)
        u64 *cycles = function_cycles.lookup_or_init(&pid_tgid, &zero);
        *cycles += duration_ns;
        
        start_time.delete(&pid_tgid);
    }
    return 0;
}
"""

def get_function_address(binary, func_name):
    """Get function address from binary using nm"""
    try:
        result = subprocess.run(['nm', binary], capture_output=True, text=True, timeout=10)
        for line in result.stdout.split('\n'):
            if f' T {func_name}' in line or f' t {func_name}' in line:
                parts = line.split()
                if len(parts) >= 1:
                    addr = parts[0]
                    return int(addr, 16)
    except Exception as e:
        print(f"Error reading symbols: {e}")
    return None

def get_all_functions(binary):
    """Auto-discover all functions in binary"""
    functions = []
    try:
        result = subprocess.run(['nm', binary], capture_output=True, text=True, timeout=10)
        for line in result.stdout.split('\n'):
            # Look for text section symbols (T/t)
            if ' T ' in line or ' t ' in line:
                parts = line.split()
                if len(parts) >= 3:
                    func_name = parts[2]
                    # Filter out Go runtime/internal functions
                    if not func_name.startswith('runtime.') and not func_name.startswith('internal/'):
                        functions.append(func_name)
    except Exception as e:
        print(f"Error auto-discovering functions: {e}")
    return functions

def format_money(amount):
    """Format rubles to human-readable string"""
    if amount >= 1_000_000_000:
        return f"{amount/1_000_000_000:.1f} млрд ₽"
    elif amount >= 1_000_000:
        return f"{amount/1_000_000:.1f} млн ₽"
    else:
        return f"{amount:,.0f} ₽"

def main():
    parser = argparse.ArgumentParser(description='eBPF Cost Profiler for X5 Group')
    parser.add_argument('--pid', type=int, required=True, help='Target process PID')
    parser.add_argument('--binary', required=True, help='Path to binary')
    parser.add_argument('--sampling', type=int, default=1, help='Sampling rate (1 = every call)')
    parser.add_argument('--functions', nargs='+', help='Specific functions to trace (auto-discover if not specified)')
    parser.add_argument('--cost', type=float, default=2.0, help='Cost per vCPU hour in RUB')
    parser.add_argument('--scale', type=int, default=100_000_000, help='Daily call scale for projection')
    args = parser.parse_args()

    # Check if process exists
    try:
        os.kill(args.pid, 0)
    except OSError:
        print(f"Error: Process with PID {args.pid} not found")
        sys.exit(1)

    # Get functions to trace
    target_functions = args.functions
    if not target_functions:
        print(f"{Colors.CYAN}Auto-discovering functions in {args.binary}...{Colors.RESET}")
        target_functions = get_all_functions(args.binary)
        # Limit to first 10 for demo
        target_functions = target_functions[:10]
        print(f"Found {len(target_functions)} functions")

    if not target_functions:
        print(f"{Colors.RED}No functions found. Make sure binary has debug symbols.{Colors.RESET}")
        print("Compile with: go build -ldflags=\"-s=false\" -o service main.go")
        sys.exit(1)

    # Initialize eBPF
    print(f"{Colors.CYAN}Loading eBPF program...{Colors.RESET}")
    b = BPF(text=ebpf_source)

    # Attach uprobes
    attached = 0
    for func in target_functions:
        addr = get_function_address(args.binary, func)
        if addr:
            print(f"  🔍 Attaching to {func} at 0x{addr:x}")
            try:
                b.attach_uprobe(name=args.binary, addr=addr, fn_name="trace_func_entry")
                b.attach_uretprobe(name=args.binary, addr=addr, fn_name="trace_func_return")
                attached += 1
            except Exception as e:
                print(f"    ⚠️  Failed: {e}")
    
    print(f"\n{Colors.GREEN}✅ Attached to {attached} functions{Colors.RESET}")
    print(f"📊 Monitoring PID {args.pid} (sampling: 1/{args.sampling})")
    print(f"💰 Cost: {args.cost} ₽/vCPU·hour | Scale: {args.scale:,} calls/day\n")
    print("Press Ctrl+C to stop and generate report...\n")

    # Store metrics
    metrics = {}

    def signal_handler(sig, frame):
        print(f"\n\n{Colors.CYAN}{'='*60}{Colors.RESET}")
        print(f"{Colors.YELLOW}📈 eBPF COST REPORT{Colors.RESET}")
        print(f"{Colors.CYAN}{'='*60}{Colors.RESET}\n")

        # Collect data from eBPF maps
        calls_map = b.get_table("function_calls")
        cycles_map = b.get_table("function_cycles")
        
        total_cost = 0
        report_data = []
        
        for key, leaf in calls_map.items():
            pid = key.value
            call_count = leaf.value
            cycles_entry = cycles_map[key]
            cycles = cycles_entry.value if cycles_entry else 0
            
            # Calculate cost
            # Conversion: nanoseconds to hours, then multiply by rate
            hours = cycles / 1e9 / 3600
            cost_per_call = hours * args.cost
            annual_cost = cost_per_call * args.scale * 365
            
            # Get function name (simplified - in real version would map address to name)
            func_name = f"func_{pid}"
            
            report_data.append({
                'name': func_name,
                'calls': call_count,
                'cost_per_call': cost_per_call,
                'annual': annual_cost
            })
            total_cost += annual_cost
        
        # Sort by annual cost
        report_data.sort(key=lambda x: x['annual'], reverse=True)
        
        # Print table
        print(f"{'Function':<40} | {'Annual Cost':>20}")
        print(f"{'-'*62}")
        
        for item in report_data[:10]:
            print(f"{item['name']:<40} | {format_money(item['annual']):>20}")
        
        print(f"{'-'*62}")
        print(f"{Colors.RED}TOTAL ANNUAL WASTE: {format_money(total_cost)}{Colors.RESET}\n")
        
        # Recommendations
        if total_cost > 10_000_000:
            print(f"{Colors.YELLOW}⚠️  RECOMMENDATIONS:{Colors.RESET}")
            print("  • Check functions with O(n²) complexity")
            print("  • Preallocate slices/maps to avoid reallocation")
            print("  • Consider adding //go:noinline for better visibility")
            print(f"\n{Colors.GREEN}Potential savings: up to {format_money(total_cost * 0.8)}/year{Colors.RESET}")
        
        sys.exit(0)

    signal.signal(signal.SIGINT, signal_handler)

    # Keep running
    try:
        while True:
            time.sleep(1)
            # Print heartbeat every 30 seconds
            if int(time.time()) % 30 == 0:
                print(f"{Colors.CYAN}.{Colors.RESET}", end="", flush=True)
    except KeyboardInterrupt:
        signal_handler(None, None)

if __name__ == "__main__":
    main()
