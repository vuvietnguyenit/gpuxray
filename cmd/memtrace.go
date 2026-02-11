package cmd

import (
	"fmt"
	"sync"

	"github.com/spf13/cobra"
	"github.com/vuvietnguyenit/gpuxray/internal/lifecycle"
	"github.com/vuvietnguyenit/gpuxray/internal/logging"
	"github.com/vuvietnguyenit/gpuxray/internal/memtrace"
	"github.com/vuvietnguyenit/gpuxray/internal/pid"
)

var (
	Pid      uint32
	DeviceID int
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "memtrace",
		Short: "Trace GPU memory activity",
		Long:  "Trace CUDA GPU memory allocations and frees using eBPF",
		RunE:  runMemtrace,
	}

	cmd.Flags().Uint32VarP(&Pid, "pid", "p", 0, "Trace specific PID (0 = all)")
	cmd.Flags().IntVar(&DeviceID, "device", -1, "GPU device ID (-1 = all)")

	return cmd
}

func init() {
	rootCmd.AddCommand(NewCmd())
}

var wg sync.WaitGroup

func runMemtrace(cmd *cobra.Command, _ []string) error {
	// get context from root
	ctx := cmd.Context()
	// DEBUG purpose
	go func() {
		<-rootCtx.Done()
		logging.L().Debug().
			Err(ctx.Err()).
			Msg("runMemtrace context canceled")

	}()
	//
	cfg := memtrace.Config{
		PID:      Pid,
		DeviceID: DeviceID,
	}
	objs, err := memtrace.LoadObjects(cfg)
	if err != nil {
		return err
	}
	defer objs.Close()
	var pids pid.ListPIDInspection
	if Pid == 0 {
		return fmt.Errorf("pid %d doesn't exist", Pid)
	}
	process := pid.InspectPID(Pid)
	pids = append(pids, *process)

	if len(pids) != 0 {
		pids.CachePID()
	}
	syms := pids.EnumerateSymNames("*")
	for _, libPath := range pid.GlobalPIDCache().GetCUDASharedObjectPaths() {
		logging.L().Debug().
			Str("cuda_lib", libPath).
			Msg("attaching probes to CUDA library")

		links := memtrace.AttachProbes(libPath, objs, syms)
		defer func() {
			for _, l := range links {
				l.Close()
			}
		}()
	}
	wg.Go(func() {
		lifecycle.TraceProcessExit(ctx)
	})
	rd, err := memtrace.NewRingbufReader(objs)
	if err != nil {
		return err
	}
	defer rd.Close()
	err = memtrace.Run(ctx, rd, cfg)
	if err != nil {
		return err
	}
	wg.Wait()
	return nil
}
