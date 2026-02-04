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
	"github.com/vuvietnguyenit/gpuxray/internal/logging"
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
	logging.L().Debug().Msg("removed memlock limit successfully")
	return nil
}

func initLogger() {
	logging.Init(logging.Config{
		Level:  internal.LogLevel,
		Format: internal.LogFormat, // auto | json | console
	})
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
		logging.L().Debug().Msg("initialized NVML")
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		ret := nvml.Shutdown()
		if ret != nvml.SUCCESS {
			log.Fatalf("Unable to shutdown NVML: %v", nvml.ErrorString(ret))
			os.Exit(1)
		}
		logging.L().Debug().Msg("Shutdown NVML")
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&internal.RemoveMemlock, "remove-memlock", true, "Remove RLIMIT_MEMLOCK before loading eBPF programs")
	rootCmd.PersistentFlags().IntVarP(&internal.FetchInterval, "fetch-interval", "i", 5, "Interval in seconds to fetch GPU tracing console")
	rootCmd.PersistentFlags().StringVarP(&internal.CudaSo, "cuda-so", "c", "/usr/lib/x86_64-linux-gnu/libcuda.so", "Path to CUDA shared object")
	rootCmd.PersistentFlags().StringVarP(&internal.LogFormat, "log-format", "l", "console", "The format of log, it can be auto | json | console")
	rootCmd.PersistentFlags().StringVarP(&internal.LogLevel, "log-level", "v", "info", "Log level")
	initLogger()
	rootCmd.AddCommand(memleak.NewCmd())
}

func startLifecycle(parent context.Context) (func(), error) {
	logging.L().Debug().Msg("Starting lifecycle tracer...")

	ctx, cancel := context.WithCancel(parent)

	cfg := lifecycle.Config{} // if you have any config, set it here
	loader, err := lifecycle.LoadProcExitObjects(cfg)
	if err != nil {
		cancel()
		return nil, err
	}
	if err := loader.Attach(cfg); err != nil {
		cancel()
		log.Fatal(err)
	}

	reader, err := lifecycle.NewRingbufReader(loader)
	if err != nil {
		cancel()
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
		cancel()
		reader.Close()
		<-done
		loader.Close()
	}, nil
}

func startCuInitTracer(parent context.Context) (func(), error) {
	logging.L().Debug().Msg("Starting cuInit tracer...")
	ctx, cancel := context.WithCancel(parent)

	cfg := lifecycle.Config{}
	loader, err := lifecycle.LoadCuInitObjects(cfg)
	if err != nil {
		cancel()
		return nil, err
	}

	links, err := loader.Attach(internal.CudaSo, loader)
	if err != nil {
		cancel()
		loader.Close()
		return nil, err
	}

	reader, err := lifecycle.NewCuInitRingbufReader(loader)
	if err != nil {
		cancel()
		for _, l := range links {
			l.Close()
		}
		loader.Close()
		return nil, err
	}

	done := make(chan struct{})

	go func() {
		defer close(done)
		lifecycle.RunCuInitRd(ctx, reader, cfg)
	}()

	// cleanup
	return func() {
		cancel()
		reader.Close()
		for _, link := range links {
			link.Close()
		}
		<-done
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
