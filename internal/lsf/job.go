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
	MemoryGB, TmpSpaceGB                                                      float64
	Threads, ArrayStart, ArrayEnd, ArrayLimit, CheckpointPeriod, TokensNumber int
	Checkpoint                                                                bool
	CheckpointDir, MemoryUnits, TokensName                                    string
	Done, Ended                                                               []string
}

func memoryMB(gb float64) int { return int(1000 * (float64(int(gb*1000+0.5)) / 1000)) }

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
	if j.Out == "" {
		j.Out = j.Name + ".o"
	}
	if (j.ArrayStart == 0) != (j.ArrayEnd == 0) {
		return "", fmt.Errorf("--start and --end must be supplied together")
	}
	if j.Err == "" {
		j.Err = j.Name + ".e"
	}
	for _, f := range []string{j.Out, j.Err} {
		if dir := filepath.Dir(f); dir != "." {
			if info, err := os.Stat(dir); err != nil || !info.IsDir() {
				return "", fmt.Errorf("log directory does not exist: %s", dir)
			}
		}
	}
	units, err := j.memoryUnits()
	if err != nil {
		return "", err
	}
	mem, tmp := memoryMB(j.MemoryGB), memoryMB(j.TmpSpaceGB)
	parts := []string{"bsub"}
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
		parts = append(parts, "-n", strconv.Itoa(j.Threads))
	}
	parts = append(parts, "-R", quote(resource), "-M", strconv.Itoa(mem))
	if units == "KB" {
		parts[len(parts)-1] += "000"
	}
	if j.ArrayStart > 0 {
		if j.ArrayEnd < j.ArrayStart {
			return "", fmt.Errorf("array end must be at least array start")
		}
		parts = append(parts, "-o", quote(j.Out+".%I"), "-e", quote(j.Err+".%I"), "-J", quote(fmt.Sprintf("%s[%d-%d]%%%d", j.Name, j.ArrayStart, j.ArrayEnd, j.ArrayLimit)))
		j.Command = strings.ReplaceAll(j.Command, "INDEX", "\\$LSB_JOBINDEX")
	} else {
		parts = append(parts, "-o", quote(j.Out), "-e", quote(j.Err), "-J", quote(j.Name))
	}
	deps := []string{}
	for _, d := range j.Done {
		deps = append(deps, "done("+dependency(d)+")")
	}
	for _, d := range j.Ended {
		deps = append(deps, "ended("+dependency(d)+")")
	}
	if len(deps) > 0 {
		parts = append(parts, "-w", quote(strings.Join(deps, " && ")))
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
