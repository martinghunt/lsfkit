package lsf

import (
	"slices"
	"strings"
	"testing"
)

func TestJobString(t *testing.T) {
	j := Job{
		Name:        "a very long job name",
		Out:         "out",
		Err:         "err",
		MemoryGB:    1.5,
		Command:     "echo INDEX",
		MemoryUnits: "MB",
		Threads:     2,
		ArrayStart:  1,
		ArrayEnd:    3,
		ArrayLimit:  2,
		Done:        []string{"42", "another job"},
	}
	got, e := j.String()
	if e != nil {
		t.Fatal(e)
	}
	for _, want := range []string{"-J 'a very long job name[1-3]%2'", "-M 1500", "echo \\$LSB_JOBINDEX", "done(42) && done(\"another job\")"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q missing from %s", want, got)
		}
	}
}

func TestJobValidation(t *testing.T) {
	jobs := []Job{
		{Name: "x", Command: "echo", MemoryUnits: "bad"},
		{Name: "x", Command: "", MemoryUnits: "MB"},
		{Name: "x", Command: "x", MemoryUnits: "MB", ArrayStart: 3, ArrayEnd: 2},
	}
	for _, j := range jobs {
		if _, e := j.String(); e == nil {
			t.Errorf("expected error for %#v", j)
		}
	}
}

func TestInteractiveJobString(t *testing.T) {
	job := Job{
		Name:        "interactive-shell",
		Command:     "bash",
		MemoryGB:    1,
		MemoryUnits: "MB",
		Interactive: true,
	}
	got, err := job.String()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "bsub -Is") {
		t.Fatalf("interactive job did not include bsub -Is: %s", got)
	}
	if strings.Contains(got, "-o '") || strings.Contains(got, "-e '") {
		t.Fatalf("interactive job unexpectedly redirected terminal output: %s", got)
	}
}

func TestInteractiveJobHonoursExplicitOutputFiles(t *testing.T) {
	job := Job{
		Name:        "interactive-shell",
		Out:         "interactive.out",
		Err:         "interactive.err",
		Command:     "bash",
		MemoryGB:    1,
		MemoryUnits: "MB",
		Interactive: true,
	}
	got, err := job.String()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "-o 'interactive.out' -e 'interactive.err'") {
		t.Fatalf("interactive job ignored explicit output files: %s", got)
	}
}

func TestInteractiveArgsPreserveCommandWords(t *testing.T) {
	job := Job{
		Name:        "shell",
		Command:     "bash -lc echo hello",
		CommandArgs: []string{"bash", "-lc", "echo hello"},
		MemoryGB:    1,
		MemoryUnits: "MB",
		Interactive: true,
	}
	args, err := job.InteractiveArgs()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"bash", "-lc", "echo hello"}
	if got := args[len(args)-len(want):]; !slices.Equal(got, want) {
		t.Fatalf("command words changed: got %q, want %q", got, want)
	}
}
