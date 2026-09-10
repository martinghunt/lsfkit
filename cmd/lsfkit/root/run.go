package root

import (
	"fmt"
	"strconv"

	"github.com/martinghunt/lsfkit/internal/lsf"
	"github.com/spf13/cobra"
)

var runOptions struct {
	out, err, checkpointDir, memoryUnits, tokensName, queue string
	checkpoint, norun                                       bool
	checkpointPeriod                                        int
	arrayLimit, start, end, threads, tokensNumber           int
	memory, tmpSpace                                        float64
	done, ended                                             []string
}
var runCmd = &cobra.Command{
	Use:   "run [options] <memory-gb> <name> <command>",
	Short: "Submit an LSF job",
	Args:  cobra.MinimumNArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		memory, err := strconv.ParseFloat(args[0], 64)
		if err != nil || memory < 0 {
			return fmt.Errorf("memory must be a non-negative number of GB")
		}

		job := lsf.Job{
			Out:              runOptions.out,
			Err:              runOptions.err,
			Name:             args[1],
			Command:          joinCommand(args[2:]),
			MemoryGB:         memory,
			TmpSpaceGB:       runOptions.tmpSpace,
			Threads:          runOptions.threads,
			ArrayStart:       runOptions.start,
			ArrayEnd:         runOptions.end,
			ArrayLimit:       runOptions.arrayLimit,
			Checkpoint:       runOptions.checkpoint,
			CheckpointDir:    runOptions.checkpointDir,
			CheckpointPeriod: runOptions.checkpointPeriod,
			MemoryUnits:      runOptions.memoryUnits,
			TokensName:       runOptions.tokensName,
			TokensNumber:     runOptions.tokensNumber,
			Queue:            runOptions.queue,
			Done:             runOptions.done,
			Ended:            runOptions.ended,
		}

		text, err := job.String()
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), text)

		if runOptions.norun {
			return nil
		}
		id, err := job.Submit()
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), id, "submitted")
		return nil
	},
}

func joinCommand(args []string) string {
	result := ""
	for i, a := range args {
		if i > 0 {
			result += " "
		}
		result += a
	}
	return result
}

func init() {
	f := runCmd.Flags()
	f.StringVarP(&runOptions.err, "err", "e", "", "stderr file (default: name.e)")
	f.StringVarP(&runOptions.out, "out", "o", "", "stdout file (default: name.o)")
	f.BoolVarP(&runOptions.checkpoint, "checkpoint", "c", false, "use checkpointing")
	f.StringVarP(&runOptions.checkpointDir, "checkpoint-dir", "d", "", "checkpoint directory")
	f.IntVarP(&runOptions.checkpointPeriod, "checkpoint-period", "p", 600, "checkpoint period in minutes")
	f.IntVar(&runOptions.arrayLimit, "array-limit", 100, "maximum concurrently running array jobs")
	f.IntVar(&runOptions.start, "start", 0, "array start index")
	f.IntVar(&runOptions.end, "end", 0, "array end index")
	f.StringSliceVar(&runOptions.done, "done", nil, "depend on successfully completed job (repeatable)")
	f.StringSliceVar(&runOptions.ended, "ended", nil, "depend on ended job (repeatable)")
	f.StringVar(&runOptions.memoryUnits, "memory-units", "", "LSF memory units: KB or MB")
	f.Float64Var(&runOptions.tmpSpace, "tmp-space", 0, "temporary space in GB")
	f.IntVar(&runOptions.threads, "threads", 1, "requested threads")
	f.StringVar(&runOptions.tokensName, "tokens-name", "", "resource token name")
	f.IntVar(&runOptions.tokensNumber, "tokens-number", 100, "resource token value")
	f.StringVarP(&runOptions.queue, "queue", "q", "", "queue")
	f.BoolVar(&runOptions.norun, "norun", false, "print bsub command without submitting")
}
