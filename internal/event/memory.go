// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

package event

import "time"

type MemoryEventType uint8

type Event interface {
	Type() Type
	Timestamp() time.Time
	DeviceID() int
}

const (
	MemUnknown MemoryEventType = iota
	MemAlloc
	MemFree
	MemMemcpy
	MemMemset
)

type MemoryEvent struct {
	TS     time.Time
	Device int
	PID    int
	TID    int
	Bytes  uint64
	Ptr    uint64
	Op     MemoryEventType
}

func (e MemoryEvent) Type() Type           { return EventMemory }
func (e MemoryEvent) Timestamp() time.Time { return e.TS }
func (e MemoryEvent) DeviceID() int        { return e.Device }
