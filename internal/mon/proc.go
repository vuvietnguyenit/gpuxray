package mon

import (
	"fmt"
	"os"
	"strings"
)

// ProcessInfo holds metadata about a process using the GPU.
type ProcessInfo struct {
	PID  uint32
	Comm string // executable name (e.g. "python3")
	Args string // full command line joined
	GPU  string // GPU UUID or index label
}

// PIDResolver maps a PID to optional Kubernetes metadata.
// Implement this interface to plug in a real k8s resolver.
type PIDResolver interface {
	// Resolve attempts to return PodInfo for the given PID.
	// Returns nil, nil when the PID is not found in any pod.
}

// NoopResolver is the default resolver that returns nothing.
// Replace with a real implementation (e.g. via /proc + cgroup inspection
// or the Kubernetes Downward API + container runtime socket) when needed.
type NoopResolver struct{}

// readFile is a thin wrapper around os.ReadFile, centralised so tests can
// swap it out without touching every call site.
var readFile = func(path string) (string, error) {
	b, err := os.ReadFile(path)
	return string(b), err
}

// procComm reads the comm (executable name) for a PID from /proc/<pid>/comm.
func procComm(pid uint32) string {
	data, err := readFile(fmt.Sprintf("/proc/%d/comm", pid))
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(data)
}

// procArgs reads the full command line for a PID from /proc/<pid>/cmdline.
// Arguments are NUL-separated; we join them with spaces.
func procArgs(pid uint32) string {
	data, err := readFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return ""
	}
	parts := strings.Split(data, "\x00")
	var filtered []string
	for _, p := range parts {
		if p != "" {
			filtered = append(filtered, p)
		}
	}
	return strings.Join(filtered, " ")
}
