#!/usr/bin/env python3

import sys
import re
from collections import defaultdict

def format_time(ns):
    """Format nanoseconds into human-readable time units"""
    ns = float(ns)
    if ns >= 1_000_000:
        return f"{ns/1_000_000:.1f} ms"
    elif ns >= 1_000:
        return f"{ns/1_000:.1f} μs"
    else:
        return f"{ns:.0f} ns"

def format_memory(bytes_val):
    """Format bytes into human-readable memory units"""
    bytes_val = float(bytes_val)
    if bytes_val >= 1_048_576:
        return f"{bytes_val/1_048_576:.1f} MB"
    elif bytes_val >= 1_024:
        return f"{bytes_val/1_024:.1f} KB"
    else:
        return f"{bytes_val:.0f} B"

def format_number(num):
    """Format large numbers with K/M suffixes"""
    num = float(num)
    if num >= 1_000_000:
        return f"{num/1_000_000:.1f}M"
    elif num >= 1_000:
        return f"{num/1_000:.1f}K"
    else:
        return f"{num:.0f}"

def parse_benchmark_output():
    """Parse Go benchmark output from stdin"""
    data = defaultdict(lambda: defaultdict(dict))
    
    for line in sys.stdin:
        line = line.strip()
        if not line.startswith('Benchmark'):
            continue

        # Parse benchmark line: BenchmarkSigning_CosmosSDK-10  31327  37620 ns/op  1744 B/op  34 allocs/op
        parts = line.split()
        if len(parts) < 8:
            continue
            
        bench_name = parts[0]
        iterations = parts[1]
        ns_per_op = parts[2]
        bytes_per_op = parts[4]
        allocs_per_op = parts[6]
        
        # Extract operation and library from benchmark name
        # BenchmarkSigning_CosmosSDK-10 -> Signing, CosmosSDK
        # BenchmarkSigningNoCgo_CosmosSDK-10 -> Signing, CosmosSDK
        match = re.match(r'Benchmark(\w+?)(?:NoCgo)?_(\w+)-\d+', bench_name)
        if not match:
            continue
            
        operation, library = match.groups()
        # Clean up operation names
        operation = operation.replace('NoCgo', '')
        
        data[operation][library] = {
            'ns': float(ns_per_op),
            'bytes': float(bytes_per_op),
            'allocs': float(allocs_per_op),
            'iterations': int(iterations)
        }
    
    return data

def print_formatted_results(data):
    """Print formatted benchmark results"""
    operations = ['KeyGeneration', 'Signing', 'Verification']
    medals = ['🥇', '🥈', '🥉', '']
    
    for operation in operations:
        if operation not in data:
            continue
            
        print(f"\n\033[1m📊 {operation.upper()} PERFORMANCE:\033[0m")
        print(f"\033[1m{'Library':<15} {'Time/op':<12} {'Memory/op':<12} {'Allocs/op':<12} {'Iterations':<15}\033[0m")
        print(f"{'-------':<15} {'--------':<12} {'---------':<12} {'---------':<12} {'----------':<15}")
        
        # Sort libraries by performance (time)
        libs_sorted = sorted(data[operation].items(), key=lambda x: x[1]['ns'])
        
        for i, (library, metrics) in enumerate(libs_sorted):
            time_str = format_time(metrics['ns'])
            memory_str = format_memory(metrics['bytes'])
            allocs_str = format_number(metrics['allocs'])
            iter_str = format_number(metrics['iterations'])
            medal = medals[i] if i < len(medals)-1 else medals[-1]
            
            print(f"{library:<15} \033[32m{time_str:<12}\033[0m \033[34m{memory_str:<12}\033[0m \033[33m{allocs_str:<12}\033[0m \033[36m{iter_str:<15}\033[0m {medal}")

if __name__ == "__main__":
    data = parse_benchmark_output()
    if not data:
        print("❌ No benchmark data found. Check that:")
        print("   • Go benchmarks are running correctly")
        print("   • Benchmark names match the expected pattern")
        print("   • CGO dependencies are available if needed")
        sys.exit(1)
    print_formatted_results(data)
