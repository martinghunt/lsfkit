package root

import (
	"fmt"
	"strconv"

	"github.com/martinghunt/lsfkit/internal/lsf"
	"github.com/spf13/cobra"
)

type runOptions struct {
	out              string
	err              string
	checkpointDir    string
	memoryUnits      string
	tokensName       string
	queue            string
	checkpoint       bool
	interactive      bool
	norun            bool
	checkpointPeriod int
	arrayLimit       int
	start            int
	end              int
	threads          int
	tokensNumber     int
	tmpSpace         float64
	done             []string
	ended            []string
}

func newRunCommand() *cobra.Command {
	options := runOptions{}
	command := &cobra.Command{
		Use:   "run [options] <memory-gb> <name> <command>",
		Short: "Submit an LSF job",
		Args:  cobra.MinimumNArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			memory, err := strconv.ParseFloat(args[0], 64)
			if err != nil || memory < 0 {
				return fmt.Errorf("memory must be a non-negative number of GB")
			}

			job := lsf.Job{
				Out:              options.out,
				Err:              options.err,
				Name:             args[1],
				CommandArgs:      args[2:],
				MemoryGB:         memory,
				TmpSpaceGB:       options.tmpSpace,
				Threads:          options.threads,
				ArrayStart:       options.start,
				ArrayEnd:         options.end,
				ArrayLimit:       options.arrayLimit,
				Checkpoint:       options.checkpoint,
				Interactive:      options.interactive,
				CheckpointDir:    options.checkpointDir,
				CheckpointPeriod: options.checkpointPeriod,
				MemoryUnits:      options.memoryUnits,
				TokensName:       options.tokensName,
				TokensNumber:     options.tokensNumber,
				Queue:            options.queue,
				Done:             options.done,
				Ended:            options.ended,
			}

			bsubArgs, err := job.Args()
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), lsf.CommandString(bsubArgs))
			if options.norun {
				return nil
			}
			if options.interactive {
				return lsf.ExecInteractiveArgs(bsubArgs)
			}
			id, err := lsf.SubmitArgs(bsubArgs)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), id, "submitted")
			return nil
		},
	}

	flags := command.Flags()
	flags.StringVarP(&options.err, "err", "e", "", "stderr file (default: name.e)")
	flags.StringVarP(&options.out, "out", "o", "", "stdout file (default: name.o)")
	flags.BoolVarP(&options.checkpoint, "checkpoint", "c", false, "use checkpointing")
	flags.BoolVarP(&options.interactive, "interactive", "i", false, "run interactively using bsub -Is")
	flags.StringVarP(&options.checkpointDir, "checkpoint-dir", "d", "", "checkpoint directory")
	flags.IntVarP(&options.checkpointPeriod, "checkpoint-period", "p", 600, "checkpoint period in minutes")
	flags.IntVar(&options.arrayLimit, "array-limit", 100, "maximum concurrently running array jobs")
	flags.IntVar(&options.start, "start", 0, "array start index")
	flags.IntVar(&options.end, "end", 0, "array end index")
	flags.StringSliceVar(&options.done, "done", nil, "depend on successfully completed job (repeatable)")
	flags.StringSliceVar(&options.ended, "ended", nil, "depend on ended job (repeatable)")
	flags.StringVar(&options.memoryUnits, "memory-units", "", "LSF memory units: KB or MB")
	flags.Float64Var(&options.tmpSpace, "tmp-space", 0, "temporary space in GB")
	flags.IntVar(&options.threads, "threads", 1, "requested threads")
	flags.StringVar(&options.tokensName, "tokens-name", "", "resource token name")
	flags.IntVar(&options.tokensNumber, "tokens-number", 100, "resource token value")
	flags.StringVarP(&options.queue, "queue", "q", "", "queue")
	flags.BoolVar(&options.norun, "norun", false, "print bsub command without submitting")
	flags.SetInterspersed(false)
	return command
}
