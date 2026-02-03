package lifecycle

import (
	"fmt"
	"log"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/vuvietnguyenit/gpuxray/internal/lifecycle/gen"
)

type ProcExitObjects struct {
	*gen.ProcessExitObjects
	Tp link.Link
}

type CuInitObjects struct {
	*gen.CudaObjects
	Tp link.Link
}

func LoadProcExitObjects(cfg Config) (*ProcExitObjects, error) {
	var objs gen.ProcessExitObjects
	if err := gen.LoadProcessExitObjects(&objs, nil); err != nil {
		return nil, err
	}
	return &ProcExitObjects{&objs, nil}, nil
}

func LoadCuInitObjects(cfg Config) (*CuInitObjects, error) {
	var objs gen.CudaObjects
	if err := gen.LoadCudaObjects(&objs, nil); err != nil {
		return nil, err
	}
	return &CuInitObjects{&objs, nil}, nil
}

func (o *ProcExitObjects) Close() error {
	if o.Tp != nil {
		o.Tp.Close()
	}
	return nil
}

func (o *CuInitObjects) Close() error {
	if o.Tp != nil {
		o.Tp.Close()
	}
	return nil
}

func (o *ProcExitObjects) Attach(cfg Config) error {
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

func (o *CuInitObjects) Attach(cudaLib string, objs *CuInitObjects) ([]link.Link, error) {
	if cudaLib == "" {
		return nil, fmt.Errorf("cuda library path is empty")
	}

	ex, err := link.OpenExecutable(cudaLib)
	if err != nil {
		return nil, fmt.Errorf("open executable %s: %w", cudaLib, err)
	}

	var links []link.Link

	attach := func(
		symbols []string,
		prog *ebpf.Program,
		ret bool,
	) {
		for _, sym := range symbols {
			var (
				l   link.Link
				err error
			)

			if ret {
				l, err = ex.Uretprobe(sym, prog, nil)
			} else {
				l, err = ex.Uprobe(sym, prog, nil)
			}

			if err != nil {
				log.Printf("failed to attach %sprobe to %s: %v",
					map[bool]string{true: "uret", false: "u"}[ret],
					sym,
					err,
				)
				continue
			}

			links = append(links, l)
			log.Printf("✓ attached %sprobe to %s",
				map[bool]string{true: "uret", false: "u"}[ret],
				sym,
			)
		}
	}

	attach(
		[]string{"cuInit"},
		objs.UprobeCuinit,
		false,
	)

	return links, nil
}
