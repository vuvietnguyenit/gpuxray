package lifecycle

import (
	"bytes"
	"encoding/binary"
	"fmt"

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

func decodeCuInitEvent(data []byte) (*event.CuInitEvent, error) {
	if len(data) < 24 {
		return nil, fmt.Errorf("invalid cuinit event size: %d", len(data))
	}

	var ev event.CuInitEvent
	err := binary.Read(bytes.NewReader(data), binary.LittleEndian, &ev)
	if err != nil {
		return nil, err
	}

	return &ev, nil
}
