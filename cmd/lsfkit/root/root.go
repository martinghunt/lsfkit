package root

import (
	"fmt"
	"os"

	"github.com/martinghunt/lsfkit/internal/buildinfo"
	"github.com/spf13/cobra"
)

func newRootCommand() *cobra.Command {
	command := &cobra.Command{
		Use:          "lsfkit",
		Short:        "LSF job submission and output statistics",
		SilenceUsage: true,
		Version:      buildinfo.Version,
	}
	command.SetVersionTemplate("{{.Name}} {{.Version}}\n")
	command.AddCommand(newRunCommand(), newArrayCommand(), newOstatsCommand(), newUpdateCommand())
	return command
}

func Execute() {
	if err := newRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
