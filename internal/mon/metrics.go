package mon

import "github.com/prometheus/client_golang/prometheus"

// metricLabels are the common label names attached to every metric.
var metricLabels = []string{
	"pid",
	"comm",
	"args",
	"gpu",
	"gpu_index",
}

// Metrics bundles all Prometheus descriptors managed by the collector.
type Metrics struct {
	// usedMemoryBytes reports how many bytes of GPU memory the process
	// is currently consuming.
	usedMemoryBytes *prometheus.Desc

	// smUtilization reports the GPU SM (streaming multiprocessor)
	// utilisation percentage for the process (0–100).
	// NOTE: per-process SM util requires NVML r470+ drivers; older
	// drivers return 0 for every process — the metric is still exposed
	// so dashboards don't break.
	smUtilization *prometheus.Desc

	// processCount is a gauge that is always 1 for each active
	// (pid, gpu) pair. Useful for alerting on "any process using GPU".
	processCount *prometheus.Desc
}

// newMetrics creates and registers all descriptors with the given
// Prometheus registerer (use prometheus.DefaultRegisterer in prod).
func newMetrics() *Metrics {
	m := &Metrics{
		usedMemoryBytes: prometheus.NewDesc(
			"gpu_process_used_memory_bytes",
			"GPU memory consumed by the process in bytes.",
			metricLabels, nil,
		),
		smUtilization: prometheus.NewDesc(
			"gpu_process_sm_utilization_percent",
			"GPU SM utilisation of the process (0–100). "+
				"Requires NVML r470+ drivers; returns 0 on older drivers.",
			metricLabels, nil,
		),
		processCount: prometheus.NewDesc(
			"gpu_process_active",
			"1 for each process currently using a GPU.",
			metricLabels, nil,
		),
	}
	return m
}
