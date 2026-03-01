// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

package memtrace

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/cilium/ebpf/ringbuf"
	"github.com/vuvietnguyenit/gpuxray/internal/event"
	"github.com/vuvietnguyenit/gpuxray/internal/logging"
	"github.com/vuvietnguyenit/gpuxray/internal/pid"
	"github.com/vuvietnguyenit/gpuxray/internal/symbolizer"
)

type gpuStackTrace struct {
	pidCache   *pid.PIDCache
	ebpfObjs   *Objects
	symbolizer *symbolizer.Symbolizer
	cfg        Config
}

type gpuStackTraceEvt struct {
	TS      time.Time
	Process pid.PIDInspection
	TID     int
	Bytes   uint64
	Ptr     uint64
	Op      event.MemoryEventType
	Stacks  []uint64
}

type Allocation struct {
	Size     uint64 // bytes
	GPU      int
	Function string // or stack symbolized string
}

type LeakStat struct {
	Function string
	GPU      int
	Bytes    uint64
	Percent  float64
}

func BuildLeakDistribution(active map[uint64]Allocation) []LeakStat {
	type groupKey struct {
		Function string
		GPU      int
	}
	group := make(map[groupKey]*LeakStat)
	var total uint64

	for _, alloc := range active {
		total += alloc.Size

		key := groupKey{
			Function: alloc.Function,
			GPU:      alloc.GPU,
		}

		stat, exists := group[key]
		if !exists {
			stat = &LeakStat{
				Function: alloc.Function,
				GPU:      alloc.GPU,
			}
			group[key] = stat
		}

		stat.Bytes += alloc.Size
	}

	if total == 0 {
		return nil
	}

	var result []LeakStat
	for _, stat := range group {
		stat.Percent = float64(stat.Bytes) / float64(total) * 100
		result = append(result, *stat)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Bytes == result[j].Bytes {
			if result[i].GPU == result[j].GPU {
				return result[i].Function < result[j].Function
			}
			return result[i].GPU < result[j].GPU
		}
		return result[i].Bytes > result[j].Bytes
	})

	return result
}

func NewGPUStackTrace(o *Objects, cfg Config) *gpuStackTrace {
	return &gpuStackTrace{
		pidCache:   pid.GlobalPIDCache(),
		ebpfObjs:   o,
		symbolizer: symbolizer.New(),
		cfg:        cfg,
	}
}

func (t *gpuStackTrace) getStack(stackID int32) ([]uint64, error) {
	if stackID < 0 {
		return nil, fmt.Errorf("invalid stack id %d", stackID)
	}
	var stack []uint64 = make([]uint64, 127)

	err := t.ebpfObjs.StackTraces.Lookup(stackID, &stack)
	if err != nil {
		return nil, err
	}

	// Trim trailing zeroes
	var result []uint64
	for _, addr := range stack {
		if addr == 0 {
			break
		}
		result = append(result, addr)
	}
	logging.L().Debug().Any("stack", result).Msg("result")

	return result, nil
}

func PrintLeakDistribution(stats []LeakStat) {
	fmt.Println("Leak Distribution:")
	fmt.Println("───────────────────────────────────────────────────────────────")
	fmt.Printf("%-7s %-6s %-5s %s\n", "%LEAK", "MB", "GPU", "FUNCTION")

	for _, stat := range stats {
		mb := float64(stat.Bytes) / (1024 * 1024)
		fmt.Printf("%6.1f%% %-6.0f %-5d %s\n",
			stat.Percent,
			mb,
			stat.GPU,
			stat.Function,
		)
	}
}

func (t *gpuStackTrace) Run(ctx context.Context) error {
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
	for {
		select {
		case <-ctx.Done():
			return nil
		case raw, ok := <-events:
			if !ok {
				return nil
			}
			bpfEvent, err := decodeMemoryEvent(raw)
			if err != nil {
				logging.L().Warn().Err(err).Msg("decode failed")
			}
			s, err := t.getStack(bpfEvent.StackID)
			if err != nil {
				logging.L().Warn().Msgf("get stack ID error %s", err)

			}
			e := gpuStackTraceEvt{
				TS: time.Unix(0, int64(bpfEvent.Timestamp)),
				Process: pid.GlobalPIDCache().GetOrInspect(bpfEvent.Pid,
					func(p uint32) (pid.PIDInspection, error) {
						return *pid.InspectPID(p), nil
					}),
				TID:    int(bpfEvent.Tid),
				Bytes:  bpfEvent.Size,
				Ptr:    bpfEvent.DevicePtr,
				Op:     event.MemoryEventType(bpfEvent.Type),
				Stacks: s,
			}
			fmt.Println(e)
		}
	}
}
