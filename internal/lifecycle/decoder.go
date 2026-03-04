// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

package lifecycle

import (
	"bytes"
	"encoding/binary"

	"github.com/vuvietnguyenit/gpuxray/internal/event"
)

// ProcessExit is a decoded, userspace-friendly event
type bpfProcessExit struct {
	PID    uint32
	TGID   uint32
	ExitAt uint64
}

func decodeProcessExitEvent(data []byte) (event.LifecycleProcessExitEvent, error) {
	var raw bpfProcessExit
	if err := binary.Read(bytes.NewReader(data), binary.LittleEndian, &raw); err != nil {
		return event.LifecycleProcessExitEvent{}, err
	}
	return event.LifecycleProcessExitEvent{
		Pid:    raw.PID,
		Tgid:   raw.TGID,
		ExitTs: raw.ExitAt,
	}, nil
}
