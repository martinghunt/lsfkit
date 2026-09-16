package root

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

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
			executable, err := os.Executable()
			if err != nil {
				return fmt.Errorf("get lsfkit executable path: %w", err)
			}

			job := options.job(memory, args[1], []string{executable, "array-task", commandsFile})
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

func newArrayTaskCommand() *cobra.Command {
	command := &cobra.Command{
		Use:    "array-task <commands-file>",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			index, err := strconv.Atoi(os.Getenv("LSB_JOBINDEX"))
			if err != nil || index < 1 {
				return fmt.Errorf("array task requires a positive LSB_JOBINDEX")
			}
			line, err := commandLine(args[0], index)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "lsfkit array: line %d: %s\n", index, line)

			child := exec.Command("sh", "-c", line)
			child.Stdin = os.Stdin
			child.Stdout = cmd.OutOrStdout()
			child.Stderr = cmd.ErrOrStderr()
			if err := child.Run(); err != nil {
				return fmt.Errorf("array command at line %d: %w", index, err)
			}
			return nil
		},
	}
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

func commandLine(filename string, wanted int) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", fmt.Errorf("open commands file: %w", err)
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	for lineNumber := 1; ; lineNumber++ {
		line, err := reader.ReadString('\n')
		if lineNumber == wanted {
			line = strings.TrimSuffix(line, "\n")
			if strings.TrimSpace(line) == "" {
				return "", fmt.Errorf("no command at line %d in %s", wanted, filename)
			}
			return line, nil
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("read commands file: %w", err)
		}
	}
	return "", fmt.Errorf("no command at line %d in %s", wanted, filename)
}
