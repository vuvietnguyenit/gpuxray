// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

package memtrace

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/cilium/ebpf/ringbuf"
	"github.com/vuvietnguyenit/gpuxray/internal"
	"github.com/vuvietnguyenit/gpuxray/internal/event"
	"github.com/vuvietnguyenit/gpuxray/internal/logging"
	"github.com/vuvietnguyenit/gpuxray/internal/pid"
)

type Tracer struct {
	rd       *ringbuf.Reader
	pidCache *pid.PIDCache
}

func NewTracer(rd *ringbuf.Reader) *Tracer {
	return &Tracer{rd: rd, pidCache: pid.GlobalPIDCache()}
}

func NewRingbufReader(objs *Objects) (*ringbuf.Reader, error) {
	return ringbuf.NewReader(objs.MemleakRingbufEvents)
}

func (t *Tracer) Run(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		logging.L().Debug().Msg("SIGTERM received, closing ringbuf")
		t.rd.Close()
	}()
	aggregator := NewLeakAggregator()
	go StartSnapshotPrinter(ctx, aggregator, time.Duration(internal.FetchInterval)*time.Second)
	for {
		record, err := t.rd.Read()
		if err != nil {
			if errors.Is(err, ringbuf.ErrClosed) {
				logging.L().Debug().Msg("ringbuf closed, exiting")
				return nil
			}
			logging.L().Error().Err(err).Msg("ringbuf read failed")
			continue
		}
		ev, err := decodeMemoryEvent(record.RawSample)
		if err != nil {
			logging.L().Error().
				Err(err).
				Msg("event decode failed")
			continue
		}
		inspector := t.pidCache.GetOrInspect(
			ev.Process.Process.PID,
			func(p uint32) (pid.PIDInspection, error) {
				return pid.InspectPID(int32(p)), nil
			},
		)

		aggregator.Consume(ev, inspector)

	}
}

// Detect memory leaks based on allocation and free events
type LeakKey struct {
	PID         uint32
	TID         int
	DeviceIndex int
	UUID        string
}

type MemoryLeakResult struct {
	Key        LeakKey
	Process    pid.USProcessInfo
	AllocBytes uint64
	FreeBytes  uint64
	Leaked     uint64
	AllocCount uint64
	FreeCount  uint64
}

type LeakAggregator struct {
	mu    sync.Mutex
	state map[LeakKey]*MemoryLeakResult
}

func NewLeakAggregator() *LeakAggregator {
	return &LeakAggregator{
		state: make(map[LeakKey]*MemoryLeakResult),
	}
}

func (a *LeakAggregator) Consume(
	ev event.MemoryEvent,
	inspection pid.PIDInspection,
) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for _, gpu := range inspection.GPUs {
		key := LeakKey{
			PID:         ev.Process.Process.PID,
			TID:         ev.TID,
			DeviceIndex: gpu.DeviceIndex,
			UUID:        gpu.UUID,
		}

		entry, ok := a.state[key]
		if !ok {
			entry = &MemoryLeakResult{
				Key:     key,
				Process: ev.Process.Process,
			}
			a.state[key] = entry
		}

		switch ev.Op {
		case event.MemAlloc:
			entry.AllocBytes += ev.Bytes
			entry.AllocCount++

		case event.MemFree:
			entry.FreeBytes += ev.Bytes
			entry.FreeCount++
		}
	}
}

// Use this function whatever want to show the result of memory leak aggregation
func (a *LeakAggregator) Snapshot() []MemoryLeakResult {
	a.mu.Lock()
	defer a.mu.Unlock()

	out := make([]MemoryLeakResult, 0, len(a.state))
	for _, v := range a.state {
		leaked := uint64(0)
		if v.AllocBytes > v.FreeBytes {
			leaked = v.AllocBytes - v.FreeBytes
		}

		out = append(out, MemoryLeakResult{
			Key:        v.Key,
			Process:    v.Process,
			AllocBytes: v.AllocBytes,
			FreeBytes:  v.FreeBytes,
			Leaked:     leaked,
			AllocCount: v.AllocCount,
			FreeCount:  v.FreeCount,
		})
	}
	return out
}

func PrintHeader() {
	fmt.Printf(
		"%-10s %-8s %-4s %-12s %-8s %-8s %-16s\n",
		"TIME",
		"PID",
		"GPU",
		"ALLOC_MB",
		"AL_CNT",
		"FR_CNT",
		"COMM",
	)
}

func PrintLeakStat(ts time.Time, r MemoryLeakResult) {
	fmt.Printf(
		"%-10s %-8d %-4d %-12s %-8d %-8d %-16s\n",
		ts.Format("15:04:05"),
		r.Key.PID,
		r.Key.DeviceIndex,
		internal.HumanBytes(r.Leaked),
		r.AllocCount,
		r.FreeCount,
		internal.Truncate(r.Process.Comm, 16),
	)
}

func StartSnapshotPrinter(
	ctx context.Context,
	agg *LeakAggregator,
	interval time.Duration,
) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	PrintHeader()
	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			ts := time.Now()
			results := agg.Snapshot()

			for _, r := range results {
				PrintLeakStat(ts, r)
			}
		}
	}
}
