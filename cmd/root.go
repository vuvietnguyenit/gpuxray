// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/cilium/ebpf/rlimit"
	"github.com/spf13/cobra"
	"github.com/vuvietnguyenit/gpuxray/cmd/memleak"
	"github.com/vuvietnguyenit/gpuxray/internal"
	"github.com/vuvietnguyenit/gpuxray/internal/lifecycle"
	"github.com/vuvietnguyenit/gpuxray/internal/pid"
	"github.com/vuvietnguyenit/gpuxray/internal/so"
)

func removeMemlock() error {
	if !internal.RemoveMemlock {
		return nil
	}
	// Allow the current process to lock memory for eBPF resources.
	if err := rlimit.RemoveMemlock(); err != nil {
		return fmt.Errorf("failed to remove memlock limit: %w", err)
	}
	log.Println("Removed memlock limit successfully")
	return nil
}

var rootCmd = &cobra.Command{
	Use:   "gpuxray",
	Short: "eBPF tool this help tracing and investigating GPU",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		removeMemlock()
		ret := nvml.Init()
		if ret != nvml.SUCCESS {
			log.Fatalf("Unable to initialize NVML: %v", nvml.ErrorString(ret))
			os.Exit(1)
		}
		log.Println("Initialized NVML")
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		ret := nvml.Shutdown()
		if ret != nvml.SUCCESS {
			log.Fatalf("Unable to shutdown NVML: %v", nvml.ErrorString(ret))
			os.Exit(1)
		}
		log.Println("Shutdown NVML")
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&internal.RemoveMemlock, "remove-memlock", true, "Remove RLIMIT_MEMLOCK before loading eBPF programs")
	rootCmd.PersistentFlags().IntVarP(&internal.FetchInterval, "fetch-interval", "i", 5, "Interval in seconds to fetch GPU tracing console")
	rootCmd.PersistentFlags().StringVarP(&internal.CudaSo, "cuda-so", "c", "/usr/lib/x86_64-linux-gnu/libcuda.so", "Path to CUDA shared object")
	rootCmd.AddCommand(memleak.NewCmd())
}

func startLifecycle(ctx context.Context) (func(), error) {
	fmt.Println("Starting lifecycle tracer...")
	cfg := lifecycle.Config{} // if you have any config, set it here
	loader, err := lifecycle.LoadProcExitObjects(cfg)
	if err != nil {
		return nil, err
	}
	if err := loader.Attach(cfg); err != nil {
		log.Fatal(err)
	}

	reader, err := lifecycle.NewRingbufReader(loader)
	if err != nil {
		loader.Close()
		return nil, err
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		lifecycle.RunProcExitRd(ctx, reader, cfg)
	}()

	// cleanup function
	return func() {
		<-done
		loader.Close()
	}, nil
}

func startCuInitTracer(ctx context.Context) (func(), error) {
	fmt.Println("Starting cuInit tracer...")
	cfg := lifecycle.Config{} // if you have any config, set it here
	loader, err := lifecycle.LoadCuInitObjects(cfg)
	if err != nil {
		return nil, err
	}
	links, err := loader.Attach(internal.CudaSo, loader)
	if err != nil {
		log.Fatal(err)
	}

	reader, err := lifecycle.NewCuInitRingbufReader(loader)
	if err != nil {
		loader.Close()
		return nil, err
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		lifecycle.RunCuInitRd(ctx, reader, cfg)
	}()

	// cleanup function
	return func() {
		<-done
		for _, link := range links {
			link.Close()
		}
		loader.Close()
	}, nil
}
func Execute() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	// Thread 1: Start lifecycle tracer
	cleanupLifecycle, err := startLifecycle(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer cleanupLifecycle()

	// Thread 2: get current GPU procs are running, get all .so and syms
	pids, err := pid.GetRunningProcesses()
	if err != nil {
		log.Fatal(err)
	}
	if len(pids) == 0 {
		log.Println("No GPU processes found.")
		return
	}
	pids.CachePID() // cache the PID info into map
	// c := pid.GlobalPIDCache() get global cache instance

	err = so.InitFromSharedObject(internal.CudaSo)
	if err != nil {
		log.Fatal(err)
	}
	// Thread 3: Observe cuInit events
	cleanupCuInit, err := startCuInitTracer(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer cleanupCuInit()

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
