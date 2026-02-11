package lifecycle

import (
	"context"
	"errors"
	"log"
	"os"

	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	"github.com/cilium/ebpf/ringbuf"
	"github.com/vuvietnguyenit/gpuxray/internal/lifecycle/gen"
	"github.com/vuvietnguyenit/gpuxray/internal/logging"
	"github.com/vuvietnguyenit/gpuxray/internal/pid"
)

func TraceProcessExit(parent context.Context) {
	objs := gen.ProcessExitObjects{}
	if err := gen.LoadProcessExitObjects(&objs, nil); err != nil {
		logging.L().Err(err).Msg("load sched_process_exit")
		os.Exit(1)
	}
	defer objs.Close()
	tp, err := link.Tracepoint(
		"sched",
		"sched_process_exit",
		objs.HandleProcessExit,
		nil,
	)
	if err != nil {
		logging.L().Err(err).Msg("opening tracepoint")
		os.Exit(1)

	}
	defer tp.Close()

	rd, err := ringbuf.NewReader(objs.LifecycleEvents)
	if err != nil {
		logging.L().Err(err).Msg("opening ringbuf reader")
		os.Exit(1)
	}
	defer rd.Close()

	go func() {
		<-parent.Done()
		logging.L().Debug().Msg("TraceProcessExit received exit signal...")
		if err := rd.Close(); err != nil {
			logging.L().Err(err).Msg("closing ringbuf reader")
			return
		}
	}()
	for {
		record, err := rd.Read()
		if err != nil {
			if errors.Is(err, perf.ErrClosed) {
				log.Println("Received signal, exiting..")
				return
			}
			log.Printf("reading from reader: %s", err)
			continue
		}
		ev, err := decodeProcessExitEvent(record.RawSample)
		if err != nil {
			continue
		}
		logging.L().Debug().Uint32("pid", ev.Pid).Msg("pid is exited")
		pid.GlobalPIDCache().Delete(ev.Pid)
	}

}
