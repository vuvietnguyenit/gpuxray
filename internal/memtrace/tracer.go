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
	"github.com/vuvietnguyenit/gpuxray/internal/symbolizer"
)

type Tracer struct {
	pidCache   *pid.PIDCache
	ebpfObjs   *Objects
	symbolizer *symbolizer.Symbolizer
}

func NewTracer(o *Objects) *Tracer {
	return &Tracer{
		pidCache:   pid.GlobalPIDCache(),
		ebpfObjs:   o,
		symbolizer: symbolizer.New(),
	}
}

func newRingbufReader(objs *Objects) (*ringbuf.Reader, error) {
	return ringbuf.NewReader(objs.MemleakRingbufEvents)
}

func (t *Tracer) Run(ctx context.Context) error {
	rd, err := newRingbufReader(t.ebpfObjs)
	if err != nil {
		return err
	}
	defer rd.Close()
	go func() {
		<-ctx.Done()
		logging.L().Debug().Msg("SIGTERM received, closing ringbuf")
		t.ebpfObjs.MemleakRingbufEvents.Close()
	}()
	events := make(chan []byte, 1024)
	go func() {
		defer close(events)

		for {
			record, err := rd.Read()
			if err != nil {
				if errors.Is(err, ringbuf.ErrClosed) {
					return
				}
				continue
			}

			select {
			case events <- record.RawSample:
			case <-ctx.Done():
				return
			}
		}
	}()
	aggregator := NewLeakAggregator()
	go StartSnapshotPrinter(ctx, aggregator, time.Duration(internal.FetchInterval)*time.Second)
	for {
		select {
		case <-ctx.Done():
			return nil
		case raw, ok := <-events:
			if !ok {
				return nil
			}
			e, err := t.decodeMemoryEvent(raw)
			if err != nil {
				logging.L().Error().Err(err).Msg("decode failed")
			}
			aggregator.Consume(e)
		}
	}
}

type LeakKey struct {
	PID         uint32
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
	mu       sync.Mutex
	state    map[LeakKey]*MemoryLeakResult
	pidCache *pid.PIDCache
}

func NewLeakAggregator() *LeakAggregator {
	return &LeakAggregator{
		state:    make(map[LeakKey]*MemoryLeakResult),
		pidCache: pid.GlobalPIDCache(),
	}
}

func (a *LeakAggregator) Consume(
	ev event.MemoryEvent,
) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for _, gpu := range ev.Process.GPUs {
		key := LeakKey{
			PID:         ev.Process.Process.PID,
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

	live := make(map[uint32]pid.PIDInspection)
	for _, p := range a.pidCache.List() {
		live[p.Process.PID] = p
	}

	for k := range a.state {
		if _, ok := live[k.PID]; !ok {
			delete(a.state, k)
		}
	}
	for _, proc := range live {
		for _, gpu := range proc.GPUs {
			key := LeakKey{
				PID:         proc.Process.PID,
				DeviceIndex: gpu.DeviceIndex,
				UUID:        gpu.UUID,
			}

			if _, ok := a.state[key]; !ok {
				a.state[key] = &MemoryLeakResult{
					Key:     key,
					Process: proc.Process,
				}
			}
		}
	}

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
		"INUSE_MB",
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
