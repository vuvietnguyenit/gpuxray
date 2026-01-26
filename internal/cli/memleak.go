package cli

import (
	"github.com/spf13/cobra"
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

type MemleakFlags struct {
	Pid int
}

var MemoryleakFlags MemleakFlags

func init() {
	rootCmd.AddCommand(memleakCmd)
	// Define a local flag for the 'serve' command.
	memleakCmd.Flags().IntVarP(&MemoryleakFlags.Pid, "pid", "p", 0, "trace GPU memory leaked by PID")
}
