package lifecycle

import (
	"context"
	"errors"

	"github.com/cilium/ebpf/ringbuf"
	"github.com/vuvietnguyenit/gpuxray/internal/logging"
	"github.com/vuvietnguyenit/gpuxray/internal/pid"
)

type ProcExitTracer struct {
	procExitRd *ringbuf.Reader
	pidCache   *pid.PIDCache
}

type CuInitTracer struct {
	cuInitRd *ringbuf.Reader
	pidCache *pid.PIDCache
}

func NewProcExitTracer(procExitRd *ringbuf.Reader) *ProcExitTracer {
	return &ProcExitTracer{procExitRd: procExitRd, pidCache: pid.GlobalPIDCache()}
}

func NewCuInitTracer(cuInitRd *ringbuf.Reader) *CuInitTracer {
	return &CuInitTracer{cuInitRd: cuInitRd, pidCache: pid.GlobalPIDCache()}
}

func NewRingbufReader(objs *ProcExitObjects) (*ringbuf.Reader, error) {
	return ringbuf.NewReader(objs.LifecycleEvents)
}

func NewCuInitRingbufReader(objs *CuInitObjects) (*ringbuf.Reader, error) {
	return ringbuf.NewReader(objs.CuInitEvents)
}

func (r *ProcExitTracer) Run(ctx context.Context) error {
	defer r.procExitRd.Close()

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		record, err := r.procExitRd.Read()
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
		r.pidCache.Delete(ev.Pid)
	}
}

func (r *CuInitTracer) Run(ctx context.Context) error {
	defer r.cuInitRd.Close()

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		record, err := r.cuInitRd.Read()
		if err != nil {
			if errors.Is(err, ringbuf.ErrClosed) {
				return nil
			}
			continue
		}
		ev, err := decodeCuInitEvent(record.RawSample)
		if err != nil {
			continue
		}
		logging.L().Debug().
			Int("pid", int(ev.PID)).
			Msg("cuInit called")

	}
}
