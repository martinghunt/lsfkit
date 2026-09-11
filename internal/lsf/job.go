// Package lsf constructs and submits LSF bsub jobs.
package lsf

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	numericDependencyRE = regexp.MustCompile(`^[0-9]+$`)
	safeShellArgRE      = regexp.MustCompile(`^[A-Za-z0-9_@%+=:,./-]+$`)
	jobIDRE             = regexp.MustCompile(`(?m)^Job <([0-9]+)> is submitted to`)
)

// Job describes an LSF job submission.
type Job struct {
	Out              string
	Err              string
	Name             string
	Queue            string
	CommandArgs      []string
	MemoryGB         float64
	TmpSpaceGB       float64
	Threads          int
	ArrayStart       int
	ArrayEnd         int
	ArrayLimit       int
	CheckpointPeriod int
	TokensNumber     int
	Checkpoint       bool
	Interactive      bool
	CheckpointDir    string
	MemoryUnits      string
	TokensName       string
	Done             []string
	Ended            []string
}

// Args returns the exact argument vector to pass to bsub.
func (j Job) Args() ([]string, error) {
	if err := j.validate(); err != nil {
		return nil, err
	}

	outputSpecified := j.Out != "" || j.Err != ""
	if j.Out == "" {
		j.Out = j.Name + ".o"
	}
	if j.Err == "" {
		j.Err = j.Name + ".e"
	}
	if !j.Interactive || outputSpecified {
		if err := validateLogFiles(j.Out, j.Err); err != nil {
			return nil, err
		}
	}

	resource, maxMemory, err := j.resourceRequest()
	if err != nil {
		return nil, err
	}
	args := make([]string, 0, 24+len(j.CommandArgs))
	if j.Interactive {
		args = append(args, "-Is")
	}
	if j.Checkpoint {
		dir := j.CheckpointDir
		if dir == "" {
			dir = j.Out + ".checkpoint"
		}
		absoluteDir, err := filepath.Abs(dir)
		if err != nil {
			return nil, err
		}
		args = append(args, "-k", absoluteDir+" method=blcr "+strconv.Itoa(j.CheckpointPeriod))
	}
	if j.Queue != "" {
		args = append(args, "-q", j.Queue)
	}
	if home, err := os.UserHomeDir(); err == nil {
		args = append(args, "-E", "test -e "+home)
	}
	if j.Threads > 1 {
		args = append(args, "-n", strconv.Itoa(j.Threads))
	}
	args = append(args, "-R", resource, "-M", maxMemory)

	jobName := j.Name
	if j.ArrayStart > 0 {
		args = append(args, "-o", j.Out+".%I", "-e", j.Err+".%I")
		jobName = fmt.Sprintf("%s[%d-%d]%%%d", j.Name, j.ArrayStart, j.ArrayEnd, j.ArrayLimit)
	} else if !j.Interactive || outputSpecified {
		args = append(args, "-o", j.Out, "-e", j.Err)
	}
	args = append(args, "-J", jobName)
	if dependencies := j.dependencyExpression(); dependencies != "" {
		args = append(args, "-w", dependencies)
	}

	commandArgs := append([]string(nil), j.CommandArgs...)
	if j.ArrayStart > 0 {
		for i := range commandArgs {
			commandArgs[i] = strings.ReplaceAll(commandArgs[i], "INDEX", "$LSB_JOBINDEX")
		}
	}
	if j.Checkpoint {
		commandArgs = append([]string{"cr_run"}, commandArgs...)
	}
	return append(args, commandArgs...), nil
}

func (j Job) validate() error {
	if len(j.CommandArgs) == 0 {
		return fmt.Errorf("no command given")
	}
	if j.MemoryGB < 0 || j.TmpSpaceGB < 0 {
		return fmt.Errorf("memory and temporary space must not be negative")
	}
	if j.Threads < 1 {
		return fmt.Errorf("threads must be at least 1")
	}
	if (j.ArrayStart == 0) != (j.ArrayEnd == 0) {
		return fmt.Errorf("--start and --end must be supplied together")
	}
	if j.ArrayStart < 0 || j.ArrayEnd < j.ArrayStart {
		return fmt.Errorf("array end must be at least array start")
	}
	if j.ArrayStart > 0 && j.ArrayLimit < 1 {
		return fmt.Errorf("array limit must be at least 1")
	}
	if j.Interactive && j.ArrayStart != 0 {
		return fmt.Errorf("--interactive cannot be used with a job array")
	}
	if j.Interactive && j.Checkpoint {
		return fmt.Errorf("--interactive cannot be used with checkpointing")
	}
	return nil
}

func memoryMB(gb float64) int {
	return int(math.Round(gb * 1000))
}

func (j Job) resourceRequest() (string, string, error) {
	units, err := j.memoryUnits()
	if err != nil {
		return "", "", err
	}
	mem, tmp := memoryMB(j.MemoryGB), memoryMB(j.TmpSpaceGB)
	resource := "select[mem>" + strconv.Itoa(mem)
	if tmp > 0 {
		resource += " && tmp>" + strconv.Itoa(tmp)
	}
	resource += "] rusage[mem=" + strconv.Itoa(mem)
	if tmp > 0 {
		resource += ",tmp=" + strconv.Itoa(tmp)
	}
	if j.TokensName != "" {
		resource += "," + j.TokensName + "=" + strconv.Itoa(j.TokensNumber)
	}
	resource += "]"
	if j.Threads > 1 {
		resource = "span[hosts=1] " + resource
	}
	maxMemory := strconv.Itoa(mem)
	if units == "KB" {
		maxMemory += "000"
	}
	return resource, maxMemory, nil
}

func validateLogFiles(files ...string) error {
	for _, file := range files {
		dir := filepath.Dir(file)
		if dir == "." {
			continue
		}
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			return fmt.Errorf("log directory does not exist: %s", dir)
		}
	}
	return nil
}

func (j Job) dependencyExpression() string {
	dependencies := make([]string, 0, len(j.Done)+len(j.Ended))
	for _, name := range j.Done {
		dependencies = append(dependencies, "done("+dependency(name)+")")
	}
	for _, name := range j.Ended {
		dependencies = append(dependencies, "ended("+dependency(name)+")")
	}
	return strings.Join(dependencies, " && ")
}

func dependency(value string) string {
	if numericDependencyRE.MatchString(value) {
		return value
	}
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return `"` + value + `"`
}

func (j Job) memoryUnits() (string, error) {
	if j.MemoryUnits != "" {
		return validUnits(j.MemoryUnits)
	}
	if units := os.Getenv("LSFKIT_LSF_MEMORY_UNITS"); units != "" {
		return validUnits(units)
	}
	host, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("get hostname for LSF memory-unit lookup: %w", err)
	}
	out, err := exec.Command("lsadmin", "showconf", "lim", host).Output()
	if err != nil {
		return "", fmt.Errorf("get LSF memory units: run lsadmin showconf lim or set LSFKIT_LSF_MEMORY_UNITS to KB or MB: %w", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[0] == "LSF_UNIT_FOR_LIMITS" {
			return validUnits(fields[2])
		}
	}
	return "KB", nil
}

func validUnits(units string) (string, error) {
	if units == "KB" || units == "MB" {
		return units, nil
	}
	return "", fmt.Errorf("invalid LSF memory units %q (want KB or MB)", units)
}

func shellQuote(arg string) string {
	if safeShellArgRE.MatchString(arg) {
		return arg
	}
	return "'" + strings.ReplaceAll(arg, "'", "'\"'\"'") + "'"
}

// CommandString returns a shell-safe representation of a bsub argument vector.
func CommandString(args []string) string {
	parts := make([]string, 1, len(args)+1)
	parts[0] = "bsub"
	for _, arg := range args {
		parts = append(parts, shellQuote(arg))
	}
	return strings.Join(parts, " ")
}

// String validates the job and returns its shell-safe bsub command.
func (j Job) String() (string, error) {
	args, err := j.Args()
	if err != nil {
		return "", err
	}
	return CommandString(args), nil
}

// SubmitArgs runs bsub with a prepared argument vector and returns its job ID.
func SubmitArgs(args []string) (string, error) {
	out, err := exec.Command("bsub", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("bsub failed: %w\n%s", err, out)
	}
	matches := jobIDRE.FindStringSubmatch(string(out))
	if matches == nil {
		return "", fmt.Errorf("could not get job ID from bsub output:\n%s", out)
	}
	return matches[1], nil
}

// Submit validates and submits the job.
func (j Job) Submit() (string, error) {
	args, err := j.Args()
	if err != nil {
		return "", err
	}
	return SubmitArgs(args)
}
