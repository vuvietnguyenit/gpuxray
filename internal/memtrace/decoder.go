// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

package memtrace

import (
	"bytes"
	"encoding/binary"

	"github.com/vuvietnguyenit/gpuxray/internal/event"
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

func decodeMemoryEvent(data []byte) (*bpfMemEvent, error) {
	var raw bpfMemEvent
	if err := binary.Read(bytes.NewReader(data), binary.LittleEndian, &raw); err != nil {
		return nil, err
	}
	return &raw, nil
}
