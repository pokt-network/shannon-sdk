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
	"sort"
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
	// Benchmarks include: Sign, SignWithCachedPrivateKey, SerializeSignature, SignLargePayload,
	// SignCore, SignReuseRing and ring signature verification (VerifyRingSignature*,
	// DeserializeAndVerifyRingSignature*)
	args = append(args,
		"-bench=Benchmark(Sign$|SignWithCachedPrivateKey$|SerializeSignature$|SignLargePayload$|SignCore$|SignReuseRing$|VerifyRingSignature.*$|DeserializeAndVerifyRingSignature.*$)",
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
	header := fmt.Sprintf("%-34s %15s %15s %12s", "Benchmark", "Time", "Bytes/op", "Allocs/op")
	fmt.Printf("%s%s%s\n", colorCyan, header, colorReset)
	fmt.Printf("%s%s%s\n", colorCyan, strings.Repeat("-", len(header)), colorReset)
	for _, r := range results {
		fmt.Printf("%-34s %15s %15d %12d\n", r.Name, formatTime(r.NsOp), r.BytesOp, r.AllocsOp)
	}
}

func displayComparison(portable, eth []BenchmarkResult) {
	fmt.Printf("\n%s=== SDK Performance Comparison ===%s\n\n", colorGreen, colorReset)
	ethMap := make(map[string]BenchmarkResult)
	for _, r := range eth {
		ethMap[r.Name] = r
	}
    header := fmt.Sprintf("%-34s %17s %17s %16s %14s %10s", "Benchmark", "Portable", "Ethereum", "Improvement", "Δ Time", "Δ %")
    fmt.Printf("%s%s%s\n", colorCyan, header, colorReset)
    fmt.Printf("%s%s%s\n", colorCyan, strings.Repeat("-", len(header)), colorReset)
    // collections for summary statistics
    var summaryImprovements []float64
    var summaryPctDeltas []float64

    for _, p := range portable {
        if e, ok := ethMap[p.Name]; ok {
            dTime := formatTime(p.NsOp)
            eTime := formatTime(e.NsOp)
            improvement := p.NsOp / e.NsOp
            // collect for summary
            // ignore pathological zero values
            // add to slices below outside the loop
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
            // Pre-pad then colorize so spacing remains consistent with ANSI codes
            dTimePad := fmt.Sprintf("%17s", dTime)
            eTimePad := fmt.Sprintf("%17s", eTime)
            dTimeCol := fmt.Sprintf("%s%s%s", colorBlue, dTimePad, colorReset)
            eTimeCol := fmt.Sprintf("%s%s%s", colorGreen, eTimePad, colorReset)
            // Delta time (Eth - Port): negative is better (green), positive worse (red)
            deltaNs := e.NsOp - p.NsOp
            deltaColor := colorRed
            if deltaNs < 0 {
                deltaColor = colorGreen
            } else if deltaNs == 0 {
                deltaColor = colorYellow
            }
            // format delta with sign and human-readable unit
            deltaStr := formatTimeAbs(deltaNs)
            // percent delta relative to Portable: (Eth-Port)/Port * 100
            pct := 0.0
            if p.NsOp != 0 {
                pct = (e.NsOp - p.NsOp) / p.NsOp * 100.0
            }
            pctStr := fmt.Sprintf("%+.1f%%", pct)
            pctColor := colorRed
            if pct < 0 {
                pctColor = colorGreen
            } else if pct == 0 {
                pctColor = colorYellow
            }
            // Pre-pad improvement, delta and percent, then color
            impPad := fmt.Sprintf("%16s", impStr)
            impCol := fmt.Sprintf("%s%s%s", color, impPad, colorReset)
            deltaPad := fmt.Sprintf("%14s", deltaStr)
            deltaCol := fmt.Sprintf("%s%s%s", deltaColor, deltaPad, colorReset)
            pctPad := fmt.Sprintf("%10s", pctStr)
            pctCol := fmt.Sprintf("%s%s%s", pctColor, pctPad, colorReset)

            namePad := fmt.Sprintf("%-34s", p.Name)
            fmt.Printf("%s %s %s %s %s %s\n", namePad, dTimeCol, eTimeCol, impCol, deltaCol, pctCol)
            // Append to summary collections
            if p.NsOp > 0 && e.NsOp > 0 {
                summaryImprovements = append(summaryImprovements, improvement)
                summaryPctDeltas = append(summaryPctDeltas, pct)
            }
        }
    }

	// Summary section
	if len(summaryImprovements) > 0 {
		avgImp := avgFloat64(summaryImprovements)
		medianImp := medianFloat64(summaryImprovements)
		avgPct := avgFloat64(summaryPctDeltas)

		impColor := colorRed
		if avgImp > 1 {
			impColor = colorGreen
		} else if avgImp == 1 {
			impColor = colorYellow
		}

		pctColor := colorRed
		if avgPct < 0 {
			pctColor = colorGreen
		} else if avgPct == 0 {
			pctColor = colorYellow
		}

		fmt.Println()
		fmt.Printf("%sSummary%s\n", colorCyan, colorReset)
		fmt.Printf("  Benchmarks: %d\n", len(summaryImprovements))
		fmt.Printf("  Avg speedup: %s%.2fx%s\n", impColor, avgImp, colorReset)
		fmt.Printf("  Median speedup: %s%.2fx%s\n", impColor, medianImp, colorReset)
		fmt.Printf("  Avg Δ%%%%: %s%+.1f%%%s\n", pctColor, avgPct, colorReset)
	}

	fmt.Println()
	fmt.Printf("%s=== Memory Usage Comparison ===%s\n\n", colorCyan, colorReset)
    // Columns: Benchmark | Portable B/op | Portable Allocs | Ethereum B/op | Ethereum Allocs | Δ B/op | Δ Allocs | Δ% B | Δ% Allocs
    memHeader := fmt.Sprintf("%-34s %14s %16s %16s %16s %10s %12s %8s %12s", "Benchmark", "Port B/op", "Port Allocs", "Eth B/op", "Eth Allocs", "Δ B/op", "Δ Allocs", "Δ% B", "Δ% Allocs")
    fmt.Printf("%s%s%s\n", colorCyan, memHeader, colorReset)
    fmt.Printf("%s%s%s\n", colorCyan, strings.Repeat("-", len(memHeader)), colorReset)
    for _, p := range portable {
        if e, ok := ethMap[p.Name]; ok {
            pB := fmt.Sprintf("%14d", p.BytesOp)
            pA := fmt.Sprintf("%16d", p.AllocsOp)
            eB := fmt.Sprintf("%16d", e.BytesOp)
            eA := fmt.Sprintf("%16d", e.AllocsOp)
            // Deltas (Eth - Port): negative is better (green), positive worse (red)
            dB := e.BytesOp - p.BytesOp
            dA := e.AllocsOp - p.AllocsOp
            dBStr := fmt.Sprintf("%+10d", dB)
            dAStr := fmt.Sprintf("%+12d", dA)
            dBColor := colorRed
            dAColor := colorRed
            if dB < 0 {
                dBColor = colorGreen
            } else if dB == 0 {
                dBColor = colorYellow
            }
            if dA < 0 {
                dAColor = colorGreen
            } else if dA == 0 {
                dAColor = colorYellow
            }
            // Percent deltas relative to Portable
            pctB := 0.0
            pctA := 0.0
            if p.BytesOp != 0 {
                pctB = float64(e.BytesOp-p.BytesOp) / float64(p.BytesOp) * 100.0
            }
            if p.AllocsOp != 0 {
                pctA = float64(e.AllocsOp-p.AllocsOp) / float64(p.AllocsOp) * 100.0
            }
            pctBStr := fmt.Sprintf("%+8.1f%%", pctB)
            pctAStr := fmt.Sprintf("%+12.1f%%", pctA)
            pctBColor := colorRed
            pctAColor := colorRed
            if pctB < 0 {
                pctBColor = colorGreen
            } else if pctB == 0 {
                pctBColor = colorYellow
            }
            if pctA < 0 {
                pctAColor = colorGreen
            } else if pctA == 0 {
                pctAColor = colorYellow
            }
            // Colorize padded deltas and percents
            dBCol := fmt.Sprintf("%s%s%s", dBColor, dBStr, colorReset)
            dACol := fmt.Sprintf("%s%s%s", dAColor, dAStr, colorReset)
            pctBCol := fmt.Sprintf("%s%s%s", pctBColor, pctBStr, colorReset)
            pctACol := fmt.Sprintf("%s%s%s", pctAColor, pctAStr, colorReset)

            namePad := fmt.Sprintf("%-34s", p.Name)
            fmt.Printf("%s %s %s %s %s %s %s %s %s\n",
                namePad, pB, pA, eB, eA, dBCol, dACol, pctBCol, pctACol,
            )
        }
    }
}

// formatTimeAbs returns a signed, human-readable duration using formatTime on the absolute value
// and prefixes with '+' or '-' indicating the sign of ns.
func formatTimeAbs(ns float64) string {
	sign := "+"
	if ns < 0 {
		sign = "-"
		ns = -ns
	}
	return sign + " " + formatTime(ns)
}

// helpers for summary stats
func avgFloat64(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := 0.0
	for _, v := range xs {
		s += v
	}
	return s / float64(len(xs))
}

func medianFloat64(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	ys := append([]float64(nil), xs...)
	sort.Float64s(ys)
	n := len(ys)
	if n%2 == 1 {
		return ys[n/2]
	}
	return (ys[n/2-1] + ys[n/2]) / 2
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
