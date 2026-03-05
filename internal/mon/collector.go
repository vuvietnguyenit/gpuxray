// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

package mon

import (
	"fmt"
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

type GPUMemory struct {
	Total uint64
	Used  uint64
	Free  uint64
}

// NewCollector creates a Collector and registers it with reg.
// resolver may be nil — a NoopResolver is used in that case.
func NewCollector(reg prometheus.Registerer, resolver PIDResolver, logger *zerolog.Logger) (*Collector, error) {
	if resolver == nil {
		resolver = &NoopResolver{}
	}
	if logger == nil {
		panic("need configurate logger")
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
	ch <- c.metrics.totalMemory
	ch <- c.metrics.usedMemory
	ch <- c.metrics.freeMemory
	ch <- c.metrics.usedProcessMemoryBytes
	ch <- c.metrics.smProcessUtilization
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
		m, err := c.getMemoryStat(dev)
		if err != nil {
			c.logger.Warn().Msgf("Failed to get total memory: %s", err)
		}
		c.logger.Debug().Msgf("metrics for GPU %d: total=%d used=%d free=%d", i, m.Total, m.Used, m.Free)
		if err != nil {
			c.logger.Warn().Msgf("Failed to get SM utilization: %s", err)
		}
		gpuLabel := c.gpuLabel(dev, i)
		c.collectDevice(ch, dev, gpuLabel, i, smUtil, m)
	}
}

// collectDevice emits metrics for all processes on a single GPU device.
func (c *Collector) collectDevice(ch chan<- prometheus.Metric, dev nvml.Device, gpuLabel string, gpuIdx int, smUtil map[uint32]uint32, mem *GPUMemory) {
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

	// Global GPU metrics (not per-process).
	ch <- prometheus.MustNewConstMetric(
		c.metrics.totalMemory,
		prometheus.GaugeValue,
		float64(mem.Total),
		gpuLabel, strconv.FormatInt(int64(gpuIdx), 10),
	)
	ch <- prometheus.MustNewConstMetric(
		c.metrics.usedMemory,
		prometheus.GaugeValue,
		float64(mem.Used),
		gpuLabel, strconv.FormatInt(int64(gpuIdx), 10),
	)
	ch <- prometheus.MustNewConstMetric(
		c.metrics.freeMemory,
		prometheus.GaugeValue,
		float64(mem.Free),
		gpuLabel, strconv.FormatInt(int64(gpuIdx), 10),
	)
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
		c.logger.Debug().Msgf("process %d (%s) on GPU %d: used_memory=%d sm_util=%d%%", pid, info.Comm, gpuIdx, p.UsedGpuMemory, smUtil[pid])

		ch <- prometheus.MustNewConstMetric(
			c.metrics.usedProcessMemoryBytes,
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
			c.metrics.smProcessUtilization,
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

// getMemoryStat returns the framebuffer (VRAM) memory statistics
// for the given NVML device handle.
//
// The returned values are in bytes and include:
//   - Total: total physical GPU memory available to the device
//   - Used:  memory currently allocated
//   - Free:  remaining available memory
//
// If the device handle represents a MIG device, the values reflect
// the memory assigned to that specific MIG instance rather than the
// full physical GPU.
//
// This function wraps nvmlDeviceGetMemoryInfo and returns an error
// if the underlying NVML call fails.
//
// Note:
//   - Total includes driver-reserved memory.
//   - This does not include BAR1 memory.
//   - Returned values are raw bytes; convert to MB/GB if needed.
func (c *Collector) getMemoryStat(dev nvml.Device) (*GPUMemory, error) {
	memInfo, ret := nvml.DeviceGetMemoryInfo(dev)
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf("failed to get memory info: %s", nvml.ErrorString(ret))
	}
	return &GPUMemory{
		Total: memInfo.Total,
		Used:  memInfo.Used,
		Free:  memInfo.Free,
	}, nil
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
