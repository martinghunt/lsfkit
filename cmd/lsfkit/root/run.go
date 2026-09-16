package root

import (
	"fmt"

	"github.com/martinghunt/lsfkit/internal/lsf"
	"github.com/spf13/cobra"
)

type runOptions struct {
	submissionOptions
	interactive bool
	start       int
	end         int
}

func newRunCommand() *cobra.Command {
	options := runOptions{}
	command := &cobra.Command{
		Use:   "run [options] <memory-gb> <name> <command>",
		Short: "Submit an LSF job",
		Args:  cobra.MinimumNArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			memory, err := parseMemory(args[0])
			if err != nil {
				return err
			}
			job := options.job(memory, args[1], args[2:])
			job.ArrayStart = options.start
			job.ArrayEnd = options.end
			job.Interactive = options.interactive
			if options.interactive {
				bsubArgs, err := job.Args()
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), lsf.CommandString(bsubArgs))
				if options.norun {
					return nil
				}
				return lsf.ExecInteractiveArgs(bsubArgs)
			}
			return submitJob(cmd, job, options.norun)
		},
	}

	flags := command.Flags()
	flags.BoolVarP(&options.interactive, "interactive", "i", false, "run interactively using bsub -Is")
	flags.IntVar(&options.start, "start", 0, "array start index")
	flags.IntVar(&options.end, "end", 0, "array end index")
	addSubmissionFlags(command, &options.submissionOptions, false)
	flags.SetInterspersed(false)
	return command
}
