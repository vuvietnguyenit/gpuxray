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

func runGPUMemoryTracer(ctx context.Context, objs *Objects, cfg Config) error {
	tracer := NewGPUMemoryTracer(objs, cfg)
	return tracer.Run(ctx)
}

func Run(ctx context.Context, objs *Objects, cfg Config) error {
	return runGPUMemoryTracer(ctx, objs, cfg)
}
