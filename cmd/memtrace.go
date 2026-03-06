// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

package cmd

import (
	"fmt"
	"sync"

	"github.com/spf13/cobra"
	"github.com/vuvietnguyenit/gpuxray/internal"
	"github.com/vuvietnguyenit/gpuxray/internal/logging"
	"github.com/vuvietnguyenit/gpuxray/internal/memtrace"
	"github.com/vuvietnguyenit/gpuxray/internal/pid"
)

func newMemtraceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "memtrace",
		Short: "Trace GPU memory activity",
		Long: `Display GPU memory stat base on eBPF.
This command inspects GPU memory stats It helps identify GPU memory leaks.
Columns:

  GPU
        GPU index where the allocation occurred.

  PID
        Process ID that owns the GPU memory allocation.

  INUSE_MB
        Total GPU memory currently allocated.

  AL_CNT
        Total number of allocation events recorded for this stack trace.

  FR_CNT
        Total GPU memory currently allocated by this stack trace.

  COMM
        Command name of the process that owns the GPU memory allocation.

When --print-stacks is enabled, the output includes additional columns for stack traces:

Example:

./gpuxray memtrace -p 332361 -i 1

TIME       PID      GPU  INUSE_MB     AL_CNT   FR_CNT   COMM            
15:58:36   332361   0    512 B        38       37       python3         
15:58:37   332361   0    1.00 KiB     80       78       python3 

./gpuxray memtrace -p 332361 -i 1 --print-stacks

2026-03-04T15:59:44+07:00
[1] PID: 332361   GPU: 0   StackID: 1908    Remaining Blocks: 1       TotalBytes: 512 B     
  #00  0x71263447d86e      libcudart_static_5382377d5c772c9d197c0cda9fd9742ee6ad893c
  #01  0x7126344491c3      libcudart_static_f74e2f2bcf2cf49bd1a61332e1d15bd1e748f9cf
  #02  0x71263448d993      cudaMalloc
  #03  0x712634420cde      __pyx_f_13cupy_backends_4cuda_3api_7runtime_malloc(unsigned long, int)
		`,
		RunE: runMemtrace,
	}

	cmd.Flags().Uint32VarP(&internal.MemoryleakFlags.Pid, "pid", "p", 0, "Trace specific PID (0 = all)")
	cmd.Flags().IntVar(&internal.MemoryleakFlags.DeviceID, "device", -1, "GPU device ID (-1 = all)")
	cmd.Flags().BoolVar(&internal.MemoryleakFlags.PrintStack, "print-stacks", false, "Print stack traces")

	return cmd
}

func init() {
	rootCmd.AddCommand(newMemtraceCmd())
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
		PID:      internal.MemoryleakFlags.Pid,
		DeviceID: internal.MemoryleakFlags.DeviceID,
	}
	objs, err := memtrace.LoadObjects(cfg)
	if err != nil {
		return err
	}
	defer objs.Close()
	var pids pid.ListPIDInspection
	if internal.MemoryleakFlags.Pid == 0 {
		return fmt.Errorf("pid %d doesn't exist", internal.MemoryleakFlags.Pid)
	}
	process := pid.InspectPID(internal.MemoryleakFlags.Pid)
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
	err = memtrace.Run(ctx, objs, cfg)
	if err != nil {
		return err
	}
	wg.Wait()
	return nil
}
