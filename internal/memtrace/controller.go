// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

package memtrace

import (
	"context"

	"github.com/cilium/ebpf/ringbuf"
)

type Config struct {
	PID      uint32
	DeviceID int
}

func Run(ctx context.Context, rd *ringbuf.Reader, cfg Config) error {
	tracer := NewTracer(rd)
	return tracer.Run(ctx)
}
