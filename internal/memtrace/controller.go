// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

package memtrace

import (
	"context"
)

type Config struct {
	PID      uint32
	DeviceID int
}

func Run(ctx context.Context, objs *Objects, cfg Config) error {
	tracer := NewTracer(objs)
	return tracer.Run(ctx)
}
