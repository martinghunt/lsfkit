package root

import (
	"fmt"
	"os"

	"github.com/martinghunt/lsfkit/internal/buildinfo"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:          "lsfkit",
	Short:        "LSF job submission and output statistics",
	SilenceUsage: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Version = buildinfo.Version
	rootCmd.AddCommand(runCmd, ostatsCmd)
}
