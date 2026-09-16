package root

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

const arrayLauncher = `line=$(sed -n "${LSB_JOBINDEX}p" "$1") || exit 1
case "$line" in
  *[![:space:]]*) printf 'lsfkit array: line %s: %s\n' "$LSB_JOBINDEX" "$line"; exec sh -c "$line" ;;
  *) printf 'no command at line %s in %s\n' "$LSB_JOBINDEX" "$1" >&2; exit 1 ;;
esac`

func newArrayCommand() *cobra.Command {
	options := submissionOptions{}
	command := &cobra.Command{
		Use:   "array [options] <memory-gb> <name> <commands-file>",
		Short: "Submit an LSF job array from a file of commands",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			memory, err := parseMemory(args[0])
			if err != nil {
				return err
			}
			commandsFile, count, err := validateCommandsFile(args[2])
			if err != nil {
				return err
			}

			job := options.job(memory, args[1], []string{"sh", "-c", arrayLauncher, "sh", commandsFile})
			job.ArrayStart = 1
			job.ArrayEnd = count
			job.ArrayOut = arrayLogName(options.out, args[1], "o")
			job.ArrayErr = arrayLogName(options.err, args[1], "e")
			job.DisableIndexReplacement = true
			return submitJob(cmd, job, options.norun)
		},
	}
	addSubmissionFlags(command, &options, true)
	command.Flags().SetInterspersed(false)
	return command
}

func arrayLogName(prefix, name, extension string) string {
	if prefix == "" {
		prefix = name
	}
	return prefix + ".%I." + extension
}

func validateCommandsFile(filename string) (string, int, error) {
	absFilename, err := filepath.Abs(filename)
	if err != nil {
		return "", 0, fmt.Errorf("get absolute commands-file path: %w", err)
	}
	info, err := os.Stat(absFilename)
	if err != nil {
		return "", 0, fmt.Errorf("stat commands file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", 0, fmt.Errorf("commands file is not a regular file: %s", filename)
	}
	file, err := os.Open(absFilename)
	if err != nil {
		return "", 0, fmt.Errorf("open commands file: %w", err)
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	lineNumber := 0
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			lineNumber++
			if strings.TrimSpace(line) == "" {
				return "", 0, fmt.Errorf("commands file contains an empty line at line %d", lineNumber)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", 0, fmt.Errorf("read commands file: %w", err)
		}
	}
	if lineNumber == 0 {
		return "", 0, fmt.Errorf("commands file is empty")
	}
	return absFilename, lineNumber, nil
}
