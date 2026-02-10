// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

package event

type Type uint8

const (
	EventUnknown Type = iota
	EventMemory
	EventCompute
	EventPower
)
