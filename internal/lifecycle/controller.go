package lifecycle

import (
	"context"

	"github.com/cilium/ebpf/ringbuf"
)

type Config struct {
}

func Run(ctx context.Context, rd *ringbuf.Reader, cfg Config) error {
	tracer := NewTracer(rd)
	return tracer.Run(ctx)
}
