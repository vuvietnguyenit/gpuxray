// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

package event

import (
	"time"
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
