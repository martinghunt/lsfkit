// Package lsf constructs and submits LSF bsub jobs.
package lsf

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Job is an LSF job submission.
type Job struct {
	Out, Err, Name, Queue, Command                                            string
	CommandArgs                                                               []string
	MemoryGB, TmpSpaceGB                                                      float64
	Threads, ArrayStart, ArrayEnd, ArrayLimit, CheckpointPeriod, TokensNumber int
	Checkpoint, Interactive                                                   bool
	CheckpointDir, MemoryUnits, TokensName                                    string
	Done, Ended                                                               []string
}

// InteractiveArgs returns the bsub argument vector for an interactive job.
// CommandArgs must contain the original, already shell-parsed command words.
func (j Job) InteractiveArgs() ([]string, error) {
	if !j.Interactive {
		return nil, fmt.Errorf("interactive execution requires an interactive job")
	}
	if len(j.CommandArgs) == 0 {
		return nil, fmt.Errorf("interactive execution requires command arguments")
	}
	if j.ArrayStart != 0 || j.ArrayEnd != 0 {
		return nil, fmt.Errorf("--interactive cannot be used with a job array")
	}
	if j.Checkpoint {
		return nil, fmt.Errorf("--interactive cannot be used with checkpointing")
	}

	outputSpecified := j.Out != "" || j.Err != ""
	if j.Out == "" {
		j.Out = j.Name + ".o"
	}
	if j.Err == "" {
		j.Err = j.Name + ".e"
	}
	if outputSpecified {
		if err := validateLogFiles(j.Out, j.Err); err != nil {
			return nil, err
		}
	}

	resource, maxMemory, err := j.resourceRequest()
	if err != nil {
		return nil, err
	}
	args := []string{"-Is"}
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
	if outputSpecified {
		args = append(args, "-o", j.Out, "-e", j.Err)
	}
	args = append(args, "-J", j.Name)

	if dependencies := j.dependencyExpression(); dependencies != "" {
		args = append(args, "-w", dependencies)
	}
	return append(args, j.CommandArgs...), nil
}

func memoryMB(gb float64) int { return int(1000 * (float64(int(gb*1000+0.5)) / 1000)) }

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

func (j Job) memoryUnits() (string, error) {
	if j.MemoryUnits != "" {
		return validUnits(j.MemoryUnits)
	}
	if units := os.Getenv("FARMPY_LSF_MEMORY_UNITS"); units != "" {
		return validUnits(units)
	}
	host, hostErr := os.Hostname()
	if hostErr != nil {
		return "", fmt.Errorf("get hostname for LSF memory-unit lookup: %w", hostErr)
	}
	out, err := exec.Command("lsadmin", "showconf", "lim", host).Output()
	if err != nil {
		return "", fmt.Errorf("get LSF memory units: run lsadmin showconf lim or set FARMPY_LSF_MEMORY_UNITS to KB or MB: %w", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[0] == "LSF_UNIT_FOR_LIMITS" {
			return validUnits(fields[2])
		}
	}
	return "KB", nil
}
func validUnits(s string) (string, error) {
	if s == "KB" || s == "MB" {
		return s, nil
	}
	return "", fmt.Errorf("invalid LSF memory units %q (want KB or MB)", s)
}

func quote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", "'\\\"'\\\"'") + "'"
}

// String returns the exact shell command that will be submitted.
func (j Job) String() (string, error) {
	if j.Command == "" {
		return "", fmt.Errorf("no command given")
	}
	outputSpecified := j.Out != "" || j.Err != ""
	if j.Out == "" {
		j.Out = j.Name + ".o"
	}
	if (j.ArrayStart == 0) != (j.ArrayEnd == 0) {
		return "", fmt.Errorf("--start and --end must be supplied together")
	}
	if j.Interactive && j.ArrayStart != 0 {
		return "", fmt.Errorf("--interactive cannot be used with a job array")
	}
	if j.Err == "" {
		j.Err = j.Name + ".e"
	}
	if err := validateLogFiles(j.Out, j.Err); err != nil {
		return "", err
	}
	resource, maxMemory, err := j.resourceRequest()
	if err != nil {
		return "", err
	}
	parts := []string{"bsub"}
	if j.Interactive {
		parts = append(parts, "-Is")
	}
	if j.Checkpoint {
		dir := j.CheckpointDir
		if dir == "" {
			dir = j.Out + ".checkpoint"
		}
		abs, err := filepath.Abs(dir)
		if err != nil {
			return "", err
		}
		parts = append(parts, "-k", quote(abs+" method=blcr "+strconv.Itoa(j.CheckpointPeriod)))
	}
	if j.Queue != "" {
		parts = append(parts, "-q", quote(j.Queue))
	}
	if home, err := os.UserHomeDir(); err == nil {
		parts = append(parts, "-E", quote("test -e "+home))
	}
	if j.Threads > 1 {
		parts = append(parts, "-n", strconv.Itoa(j.Threads))
	}
	parts = append(parts, "-R", quote(resource), "-M", maxMemory)
	if j.ArrayStart > 0 {
		if j.ArrayEnd < j.ArrayStart {
			return "", fmt.Errorf("array end must be at least array start")
		}
		parts = append(parts, "-o", quote(j.Out+".%I"), "-e", quote(j.Err+".%I"), "-J", quote(fmt.Sprintf("%s[%d-%d]%%%d", j.Name, j.ArrayStart, j.ArrayEnd, j.ArrayLimit)))
		j.Command = strings.ReplaceAll(j.Command, "INDEX", "\\$LSB_JOBINDEX")
	} else {
		if !j.Interactive || outputSpecified {
			parts = append(parts, "-o", quote(j.Out), "-e", quote(j.Err))
		}
		parts = append(parts, "-J", quote(j.Name))
	}
	if dependencies := j.dependencyExpression(); dependencies != "" {
		parts = append(parts, "-w", quote(dependencies))
	}
	if j.Checkpoint {
		j.Command = "cr_run " + j.Command
	}
	parts = append(parts, j.Command)
	return strings.Join(parts, " "), nil
}

var numeric = regexp.MustCompile(`^[0-9]+$`)

func dependency(s string) string {
	if numeric.MatchString(s) {
		return s
	}
	return `"` + strings.ReplaceAll(s, `"`, `\\"`) + `"`
}

// Submit runs bsub and returns its numeric job ID.
func (j Job) Submit() (string, error) {
	command, err := j.String()
	if err != nil {
		return "", err
	}
	out, err := exec.Command("sh", "-c", command).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("bsub failed: %w\n%s", err, out)
	}
	matches := regexp.MustCompile(`(?m)^Job <([0-9]+)> is submitted to`).FindStringSubmatch(string(out))
	if matches == nil {
		return "", fmt.Errorf("could not get job ID from bsub output:\n%s", out)
	}
	return matches[1], nil
}
