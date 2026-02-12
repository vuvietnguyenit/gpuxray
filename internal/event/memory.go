// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

package event

import (
	"time"

	"github.com/vuvietnguyenit/gpuxray/internal/pid"
)

type MemoryEventType uint32

type Event interface {
	Type() Type
	Timestamp() time.Time
	DeviceID() int
}

const (
	MemAlloc MemoryEventType = iota
	MemFree
	MemMemcpy
	MemMemset
)

type MemoryEvent struct {
	TS      time.Time
	Process pid.PIDInspection
	TID     int
	Bytes   uint64
	Ptr     uint64
	Op      MemoryEventType
	StackID []uint64
}
