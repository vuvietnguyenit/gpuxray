// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

package cmd

import (
	"fmt"
	"sync"

	"github.com/spf13/cobra"
	"github.com/vuvietnguyenit/gpuxray/internal"
	"github.com/vuvietnguyenit/gpuxray/internal/lifecycle"
	"github.com/vuvietnguyenit/gpuxray/internal/logging"
	"github.com/vuvietnguyenit/gpuxray/internal/memtrace"
	"github.com/vuvietnguyenit/gpuxray/internal/pid"
)

func newMemtraceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "memtrace",
		Short: "Trace GPU memory activity",
		Long:  "Trace CUDA GPU memory allocations and frees using eBPF",
		RunE:  runMemtrace,
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
	wg.Go(func() {
		lifecycle.TraceProcessExit(ctx)
	})
	err = memtrace.Run(ctx, objs, cfg)
	if err != nil {
		return err
	}
	wg.Wait()
	return nil
}
