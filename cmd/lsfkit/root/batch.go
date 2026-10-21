package root

import (
	"fmt"
	"io"
	"math"
	"os"

	"github.com/martinghunt/lsfkit/internal/batch"
	"github.com/martinghunt/lsfkit/internal/lsf"
	"github.com/spf13/cobra"
)

func newBatchCommand() *cobra.Command {
	var preview bool
	command := &cobra.Command{
		Use:   "batch [--norun] [jobs-file|-]",
		Short: "Submit a list of jobs with dependencies",
		Long: `Submit one job per line: <name> <depends-on> <memory-gb> [run options] <command>.
Use . for no dependencies, comma-separated earlier job names for done dependencies,
and ended:name for jobs that may finish unsuccessfully. Blank lines and lines
starting with # are ignored. Run options precede the command on each line.
The complete input is validated before submitting jobs. Input defaults to stdin.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var input io.Reader = cmd.InOrStdin()
			if len(args) == 1 && args[0] != "-" {
				file, err := os.Open(args[0])
				if err != nil {
					return fmt.Errorf("open batch file: %w", err)
				}
				defer file.Close()
				input = file
			}
			entries, err := batch.Parse(input, parseBatchJob)
			if err != nil {
				return err
			}
			return batch.Submit(entries, cmd.OutOrStdout(), preview)
		},
	}
	command.Flags().BoolVar(&preview, "norun", false, "validate and print bsub commands without submitting (dependencies shown by name)")
	return command
}

func parseBatchJob(name, memory string, words []string) (lsf.Job, error) {
	options := &runOptions{}
	command := newRunCommandWithOptions(options)
	// Parse only leading run options, leaving the command's own flags untouched.
	if err := command.ParseFlags(words); err != nil {
		return lsf.Job{}, err
	}
	if command.Flags().Changed("help") {
		return lsf.Job{}, fmt.Errorf("--help is not a job option")
	}
	if options.interactive {
		return lsf.Job{}, fmt.Errorf("interactive jobs are not supported in a batch")
	}
	if options.norun {
		return lsf.Job{}, fmt.Errorf("use batch --norun to preview the entire batch")
	}
	value, err := parseMemory(memory)
	if err != nil {
		return lsf.Job{}, err
	}
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return lsf.Job{}, fmt.Errorf("memory must be a finite number of GB")
	}
	job := options.job(value, name, command.Flags().Args())
	job.ArrayStart, job.ArrayEnd = options.start, options.end
	return job, nil
}
