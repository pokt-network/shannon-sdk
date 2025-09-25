// Shannon SDK Benchmark Runner
//
// Usage:
//   go run cmd/benchmark/main.go -compare         # Full comparison
//   go run cmd/benchmark/main.go -report          # Quick report
//   go run cmd/benchmark/main.go -compare -duration=10s  # Custom duration

package main

import (
    "bufio"
    "flag"
    "fmt"
    "os"
    "os/exec"
    "regexp"
    "strconv"
    "strings"
)

const (
    colorReset  = "\033[0m"
    colorRed    = "\033[0;31m"
    colorGreen  = "\033[0;32m"
    colorBlue   = "\033[0;34m"
    colorYellow = "\033[1;33m"
    colorCyan   = "\033[0;36m"
)

type BenchmarkResult struct {
    Name     string
    NsOp     float64
    BytesOp  int
    AllocsOp int
}

func main() {
    var (
        report   = flag.Bool("report", false, "Generate a formatted report")
        compare  = flag.Bool("compare", false, "Run comparison between backends")
        duration = flag.String("duration", "3s", "Benchmark duration")
    )
    flag.Parse()

    if *report || *compare {
        runComparison(*duration)
    } else {
        fmt.Println("Usage: go run cmd/benchmark/main.go [options]")
        fmt.Println("  -compare   Run full comparison between backends")
        fmt.Println("  -report    Generate a quick performance report")
        fmt.Println("  -duration  Benchmark duration (default: 3s)")
    }
}

func runComparison(duration string) {
    fmt.Printf("%s🔬 Shannon SDK Benchmark Comparison%s\n", colorBlue, colorReset)
    fmt.Println("====================================")
    fmt.Println()

    if !checkCGO() {
        fmt.Printf("%s⚠️  CGO not available or gcc missing. Only portable backend will be tested.%s\n\n", colorYellow, colorReset)
    }

    fmt.Printf("%s📊 Testing Portable Backend (Pure Go)%s\n", colorBlue, colorReset)
    portable := runBenchmarks("", "0", duration)

    var eth []BenchmarkResult
    if checkCGO() {
        fmt.Printf("\n%s📊 Testing Ethereum Backend (libsecp256k1)%s\n", colorBlue, colorReset)
        eth = runBenchmarks("-tags=ethereum_secp256k1", "1", duration)
    }

    if len(eth) > 0 {
        displayComparison(portable, eth)
    } else {
        displaySingle(portable)
    }
}

func checkCGO() bool {
    out, err := exec.Command("go", "env", "CGO_ENABLED").Output()
    if err != nil {
        return false
    }
    if exec.Command("gcc", "--version").Run() != nil {
        return false
    }
    return strings.TrimSpace(string(out)) != "0"
}

// runBenchmarks runs SDK benchmarks that reflect signing cost differences.
func runBenchmarks(tags, cgoEnabled, duration string) []BenchmarkResult {
    args := []string{"test"}
    if tags != "" {
        args = append(args, tags)
    }
    // Target the root package (where benchmarks live)
    // Benchmarks include: BenchmarkSign, BenchmarkSignWithCachedPrivateKey, BenchmarkSerializeSignature, BenchmarkSignLargePayload,
    // and additional SDK-focused ones that reduce non-crypto overhead: BenchmarkSignCore, BenchmarkSignReuseRing
    args = append(args,
        "-bench=Benchmark(Sign$|SignWithCachedPrivateKey$|SerializeSignature$|SignLargePayload$|SignCore$|SignReuseRing$)",
        "-benchmem",
        "-run=^$",
        "-benchtime="+duration,
        ".",
    )

    cmd := exec.Command("go", args...)
    cmd.Env = append(os.Environ(), "CGO_ENABLED="+cgoEnabled)
    output, err := cmd.CombinedOutput()
    if err != nil {
        fmt.Printf("%s❌ Error running benchmarks: %v%s\n", colorRed, err, colorReset)
        fmt.Println(string(output))
        return nil
    }
    return parseBenchmarkOutput(string(output))
}

// no ensureBackend; we rely on tags + CGO and report if no improvement is observed.

func parseBenchmarkOutput(output string) []BenchmarkResult {
    var results []BenchmarkResult
    // Example: BenchmarkSign-10  1000  1234567 ns/op  336 B/op  8 allocs/op
    re := regexp.MustCompile(`(Benchmark\w+)(?:-\d+)?\s+\d+\s+([\d.]+)\s+ns/op\s+(\d+)\s+B/op\s+(\d+)\s+allocs/op`)
    scanner := bufio.NewScanner(strings.NewReader(output))
    for scanner.Scan() {
        line := scanner.Text()
        m := re.FindStringSubmatch(line)
        if len(m) == 5 {
            ns, _ := strconv.ParseFloat(m[2], 64)
            b, _ := strconv.Atoi(m[3])
            a, _ := strconv.Atoi(m[4])
            results = append(results, BenchmarkResult{
                Name:     m[1],
                NsOp:     ns,
                BytesOp:  b,
                AllocsOp: a,
            })
        }
    }
    return results
}

func displaySingle(results []BenchmarkResult) {
    fmt.Printf("\n%s=== Portable Backend (Pure Go) ===%s\n\n", colorGreen, colorReset)
    fmt.Printf("%-30s %15s %15s %12s\n", "Benchmark", "Time", "Bytes/op", "Allocs/op")
    fmt.Println(strings.Repeat("-", 75))
    for _, r := range results {
        fmt.Printf("%-30s %15s %15d %12d\n", r.Name, formatTime(r.NsOp), r.BytesOp, r.AllocsOp)
    }
}

func displayComparison(portable, eth []BenchmarkResult) {
    fmt.Printf("\n%s=== SDK Performance Comparison ===%s\n\n", colorGreen, colorReset)
    ethMap := make(map[string]BenchmarkResult)
    for _, r := range eth {
        ethMap[r.Name] = r
    }
    fmt.Printf("%-30s %15s %15s %12s\n", "Benchmark", "Portable", "Ethereum", "Improvement")
    fmt.Println(strings.Repeat("-", 75))
    for _, p := range portable {
        if e, ok := ethMap[p.Name]; ok {
            dTime := formatTime(p.NsOp)
            eTime := formatTime(e.NsOp)
            improvement := p.NsOp / e.NsOp
            impStr := fmt.Sprintf("%.1fx", improvement)
            color := colorReset
            if improvement > 2 {
                color = colorGreen
                impStr = "🚀 " + impStr + " faster"
            } else if improvement > 1.5 {
                color = colorYellow
                impStr = "⚡ " + impStr + " faster"
            } else if improvement < 0.9 {
                color = colorRed
                impStr = "🐌 " + fmt.Sprintf("%.1fx slower", 1/improvement)
            }
            fmt.Printf("%-30s %15s %15s %s%12s%s\n", p.Name, dTime, eTime, color, impStr, colorReset)
        }
    }

    fmt.Println("\n" + strings.Repeat("-", 75))
    fmt.Printf("\n%s=== Memory Usage Comparison ===%s\n\n", colorCyan, colorReset)
    fmt.Printf("%-30s %20s %20s\n", "Benchmark", "Portable", "Ethereum")
    fmt.Println(strings.Repeat("-", 75))
    for _, p := range portable {
        if e, ok := ethMap[p.Name]; ok {
            pMem := fmt.Sprintf("%d B, %d allocs", p.BytesOp, p.AllocsOp)
            eMem := fmt.Sprintf("%d B, %d allocs", e.BytesOp, e.AllocsOp)
            fmt.Printf("%-30s %20s %20s\n", p.Name, pMem, eMem)
        }
    }
}

func formatTime(ns float64) string {
    switch {
    case ns >= 1_000_000_000:
        return fmt.Sprintf("%.1f s", ns/1_000_000_000)
    case ns >= 1_000_000:
        return fmt.Sprintf("%.1f ms", ns/1_000_000)
    case ns >= 1_000:
        return fmt.Sprintf("%.0f μs", ns/1_000)
    default:
        return fmt.Sprintf("%.0f ns", ns)
    }
}
