// Package ostats parses LSF job notification output files.
package ostats

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var AllColumns = []string{"exit_code", "cpu_time", "wall_clock_time", "max_memory", "requested_memory", "max_processes", "max_threads", "start_time", "end_time", "exec_host", "username", "working_dir", "job_name"}
var ShortColumns = []string{"exit_code", "cpu_time", "wall_clock_time", "max_memory", "requested_memory"}

type Stats struct {
	ExitCode                                           *int
	CPUTime, WallClockTime, MaxMemory, RequestedMemory *float64
	MaxProcesses, MaxThreads                           *int
	StartTime, EndTime                                 *time.Time
	ExecHost, Username, WorkingDir, JobName            *string
}

var (
	jobRE     = regexp.MustCompile(`^Job <(.*)> was submitted from host <.*> by user <(.*)> in cluster <.*>\.$`)
	execRE    = regexp.MustCompile(`^Job was executed on host\(s\) <(.*)>, in queue <.*>, as user <.*> in cluster <.*>\.$`)
	workRE    = regexp.MustCompile(`^<(.*)> was used as the working directory\.$`)
	exitRE    = regexp.MustCompile(`^Exited with exit code ([0-9]+)\.$`)
	cpuRE     = regexp.MustCompile(`^\s+CPU time\s+:\s+([0-9]+(?:\.[0-9]+)?) sec\.$`)
	maxMemRE  = regexp.MustCompile(`^\s+Max Memory\s+:\s+([0-9]+(?:\.[0-9]+)?) MB$`)
	reqMemRE  = regexp.MustCompile(`^\s+Total Requested Memory\s+:\s+([0-9]+(?:\.[0-9]+)?) MB`)
	processRE = regexp.MustCompile(`^\s+Max Processes\s+:\s+([0-9]+)$`)
	threadRE  = regexp.MustCompile(`^\s+Max Threads\s+:\s+([0-9]+)$`)
)

func ptr[T any](v T) *T { return &v }
func (s *Stats) parse(line string) {
	if m := jobRE.FindStringSubmatch(line); m != nil {
		s.JobName = ptr(m[1])
		s.Username = ptr(m[2])
	}
	if m := execRE.FindStringSubmatch(line); m != nil {
		s.ExecHost = ptr(m[1])
	}
	if m := workRE.FindStringSubmatch(line); m != nil {
		s.WorkingDir = ptr(m[1])
	}
	if line == "Successfully completed." {
		s.ExitCode = ptr(0)
	} else if m := exitRE.FindStringSubmatch(line); m != nil {
		v, _ := strconv.Atoi(m[1])
		s.ExitCode = ptr(v)
	}
	if m := cpuRE.FindStringSubmatch(line); m != nil {
		v, _ := strconv.ParseFloat(m[1], 64)
		s.CPUTime = ptr(v)
	}
	if m := maxMemRE.FindStringSubmatch(line); m != nil {
		v, _ := strconv.ParseFloat(m[1], 64)
		s.MaxMemory = ptr(v / 1000)
	}
	if m := reqMemRE.FindStringSubmatch(line); m != nil {
		v, _ := strconv.ParseFloat(m[1], 64)
		s.RequestedMemory = ptr(v / 1000)
	}
	if m := processRE.FindStringSubmatch(line); m != nil {
		v, _ := strconv.Atoi(m[1])
		s.MaxProcesses = ptr(v)
	}
	if m := threadRE.FindStringSubmatch(line); m != nil {
		v, _ := strconv.Atoi(m[1])
		s.MaxThreads = ptr(v)
	}
	if strings.HasPrefix(line, "Started ") {
		s.StartTime = parseTime(line)
	}
	if strings.HasPrefix(line, "Results reported ") {
		s.EndTime = parseTime(line)
		if s.StartTime != nil && s.EndTime != nil {
			v := s.EndTime.Sub(*s.StartTime).Seconds()
			s.WallClockTime = ptr(v)
		}
	}
}
func parseTime(line string) *time.Time {
	fields := strings.Fields(line)
	if len(fields) < 6 {
		return nil
	}
	t, err := time.Parse("Mon Jan 2 15:04:05 2006", strings.Join(fields[len(fields)-5:], " "))
	if err != nil {
		return nil
	}
	return &t
}

// Read parses each LSF notification. A missing stderr footer ends at EOF rather than looping forever.
func Read(r io.Reader) ([]Stats, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	var out []Stats
	var current *Stats
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Sender: LSF System <") {
			if current != nil {
				out = append(out, *current)
			}
			current = &Stats{}
			continue
		}
		if current != nil {
			current.parse(line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if current != nil {
		out = append(out, *current)
	}
	return out, nil
}
func ReadFile(name string) ([]Stats, error) {
	f, e := os.Open(name)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	return Read(f)
}
func value(s Stats, col, unit string) string {
	mult := 1.0
	if unit == "m" {
		mult = 1.0 / 60
	}
	if unit == "h" {
		mult = 1.0 / 3600
	}
	number := func(v *float64, scale float64) string {
		if v == nil {
			return "*"
		}
		return fmt.Sprintf("%g", *v*scale)
	}
	timeNumber := func(v *float64) string {
		if v == nil {
			return "*"
		}
		value := math.Round(*v*mult*100) / 100
		return strconv.FormatFloat(value, 'f', 2, 64)
	}
	integer := func(v *int) string {
		if v == nil {
			return "*"
		}
		return strconv.Itoa(*v)
	}
	str := func(v *string) string {
		if v == nil {
			return "*"
		}
		return *v
	}
	tim := func(v *time.Time) string {
		if v == nil {
			return "*"
		}
		return v.Format("2006-01-02 15:04:05")
	}
	switch col {
	case "exit_code":
		return integer(s.ExitCode)
	case "cpu_time":
		return timeNumber(s.CPUTime)
	case "wall_clock_time":
		return timeNumber(s.WallClockTime)
	case "max_memory":
		return number(s.MaxMemory, 1)
	case "requested_memory":
		return number(s.RequestedMemory, 1)
	case "max_processes":
		return integer(s.MaxProcesses)
	case "max_threads":
		return integer(s.MaxThreads)
	case "start_time":
		return tim(s.StartTime)
	case "end_time":
		return tim(s.EndTime)
	case "exec_host":
		return str(s.ExecHost)
	case "username":
		return str(s.Username)
	case "working_dir":
		return str(s.WorkingDir)
	case "job_name":
		return str(s.JobName)
	}
	return "*"
}
func HasData(s Stats) bool {
	return s.JobName != nil || s.ExitCode != nil || s.CPUTime != nil || s.StartTime != nil
}
func Row(s Stats, cols []string, unit string) []string {
	r := make([]string, len(cols))
	for i, c := range cols {
		r[i] = value(s, c, unit)
	}
	return r
}
