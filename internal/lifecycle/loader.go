package lifecycle

import (
	"fmt"

	"github.com/cilium/ebpf/link"
	"github.com/vuvietnguyenit/gpuxray/internal/lifecycle/gen"
)

type Objects struct {
	*gen.ProcessExitObjects
	Tp link.Link
}

func LoadObjects(cfg Config) (*Objects, error) {
	var objs gen.ProcessExitObjects
	if err := gen.LoadProcessExitObjects(&objs, nil); err != nil {
		return nil, err
	}
	return &Objects{&objs, nil}, nil
}

func (o *Objects) Close() error {
	if o.Tp != nil {
		o.Tp.Close()
	}
	return o.ProcessExitObjects.Close()
}

func (o *Objects) Attach(cfg Config) error {
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
