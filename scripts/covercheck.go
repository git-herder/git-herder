// Утилита проверки суммарного statement coverage по coverage.out (go tool cover).
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func main() {
	min := flag.Float64("min", 80, "minimum total coverage percent")
	profile := flag.String("profile", "coverage.out", "coverprofile path")
	flag.Parse()

	pct, ok := totalPercentFromGoTool(*profile)
	if !ok {
		fmt.Fprintf(os.Stderr, "covercheck: could not read coverage for %s\n", *profile)
		os.Exit(1)
	}
	if pct < *min {
		fmt.Fprintf(os.Stderr, "covercheck: %.1f%% < %.1f%% required\n", pct, *min)
		os.Exit(1)
	}
	fmt.Printf("covercheck: %.1f%% (>= %.1f%%)\n", pct, *min)
}

func totalPercentFromGoTool(profile string) (float64, bool) {
	out, err := exec.Command("go", "tool", "cover", "-func", profile).Output()
	if err != nil {
		return 0, false
	}
	lines := strings.Split(string(out), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "total:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			return 0, false
		}
		s := strings.TrimSuffix(fields[len(fields)-1], "%")
		pct, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, false
		}
		return pct, true
	}
	return 0, false
}

func parseCoverProfile(path string) (total, covered int, ok bool) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, false
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "mode:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		count, err := strconv.Atoi(fields[len(fields)-1])
		if err != nil {
			continue
		}
		total++
		if count > 0 {
			covered++
		}
	}
	return total, covered, sc.Err() == nil
}
