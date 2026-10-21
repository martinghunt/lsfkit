// Package batch reads and submits small lists of jobs with dependencies.
package batch

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/martinghunt/lsfkit/internal/lsf"
)

type dependency struct {
	name  string
	ended bool
}

// Entry is a validated job and its dependencies on earlier entries.
type Entry struct {
	Job          lsf.Job
	line         int
	dependencies []dependency
}

// ParseJob interprets the memory field and the run options and command words.
type ParseJob func(name, memory string, words []string) (lsf.Job, error)

// Parse validates the entire input before any jobs can be submitted.
func Parse(input io.Reader, parseJob ParseJob) ([]Entry, error) {
	reader := bufio.NewReader(input)
	var entries []Entry
	seen := make(map[string]bool)
	for lineNumber := 1; ; lineNumber++ {
		line, readErr := reader.ReadString('\n')
		if readErr != nil && readErr != io.EOF {
			return nil, fmt.Errorf("read batch: %w", readErr)
		}
		line = strings.TrimLeft(strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r"), " \t")
		if strings.TrimSpace(line) != "" && !strings.HasPrefix(line, "#") {
			entry, err := parseLine(line, lineNumber, seen, parseJob)
			if err != nil {
				return nil, fmt.Errorf("batch line %d: %w", lineNumber, err)
			}
			entries = append(entries, entry)
			seen[entry.Job.Name] = true
		}
		if readErr == io.EOF {
			break
		}
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("batch contains no jobs")
	}
	return entries, nil
}

func parseLine(line string, number int, seen map[string]bool, parseJob ParseJob) (Entry, error) {
	var fields [3]string
	for i := range fields {
		end := strings.IndexAny(line, " \t")
		if end < 0 {
			return Entry{}, fmt.Errorf("expected <name> <depends-on> <memory-gb> [run options] <command>")
		}
		fields[i], line = line[:end], strings.TrimLeft(line[end:], " \t")
	}
	name := fields[0]
	if name == "." || strings.ContainsAny(name, ",:") {
		return Entry{}, fmt.Errorf("job name %q must not be '.' or contain ',' or ':'", name)
	}
	if seen[name] {
		return Entry{}, fmt.Errorf("duplicate job name %q", name)
	}
	entry := Entry{line: number}
	if fields[1] != "." {
		for _, value := range strings.Split(fields[1], ",") {
			dep := dependency{name: value}
			if strings.HasPrefix(value, "ended:") {
				dep.name, dep.ended = strings.TrimPrefix(value, "ended:"), true
			}
			if !seen[dep.name] {
				return Entry{}, fmt.Errorf("dependency %q must name an earlier job", value)
			}
			entry.dependencies = append(entry.dependencies, dep)
		}
	}
	words, err := splitWords(line)
	if err != nil {
		return Entry{}, err
	}
	entry.Job, err = parseJob(name, fields[2], words)
	if err != nil {
		return Entry{}, err
	}
	// Include named dependencies during preflight, so all bsub arguments are checked.
	if _, err := entry.withDependencies(nil).Args(); err != nil {
		return Entry{}, err
	}
	return entry, nil
}

func (entry Entry) withDependencies(ids map[string]string) lsf.Job {
	job := entry.Job
	job.Done = append([]string(nil), job.Done...)
	job.Ended = append([]string(nil), job.Ended...)
	for _, dep := range entry.dependencies {
		value := dep.name
		if ids != nil {
			value = ids[dep.name]
		}
		if dep.ended {
			job.Ended = append(job.Ended, value)
		} else {
			job.Done = append(job.Done, value)
		}
	}
	return job
}

// Submit submits entries in order and uses their returned IDs for dependencies.
// Preview output uses job names because no IDs have been allocated yet.
func Submit(entries []Entry, output io.Writer, preview bool) error {
	ids := make(map[string]string)
	for _, entry := range entries {
		var job lsf.Job
		if preview {
			job = entry.withDependencies(nil)
		} else {
			job = entry.withDependencies(ids)
		}
		args, err := job.Args()
		if err == nil {
			_, err = fmt.Fprintln(output, lsf.CommandString(args))
		}
		if err == nil && !preview {
			var id string
			id, err = lsf.SubmitArgs(args)
			if err == nil {
				ids[job.Name] = id
				_, err = fmt.Fprintf(output, "%s %s submitted\n", id, job.Name)
			}
		}
		if err != nil {
			return fmt.Errorf("batch line %d (%s), stopped after %d submissions: %w", entry.line, job.Name, len(ids), err)
		}
	}
	return nil
}
