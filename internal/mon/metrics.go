// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

package mon

import "github.com/prometheus/client_golang/prometheus"

// pidMetricLabels are the common label names attached to every metric.
var pidMetricLabels = []string{
	"pid",
	"comm",
	"args",
	"gpu",
	"gpu_index",
}
var gpuMetricLabels = []string{
	"gpu",
	"gpu_index",
}

// Metrics bundles all Prometheus descriptors managed by the collector.
type Metrics struct {
	// usedProcessMemoryBytes reports how many bytes of GPU memory the process
	// is currently consuming.
	usedProcessMemoryBytes *prometheus.Desc

	// smProcessUtilization reports the GPU SM (streaming multiprocessor)
	// utilisation percentage for the process (0–100).
	// NOTE: per-process SM util requires NVML r470+ drivers; older
	// drivers return 0 for every process — the metric is still exposed
	// so dashboards don't break.
	smProcessUtilization *prometheus.Desc

	// processCount is a gauge that is always 1 for each active
	// (pid, gpu) pair. Useful for alerting on "any process using GPU".
	processCount *prometheus.Desc
	// Metrics related to GPU memory statistics.
	totalMemory *prometheus.Desc
	usedMemory  *prometheus.Desc
	freeMemory  *prometheus.Desc
}

// newMetrics creates and registers all descriptors with the given
// Prometheus registerer (use prometheus.DefaultRegisterer in prod).
func newMetrics() *Metrics {
	m := &Metrics{
		usedProcessMemoryBytes: prometheus.NewDesc(
			"gpu_process_used_memory_bytes",
			"GPU memory consumed by the process in bytes.",
			pidMetricLabels, nil,
		),
		totalMemory: prometheus.NewDesc(
			"gpu_total_memory_bytes",
			"Total GPU memory available in bytes.",
			gpuMetricLabels, nil,
		),
		usedMemory: prometheus.NewDesc(
			"gpu_used_memory_bytes",
			"GPU memory currently allocated in bytes.",
			gpuMetricLabels, nil,
		),
		freeMemory: prometheus.NewDesc(
			"gpu_free_memory_bytes",
			"Remaining available GPU memory for the process in bytes.",
			gpuMetricLabels, nil,
		),
		smProcessUtilization: prometheus.NewDesc(
			"gpu_process_sm_utilization_percent",
			"GPU SM utilisation of the process (0–100). "+
				"Requires NVML r470+ drivers; returns 0 on older drivers.",
			pidMetricLabels, nil,
		),
		processCount: prometheus.NewDesc(
			"gpu_process_active",
			"1 for each process currently using a GPU.",
			pidMetricLabels, nil,
		),
	}
	return m
}
