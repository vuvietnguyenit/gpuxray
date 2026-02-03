package lifecycle

import (
	"context"

	"github.com/cilium/ebpf/ringbuf"
)

type Config struct {
}

func RunProcExitRd(ctx context.Context, rd *ringbuf.Reader, cfg Config) error {
	tracer := NewProcExitTracer(rd)
	return tracer.Run(ctx)
}

func RunCuInitRd(ctx context.Context, rd *ringbuf.Reader, cfg Config) error {
	tracer := NewCuInitTracer(rd)
	return tracer.Run(ctx)
}
