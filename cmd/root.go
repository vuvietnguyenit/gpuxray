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
	"github.com/vuvietnguyenit/gpuxray/internal"
	"github.com/vuvietnguyenit/gpuxray/internal/logging"
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

var rootCmd = &cobra.Command{
	Use:   "gpuxray",
	Short: "eBPF tool this help tracing and investigating GPU",

	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		rootCtx, rootCancel = signal.NotifyContext(
			context.Background(),
			os.Interrupt,
			syscall.SIGTERM,
		)
		// DEBUG purpose
		go func() {
			<-rootCtx.Done()
			logging.L().Debug().
				Err(rootCtx.Err()).
				Msg("rootCmd context canceled")

		}()
		// init logging profile
		logging.Init(logging.Config{
			Level:  internal.LogLevel,
			Format: internal.LogFormat, // auto | json | console
		})
		removeMemlock()
		ret := nvml.Init()
		if ret != nvml.SUCCESS {
			log.Fatalf("Unable to initialize NVML: %v", nvml.ErrorString(ret))
			os.Exit(1)
		}
		logging.L().Debug().Msg("initialized NVML")
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if rootCancel != nil {
			rootCancel()
		}
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
}

func Execute() {

	if err := rootCmd.ExecuteContext(rootCtx); err != nil {
		os.Exit(1)
	}
}
