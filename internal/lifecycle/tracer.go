package lifecycle

import (
	"context"
	"errors"
	"fmt"

	"github.com/cilium/ebpf/ringbuf"
	"github.com/vuvietnguyenit/gpuxray/internal/pid"
)

type Tracer struct {
	rd       *ringbuf.Reader
	pidCache *pid.PIDCache
}

func NewTracer(rd *ringbuf.Reader) *Tracer {
	return &Tracer{rd: rd, pidCache: pid.Global()}
}

func NewRingbufReader(objs *Objects) (*ringbuf.Reader, error) {
	return ringbuf.NewReader(objs.LifecycleEvents)
}

func (r *Tracer) Run(ctx context.Context) error {
	defer r.rd.Close()

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		record, err := r.rd.Read()
		if err != nil {
			if errors.Is(err, ringbuf.ErrClosed) {
				return nil
			}
			continue
		}
		ev, err := decodeProcessExitEvent(record.RawSample)
		if err != nil {
			continue
		}
		fmt.Println(ev)
		// pidCache.Remove(int(ev.Pid))
	}
}
