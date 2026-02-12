// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

package memtrace

import (
	"context"
	"errors"
	"fmt"
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
