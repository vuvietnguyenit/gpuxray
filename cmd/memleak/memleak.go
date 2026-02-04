// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

package memleak

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/vuvietnguyenit/gpuxray/internal/logging"
	"github.com/vuvietnguyenit/gpuxray/internal/memtrace"
	"github.com/vuvietnguyenit/gpuxray/internal/pid"
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

func runMemtrace(cmd *cobra.Command, _ []string) error {
	ctx, cancel := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer cancel()
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
	pids.CachePID()
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

	rd, err := memtrace.NewRingbufReader(objs)
	if err != nil {
		return err
	}
	defer rd.Close()
	return memtrace.Run(ctx, rd, cfg)
}
