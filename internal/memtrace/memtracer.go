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

type gpuMemoryTracer struct {
	pidCache *pid.PIDCache
	ebpfObjs *Objects
	cfg      Config
}
type gpuMemoryEvent struct {
	TS      time.Time
	Process pid.PIDInspection
	TID     int
	Bytes   uint64
	Ptr     uint64
	StackID int32
	Op      event.MemoryEventType
}

func NewGPUMemoryTracer(o *Objects, cfg Config) *gpuMemoryTracer {
	return &gpuMemoryTracer{
		pidCache: pid.GlobalPIDCache(),
		ebpfObjs: o,
		cfg:      cfg,
	}
}

func newRingbufReader(objs *Objects) (*ringbuf.Reader, error) {
	return ringbuf.NewReader(objs.MemleakRingbufEvents)
}

func (t *gpuMemoryTracer) Run(ctx context.Context) error {
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
	if internal.MemoryleakFlags.PrintStack {
		go t.stackPrinter(ctx, aggregator, time.Duration(internal.FetchInterval)*time.Second)
	} else {
		go t.statsPrinter(ctx, aggregator, time.Duration(internal.FetchInterval)*time.Second)
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case raw, ok := <-events:
			if !ok {
				return nil
			}
			bpfEvent, err := decodeMemoryEvent(raw)
			e := gpuMemoryEvent{
				TS: time.Unix(0, int64(bpfEvent.Timestamp)),
				Process: pid.GlobalPIDCache().GetOrInspect(bpfEvent.Pid,
					func(p uint32) (pid.PIDInspection, error) {
						return *pid.InspectPID(p), nil
					}),
				TID:     int(bpfEvent.Tid),
				Bytes:   bpfEvent.Size,
				Ptr:     bpfEvent.DevicePtr,
				StackID: bpfEvent.StackID,
				Op:      event.MemoryEventType(bpfEvent.Type),
			}
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
	StackID    int32
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
	ev gpuMemoryEvent,
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
			entry.StackID = ev.StackID

		case event.MemFree:
			if ev.Bytes == 0 {
				continue
			}
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
			StackID:    v.StackID,
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

func (t *gpuMemoryTracer) stackPrinter(
	ctx context.Context,
	agg *LeakAggregator,
	interval time.Duration,
) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	s := symbolizer.New()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("stackPrinter stopped")
			return

		case <-ticker.C:
			ts := time.Now()
			results := agg.Snapshot()
			fmt.Println(ts.Format(time.RFC3339))
			if len(results) == 0 {
				fmt.Println("No leaks detected")
				continue
			}

			for i, r := range results {
				if r.StackID < 0 {
					continue
				}

				leaked := uint64(0)
				if r.AllocBytes > r.FreeBytes {
					leaked = r.AllocBytes - r.FreeBytes
				}
				remain := uint64(0)
				if r.AllocCount > r.FreeCount {
					remain = r.AllocCount - r.FreeCount
				}

				fmt.Printf("[%d] PID: %-6d   GPU: %-3d StackID: %-6d  Remaining Blocks: %-6d  TotalBytes: %-10s\n",
					i+1,
					r.Key.PID,
					r.Key.DeviceIndex,
					r.StackID,
					remain,
					internal.HumanBytes(leaked),
				)
				stack, err := t.ebpfObjs.getStack(r.StackID)
				if err != nil {
					fmt.Printf("Failed to get stack: %v\n", err)
					continue
				}

				for frameIdx, addr := range stack {
					sym, err := s.Resolve(int(r.Key.PID), addr)
					if err != nil {
						fmt.Printf("  #%02d  0x%-16x  [unresolved]\n", frameIdx, addr)
						continue
					}

					fmt.Printf("  #%02d  0x%-16x  %s\n",
						frameIdx,
						addr,
						sym,
					)
				}
			}
			fmt.Println()
		}
	}
}

func (t *gpuMemoryTracer) statsPrinter(ctx context.Context, agg *LeakAggregator, interval time.Duration,
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
