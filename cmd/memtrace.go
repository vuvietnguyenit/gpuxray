package cmd

import (
	"log"
	"os"
	"sync"

	"github.com/spf13/cobra"
	"github.com/vuvietnguyenit/gpuxray/internal"
	"github.com/vuvietnguyenit/gpuxray/internal/lifecycle"
	"github.com/vuvietnguyenit/gpuxray/internal/logging"
	"github.com/vuvietnguyenit/gpuxray/internal/memtrace"
	"github.com/vuvietnguyenit/gpuxray/internal/pid"
	"github.com/vuvietnguyenit/gpuxray/internal/so"
)

var (
	Pid      int
	DeviceID int
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "memtrace",
		Short: "Trace GPU memory activity",
		Long:  "Trace CUDA GPU memory allocations and frees using eBPF",
		RunE:  runMemtrace,
	}

	cmd.Flags().IntVar(&Pid, "pid", 0, "Trace specific PID (0 = all)")
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
	if Pid != 0 {
		process := pid.InspectPID(int32(Pid))
		pids = append(pids, process)
	} else {
		pids, err = pid.GetRunningProcesses()
		if err != nil {
			logging.L().Err(err).Msg("Failed to get running GPU processes")
			os.Exit(1)
		}
	}
	if len(pids) != 0 {
		pids.CachePID()
	}
	err = so.InitFromSharedObject(internal.CudaSo)
	if err != nil {
		log.Fatal(err)
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
	wg.Go(func() {
		lifecycle.TraceCuInitCall(ctx)
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
