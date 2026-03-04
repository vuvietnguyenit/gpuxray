// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

package mon

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"github.com/vuvietnguyenit/gpuxray/internal/pid"
)

// Collector implements prometheus.Collector.
// Each Collect() call scrapes live data from NVML.
type Collector struct {
	mu       sync.Mutex
	metrics  *Metrics
	resolver PIDResolver
	logger   *zerolog.Logger
	sess     *pid.NvmlSession // persistent; opened once in NewCollector
	lastSeen uint64
}

// NewCollector creates a Collector and registers it with reg.
// resolver may be nil — a NoopResolver is used in that case.
func NewCollector(reg prometheus.Registerer, resolver PIDResolver, logger *zerolog.Logger) (*Collector, error) {
	if resolver == nil {
		resolver = &NoopResolver{}
	}
	if logger == nil {
		var out io.Writer = os.Stderr
		l := zerolog.New(
			zerolog.ConsoleWriter{
				Out:        out,
				TimeFormat: "15:04:05",
			},
		).With().Timestamp().Logger()
		logger = &l
	}

	c := &Collector{
		metrics:  newMetrics(),
		resolver: resolver,
		logger:   logger,
	}

	if err := reg.Register(c); err != nil {
		return nil, fmt.Errorf("register collector: %w", err)
	}
	return c, nil
}

// Describe implements prometheus.Collector.
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.metrics.usedMemoryBytes
	ch <- c.metrics.smUtilization
	ch <- c.metrics.processCount
}

// gpuLabel returns a human-readable GPU identifier, preferring the UUID.
func (c *Collector) gpuLabel(dev nvml.Device, index int) string {
	uuid, ret := nvml.DeviceGetUUID(dev)
	if ret == nvml.SUCCESS {
		return uuid
	}
	return fmt.Sprintf("gpu%d", index)
}

// buildProcessInfo enriches a raw PID with /proc-derived metadata.
func (c *Collector) buildProcessInfo(pid uint32, gpuLabel string) ProcessInfo {
	return ProcessInfo{
		PID:  pid,
		Comm: procComm(pid),
		Args: procArgs(pid),
		GPU:  gpuLabel,
	}
}

// Collect implements prometheus.Collector.
// It is called by Prometheus on every scrape.
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	c.mu.Lock()
	defer c.mu.Unlock()

	deviceCount, ret := nvml.DeviceGetCount()
	if ret != nvml.SUCCESS {
		c.logger.Error().Msgf("nvml DeviceGetCount failed: %s", nvml.ErrorString(ret))
		return
	}

	for i := range deviceCount {
		dev, ret := nvml.DeviceGetHandleByIndex(i)
		if ret != nvml.SUCCESS {
			c.logger.Warn().Msgf("DeviceGetHandleByIndex failed: %s", nvml.ErrorString(ret))
			continue
		}
		smUtil, err := c.smUtilForPIDs(dev)
		fmt.Println("gpu", i, smUtil)
		if err != nil {
			c.logger.Warn().Msgf("Failed to get SM utilization: %s", err)
		}
		gpuLabel := c.gpuLabel(dev, i)
		c.collectDevice(ch, dev, gpuLabel, i, smUtil)
	}
}

// collectDevice emits metrics for all processes on a single GPU device.
func (c *Collector) collectDevice(ch chan<- prometheus.Metric, dev nvml.Device, gpuLabel string, gpuIdx int, smUtil map[uint32]uint32) {
	procs, ret := nvml.DeviceGetComputeRunningProcesses(dev)
	if ret != nvml.SUCCESS {
		c.logger.Warn().Msgf("DeviceGetComputeRunningProcesses failed: %s", nvml.ErrorString(ret))
		return
	}

	// Also collect graphics processes (games, Vulkan/OpenGL workloads).
	gfxProcs, ret := nvml.DeviceGetGraphicsRunningProcesses(dev)
	if ret == nvml.SUCCESS {
		procs = append(procs, gfxProcs...)
	}

	// Deduplicate by PID (a process can appear in both lists).
	seen := make(map[uint32]struct{})
	for _, p := range procs {
		pid := uint32(p.Pid)
		if _, dup := seen[pid]; dup {
			continue
		}
		seen[pid] = struct{}{}

		info := c.buildProcessInfo(pid, gpuLabel)

		labels := buildLabelValues(info, gpuIdx)

		ch <- prometheus.MustNewConstMetric(
			c.metrics.usedMemoryBytes,
			prometheus.GaugeValue,
			float64(p.UsedGpuMemory),
			labels...,
		)
		ch <- prometheus.MustNewConstMetric(
			c.metrics.processCount,
			prometheus.GaugeValue,
			1,
			labels...,
		)

		// Per-process SM util (driver >= r470 required).
		ch <- prometheus.MustNewConstMetric(
			c.metrics.smUtilization,
			prometheus.GaugeValue,
			float64(smUtil[pid]),
			labels...,
		)
	}
}

// smUtilForPIDs returns the SM utilisation for a single process.
func (c *Collector) smUtilForPIDs(dev nvml.Device) (map[uint32]uint32, error) {
	samples, ret := nvml.DeviceGetProcessUtilization(dev, c.lastSeen)
	if ret != nvml.SUCCESS {
		if ret != nvml.ERROR_NOT_FOUND { // this is not error
			return nil, fmt.Errorf("nvml error: %v", ret)
		}
	}
	result := make(map[uint32]uint32)

	var maxTimestamp uint64 = c.lastSeen

	for _, s := range samples {
		result[uint32(s.Pid)] = s.SmUtil

		if s.TimeStamp > maxTimestamp {
			maxTimestamp = s.TimeStamp
		}
	}

	c.lastSeen = maxTimestamp

	return result, nil
}

// buildLabelValues returns the ordered label value slice matching metricLabels.
func buildLabelValues(info ProcessInfo, gpuIdx int) []string {
	return []string{
		strconv.FormatUint(uint64(info.PID), 10),
		info.Comm,
		info.Args,
		info.GPU,
		strconv.FormatInt(int64(gpuIdx), 10),
	}
}
