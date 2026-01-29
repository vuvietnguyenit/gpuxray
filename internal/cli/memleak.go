// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

package cli

import (
	"github.com/spf13/cobra"
	"github.com/vuvietnguyenit/gpuxray/internal"
	"github.com/vuvietnguyenit/gpuxray/internal/app"
)

var memleakCmd = &cobra.Command{
	Use:   "memleak",
	Short: "Memory-leaked tracing",
	RunE: func(cmd *cobra.Command, args []string) error {
		app.RunMemleakTrace()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(memleakCmd)
	// Define a local flag for the 'serve' command.
	memleakCmd.Flags().Uint32VarP(&internal.MemoryleakFlags.Pid, "pid", "p", 0, "trace GPU memory leaked by PID")
}
