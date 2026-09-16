package root

import (
	"fmt"
	"strconv"

	"github.com/martinghunt/lsfkit/internal/lsf"
	"github.com/spf13/cobra"
)

// submissionOptions are shared by commands which submit a non-interactive LSF
// job.  Commands add their own input-specific options separately.
type submissionOptions struct {
	out          string
	err          string
	memoryUnits  string
	tokensName   string
	queue        string
	norun        bool
	arrayLimit   int
	threads      int
	tokensNumber int
	tmpSpace     float64
	done         []string
	ended        []string
}

func addSubmissionFlags(command *cobra.Command, options *submissionOptions, arrayLogs bool) {
	flags := command.Flags()
	errUsage, outUsage := "stderr file (default: name.e)", "stdout file (default: name.o)"
	if arrayLogs {
		errUsage, outUsage = "stderr filename prefix (default: name)", "stdout filename prefix (default: name)"
	}
	flags.StringVarP(&options.err, "err", "e", "", errUsage)
	flags.StringVarP(&options.out, "out", "o", "", outUsage)
	flags.IntVar(&options.arrayLimit, "array-limit", 100, "maximum concurrently running array jobs")
	flags.StringSliceVar(&options.done, "done", nil, "depend on successfully completed job (repeatable)")
	flags.StringSliceVar(&options.ended, "ended", nil, "depend on ended job (repeatable)")
	flags.StringVar(&options.memoryUnits, "memory-units", "", "LSF memory units: KB or MB")
	flags.Float64Var(&options.tmpSpace, "tmp-space", 0, "temporary space in GB")
	flags.IntVar(&options.threads, "threads", 1, "requested threads")
	flags.StringVar(&options.tokensName, "tokens-name", "", "resource token name")
	flags.IntVar(&options.tokensNumber, "tokens-number", 100, "resource token value")
	flags.StringVarP(&options.queue, "queue", "q", "", "queue")
	flags.BoolVar(&options.norun, "norun", false, "print bsub command without submitting")
}

func parseMemory(value string) (float64, error) {
	memory, err := strconv.ParseFloat(value, 64)
	if err != nil || memory < 0 {
		return 0, fmt.Errorf("memory must be a non-negative number of GB")
	}
	return memory, nil
}

func (options submissionOptions) job(memory float64, name string, commandArgs []string) lsf.Job {
	return lsf.Job{
		Out:          options.out,
		Err:          options.err,
		Name:         name,
		CommandArgs:  commandArgs,
		MemoryGB:     memory,
		TmpSpaceGB:   options.tmpSpace,
		Threads:      options.threads,
		ArrayLimit:   options.arrayLimit,
		MemoryUnits:  options.memoryUnits,
		TokensName:   options.tokensName,
		TokensNumber: options.tokensNumber,
		Queue:        options.queue,
		Done:         options.done,
		Ended:        options.ended,
	}
}

func submitJob(cmd *cobra.Command, job lsf.Job, norun bool) error {
	bsubArgs, err := job.Args()
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), lsf.CommandString(bsubArgs))
	if norun {
		return nil
	}
	id, err := lsf.SubmitArgs(bsubArgs)
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), id, "submitted")
	return nil
}
