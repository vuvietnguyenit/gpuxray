package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/vuvietnguyenit/gpuxray/internal/app"
)

var memleakCmd = &cobra.Command{
	Use:   "memleak",
	Short: "Memory-leaked tracing",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("run memleak")
		app.ShowMemleak()
		return nil
	},
}

type MemleakFlags struct {
	Pid int
}

var MemoryleakFlags MemleakFlags

func init() {
	rootCmd.AddCommand(memleakCmd)
	// Define a local flag for the 'serve' command.
	memleakCmd.Flags().IntVarP(&MemoryleakFlags.Pid, "pid", "p", 0, "trace GPU memory leaked by PID")
}
