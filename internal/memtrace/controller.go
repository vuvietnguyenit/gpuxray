// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

package memtrace

import (
	"context"

	"github.com/cilium/ebpf/ringbuf"
	"github.com/vuvietnguyenit/gpuxray/internal/pid"
)

type Config struct {
	PID      int
	DeviceID int
}

func Run(ctx context.Context, rd *ringbuf.Reader, cfg Config, cache *pid.PIDCache) error {
	tracer := NewTracer(rd)
	return tracer.Run(ctx)
}
