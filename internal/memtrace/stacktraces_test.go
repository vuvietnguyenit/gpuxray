package memtrace

import (
	"math"
	"testing"
)

func almostEqual(a, b float64) bool {
	const epsilon = 0.01
	return math.Abs(a-b) < epsilon
}

func TestBuildLeakDistribution_GroupingAndPercentage(t *testing.T) {
	active := map[uint64]Allocation{
		0x1: {Size: 100, GPU: 0, Function: "funcA"},
		0x2: {Size: 300, GPU: 0, Function: "funcA"},
		0x3: {Size: 600, GPU: 1, Function: "funcB"},
	}

	stats := BuildLeakDistribution(active)
	t.Log(stats)

	if len(stats) != 2 {
		t.Fatalf("expected 2 grouped functions, got %d", len(stats))
	}

	// Because sorting is descending by bytes,
	// funcB (600) should come before funcA (400)
	first := stats[0]
	second := stats[1]

	if first.Function != "funcB" {
		t.Fatalf("expected funcB first, got %s", first.Function)
	}

	if first.Bytes != 600 {
		t.Fatalf("expected 600 bytes for funcB, got %d", first.Bytes)
	}

	if !almostEqual(first.Percent, 60.0) {
		t.Fatalf("expected ~60%% for funcB, got %.2f", first.Percent)
	}

	if second.Function != "funcA" {
		t.Fatalf("expected funcA second, got %s", second.Function)
	}

	if second.Bytes != 400 {
		t.Fatalf("expected 400 bytes for funcA, got %d", second.Bytes)
	}

	if !almostEqual(second.Percent, 40.0) {
		t.Fatalf("expected ~40%% for funcA, got %.2f", second.Percent)
	}
}

// This test case represents that the leaks are grouped by function name, regardless of GPU differences.
func TestBuildLeakDistribution_SameFunctionDifferentGPU(t *testing.T) {
	active := map[uint64]Allocation{
		0x1: {Size: 100, GPU: 0, Function: "funcA"},
		0x2: {Size: 200, GPU: 1, Function: "funcA"},
	}

	stats := BuildLeakDistribution(active)
	t.Log(stats)
	if len(stats) != 2 {
		t.Fail()
	}
	if stats[0].Bytes != 200 {
		t.Fail()
	}
	if stats[1].Bytes != 100 {
		t.Fail()
	}

}

func TestBuildLeakDistribution_SmallVsLarge(t *testing.T) {
	active := map[uint64]Allocation{
		0x1: {Size: 1, Function: "small"},
		0x2: {Size: 1, Function: "small"},
		0x3: {Size: 1, Function: "small"},
		0x4: {Size: 1000, Function: "large"},
	}

	stats := BuildLeakDistribution(active)

	if stats[0].Function != "large" {
		t.Fatalf("expected large first, got %s", stats[0].Function)
	}

	if stats[0].Percent < 99 {
		t.Fatalf("expected large to dominate percent, got %.2f",
			stats[0].Percent)
	}
	t.Log(stats)
}
