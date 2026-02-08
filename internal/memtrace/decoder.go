// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

package memtrace

import (
	"bytes"
	"encoding/binary"
	"time"

	"github.com/vuvietnguyenit/gpuxray/internal/event"
	"github.com/vuvietnguyenit/gpuxray/internal/pid"
)

// match C bpf struct
type bpfMemEvent struct {
	Pid       uint32
	Tid       uint32
	Timestamp uint64
	Size      uint64
	DevicePtr uint64
	Type      event.MemoryEventType
}

func decodeMemoryEvent(data []byte) (event.MemoryEvent, error) {
	var raw bpfMemEvent
	if err := binary.Read(bytes.NewReader(data), binary.LittleEndian, &raw); err != nil {
		return event.MemoryEvent{}, err
	}

	return event.MemoryEvent{
		TS: time.Unix(0, int64(raw.Timestamp)),
		Process: pid.GlobalPIDCache().GetOrInspect(raw.Pid,
			// create inspector to inspect PID in slow part
			func(p uint32) (pid.PIDInspection, error) {
				return pid.InspectPID(int32(p)), nil
			}),
		TID:   int(raw.Tid),
		Bytes: raw.Size,
		Ptr:   raw.DevicePtr,
		Op:    event.MemoryEventType(raw.Type),
	}, nil
}
