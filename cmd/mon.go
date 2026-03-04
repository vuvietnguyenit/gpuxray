// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/vuvietnguyenit/gpuxray/internal"
	"github.com/vuvietnguyenit/gpuxray/internal/logging"
	"github.com/vuvietnguyenit/gpuxray/internal/mon"
)

func newMonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mon",
		Short: "Run GPU process monitor",
		Long:  "Run GPU process monitor and expose Prometheus metrics",
		RunE:  runMon,
	}

	cmd.Flags().StringVar(&internal.MonFlags.Port, "port", ":2112", "Port to expose metrics")
	cmd.Flags().StringVar(&internal.MonFlags.Path, "path", "/metrics", "Path to expose metrics")
	// cmd.Flags().BoolVarP(&internal.MonFlags.UseK8s, "use-k8s", "k", false, "Use Kubernetes integration")

	return cmd
}

func init() {
	rootCmd.AddCommand(newMonCmd())
}

func runMon(cmd *cobra.Command, _ []string) error {
	// get context from root
	var resolver mon.PIDResolver = &mon.NoopResolver{}
	m := mon.New(mon.Config{
		ListenAddr:  internal.MonFlags.Port,
		MetricsPath: internal.MonFlags.Path,
		Resolver:    resolver,
		Logger:      logging.L(),
	})
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := m.Run(ctx); err != nil {
		logging.L().Error().Msgf("monitor exited with error: %v", err)
		os.Exit(1)
	}
	return nil
}
