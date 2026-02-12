// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

package memtrace

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/vuvietnguyenit/gpuxray/internal/event"
	"github.com/vuvietnguyenit/gpuxray/internal/logging"
	"github.com/vuvietnguyenit/gpuxray/internal/pid"
)

// match C bpf struct
type bpfMemEvent struct {
	Pid     uint32
	Tid     uint32
	Type    event.MemoryEventType
	StackID int32

	Timestamp uint64
	Size      uint64
	DevicePtr uint64
}

func (t *Tracer) getStack(stackID int32) ([]uint64, error) {
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

func (t *Tracer) decodeMemoryEvent(data []byte) (event.MemoryEvent, error) {
	var raw bpfMemEvent
	if err := binary.Read(bytes.NewReader(data), binary.LittleEndian, &raw); err != nil {
		return event.MemoryEvent{}, err
	}
	stackID, err := t.getStack(raw.StackID)
	if err != nil {
		logging.L().Err(err).Msg("unwind stackID")
	}
	for _, s := range stackID {
		name, err := t.symbolizer.Resolve(int(raw.Pid), s)
		if err != nil {
			logging.L().Debug().Msgf("symbolizer err: %s", err)
			continue
		}
		fmt.Println("Symbol:", name)
	}

	return event.MemoryEvent{
		TS: time.Unix(0, int64(raw.Timestamp)),
		Process: pid.GlobalPIDCache().GetOrInspect(raw.Pid,
			// create inspector to inspect PID in slow part
			func(p uint32) (pid.PIDInspection, error) {
				return *pid.InspectPID(p), nil
			}),
		TID:     int(raw.Tid),
		Bytes:   raw.Size,
		Ptr:     raw.DevicePtr,
		Op:      event.MemoryEventType(raw.Type),
		StackID: stackID,
	}, nil
}
