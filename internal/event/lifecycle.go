// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

package event

type LifecycleProcessExitEvent struct {
	Pid    uint32
	Tgid   uint32
	ExitTs uint64
}

type CuInitEvent struct {
	PID  uint32
	TGID uint32
	Comm [16]byte
}
