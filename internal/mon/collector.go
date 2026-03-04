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

// Collect implements prometheus.Collector.
// It is called by Prometheus on every scrape.
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.sess == nil {
		sess, err := pid.OpenNVMLSession()
		if err != nil {
			c.logger.Error().Msgf("Failed to open NVML session: %v", err)
			return
		}
		c.sess = sess
	}

	inspections, err := pid.GetRunningProcesses(c.sess)
	if err != nil {
		c.logger.Error().Msgf("GetRunningProcesses failed: %v", err)
		return
	}

	for _, insp := range inspections {
		for _, e := range insp.Errors {
			c.logger.Warn().Msgf("inspection warning: %v", e)
		}

		for _, gpu := range insp.GPUs {
			labels := buildLabelValues(insp.Process, gpu)
			// get smUtils for this PID on this GPU
			smUtils, err := c.smUtilForPIDs(gpu.Device)
			if err != nil {
				c.logger.Warn().Msgf("Failed to get SM util for PID %d on GPU %s: %v", insp.Process.PID, gpu.UUID, err)
				continue
			}
			ch <- prometheus.MustNewConstMetric(
				c.metrics.usedMemoryBytes,
				prometheus.GaugeValue,
				float64(gpu.UsedMemBytes),
				labels...,
			)
			ch <- prometheus.MustNewConstMetric(
				c.metrics.processCount,
				prometheus.GaugeValue,
				1,
				labels...,
			)
			ch <- prometheus.MustNewConstMetric(
				c.metrics.smUtilization,
				prometheus.GaugeValue,
				float64(smUtils[insp.Process.PID]),
				labels...,
			)
		}
	}
}

// smUtilForPIDs returns the SM utilisation for a single process.
func (c *Collector) smUtilForPIDs(dev nvml.Device) (map[uint32]uint32, error) {
	samples, ret := nvml.DeviceGetProcessUtilization(dev, c.lastSeen)
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf("nvml error: %v", ret)
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
func buildLabelValues(info pid.USProcessInfo, gpu pid.GPUUsage) []string {
	return []string{
		strconv.FormatUint(uint64(info.PID), 10),
		info.Comm,
		info.Args,
		gpu.UUID,
		strconv.FormatInt(int64(gpu.DeviceIndex), 10),
	}
}
