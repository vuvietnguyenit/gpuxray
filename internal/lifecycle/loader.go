package lifecycle

import (
	"fmt"

	"github.com/cilium/ebpf/link"
	"github.com/vuvietnguyenit/gpuxray/internal/lifecycle/gen"
)

type Objects struct {
	*gen.ProcessExitObjects
	*gen.CudaObjects
	Tp link.Link
}

func LoadObjects(cfg Config) (*Objects, error) {
	var objs gen.ProcessExitObjects
	if err := gen.LoadProcessExitObjects(&objs, nil); err != nil {
		return nil, err
	}
	var cudaObjs gen.CudaObjects
	if err := gen.LoadCudaObjects(&cudaObjs, nil); err != nil {
		return nil, err
	}
	return &Objects{&objs, &cudaObjs, nil}, nil
}

func (o *Objects) Close() error {
	if o.Tp != nil {
		o.Tp.Close()
	}
	err := o.CudaObjects.Close()
	if err != nil {
		return err
	}
	err = o.ProcessExitObjects.Close()
	if err != nil {
		return err
	}
	return nil
}

func (o *Objects) AttachTp(cfg Config) error {
	tp, err := link.Tracepoint(
		"sched",
		"sched_process_exit",
		o.HandleProcessExit,
		nil,
	)
	if err != nil {
		return fmt.Errorf("attach tracepoint: %w", err)
	}

	o.Tp = tp
	return nil
}
