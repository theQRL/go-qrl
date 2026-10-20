package conformance

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// qrvmoneRunnerEnv names the environment variable holding the path to the
// qrvmone conformance runner (qrvmone test/conformance, target
// qrvmone-conformance). hack/vm-conformance.sh builds it and sets this.
const qrvmoneRunnerEnv = "QRVMONE_CONFORMANCE_RUNNER"

// TestGoVMMatchesQrvmone runs every vector through this package's Go VM and
// through qrvmone, and requires identical return data, error class and gas.
// It compares the two runtimes with each other rather than with the vector
// expectations, so a divergence is reported even for vectors that do not pin
// gas. The test is skipped unless the runner is available.
func TestGoVMMatchesQrvmone(t *testing.T) {
	runner := os.Getenv(qrvmoneRunnerEnv)
	if runner == "" {
		t.Skipf("set %s to the qrvmone-conformance binary to compare against qrvmone", qrvmoneRunnerEnv)
	}
	cpp, err := runQrvmone(runner, Vectors)
	if err != nil {
		t.Fatalf("qrvmone runner: %v", err)
	}
	for i, v := range Vectors {
		t.Run(v.Name, func(t *testing.T) {
			goRes, err := Run(v)
			if err != nil {
				t.Fatalf("go runner: %v", err)
			}
			if goRes != cpp[i] {
				t.Errorf("runtimes diverge\n  go:      %+v\n  qrvmone: %+v", goRes, cpp[i])
			}
		})
	}
}

// runQrvmone feeds the vectors to the qrvmone runner and parses its results,
// one per vector and in order. The line protocol is documented in the runner.
func runQrvmone(runner string, vectors []Vector) ([]Result, error) {
	var in bytes.Buffer
	for _, v := range vectors {
		if strings.ContainsAny(v.Name, "\t\n") {
			return nil, fmt.Errorf("vector name %q contains a tab or newline", v.Name)
		}
		gas := v.GasLimit
		if gas == 0 {
			gas = DefaultGasLimit
		}
		fmt.Fprintf(&in, "%s\t%s\t%s\t%d\n", v.Name, v.BytecodeHex, v.InputHex, gas)
	}
	cmd := exec.Command(runner)
	cmd.Stdin = &in
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%v: %s", err, stderr.String())
	}
	var results []Result
	scanner := bufio.NewScanner(bytes.NewReader(out))
	scanner.Buffer(make([]byte, 0, 1<<20), 1<<20)
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), "\t")
		if len(fields) != 4 {
			return nil, fmt.Errorf("malformed runner output %q", scanner.Text())
		}
		if want := vectors[len(results)].Name; fields[0] != want {
			return nil, fmt.Errorf("runner output out of order: got %q, want %q", fields[0], want)
		}
		gasUsed, err := strconv.ParseUint(fields[3], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("gas used %q: %v", fields[3], err)
		}
		results = append(results, Result{ReturnHex: fields[1], Error: ErrorClass(fields[2]), GasUsed: gasUsed})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(results) != len(vectors) {
		return nil, fmt.Errorf("runner returned %d results for %d vectors", len(results), len(vectors))
	}
	return results, nil
}
