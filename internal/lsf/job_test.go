package lsf

import (
	"os"
	"os/exec"
	"path/filepath"
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
		CommandArgs: []string{"echo", "INDEX"},
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
	for _, want := range []string{"-J 'a very long job name[1-3]%2'", "-M 1500", "echo '$LSB_JOBINDEX'", "done(42) && done(\"another job\")"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q missing from %s", want, got)
		}
	}
}

func TestJobValidation(t *testing.T) {
	jobs := []Job{
		{Name: "x", CommandArgs: []string{"echo"}, Threads: 1, MemoryUnits: "bad"},
		{Name: "x", Threads: 1, MemoryUnits: "MB"},
		{Name: "x", CommandArgs: []string{"x"}, Threads: 1, MemoryUnits: "MB", ArrayStart: 3, ArrayEnd: 2},
	}
	for _, j := range jobs {
		if _, e := j.String(); e == nil {
			t.Errorf("expected error for %#v", j)
		}
	}
}

func TestMemoryUnitsFromEnvironment(t *testing.T) {
	t.Setenv("LSFKIT_LSF_MEMORY_UNITS", "MB")

	units, err := (Job{}).memoryUnits()
	if err != nil {
		t.Fatal(err)
	}
	if units != "MB" {
		t.Fatalf("memory units = %q, want MB", units)
	}
}

func TestExplicitMemoryUnitsOverrideEnvironment(t *testing.T) {
	t.Setenv("LSFKIT_LSF_MEMORY_UNITS", "MB")

	units, err := (Job{MemoryUnits: "KB"}).memoryUnits()
	if err != nil {
		t.Fatal(err)
	}
	if units != "KB" {
		t.Fatalf("memory units = %q, want KB", units)
	}
}

func TestInteractiveJobString(t *testing.T) {
	job := Job{
		Name:        "interactive-shell",
		CommandArgs: []string{"bash"},
		MemoryGB:    1,
		MemoryUnits: "MB",
		Threads:     1,
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
		CommandArgs: []string{"bash"},
		MemoryGB:    1,
		MemoryUnits: "MB",
		Threads:     1,
		Interactive: true,
	}
	got, err := job.String()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "-o interactive.out -e interactive.err") {
		t.Fatalf("interactive job ignored explicit output files: %s", got)
	}
}

func TestInteractiveArgsPreserveCommandWords(t *testing.T) {
	job := Job{
		Name:        "shell",
		CommandArgs: []string{"bash", "-lc", "echo hello"},
		MemoryGB:    1,
		MemoryUnits: "MB",
		Threads:     1,
		Interactive: true,
	}
	args, err := job.Args()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"bash", "-lc", "echo hello"}
	if got := args[len(args)-len(want):]; !slices.Equal(got, want) {
		t.Fatalf("command words changed: got %q, want %q", got, want)
	}
}

func TestShellQuote(t *testing.T) {
	if got, want := shellQuote("job'name"), `'job'"'"'name'`; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestPreExecHomeCheckIsShellQuoted(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home dir with a ' quote")
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)

	job := Job{
		Name:        "x",
		CommandArgs: []string{"echo"},
		MemoryGB:    1,
		MemoryUnits: "MB",
		Threads:     1,
	}
	args, err := job.Args()
	if err != nil {
		t.Fatal(err)
	}

	want := "test -e " + shellQuote(home)
	found := false
	for i, a := range args {
		if a == "-E" && i+1 < len(args) {
			found = true
			if args[i+1] != want {
				t.Fatalf("-E value = %q, want %q", args[i+1], want)
			}
			// The value LSF will run through a shell on the execution host
			// must actually resolve to this home directory.
			if err := exec.Command("sh", "-c", args[i+1]).Run(); err != nil {
				t.Fatalf("sh -c %q failed: %v", args[i+1], err)
			}
		}
	}
	if !found {
		t.Fatal("-E flag not present in args")
	}
}

func TestSubmitPassesCommandArgumentsDirectly(t *testing.T) {
	tempDir := t.TempDir()
	bsub := filepath.Join(tempDir, "bsub")
	argsFile := filepath.Join(tempDir, "args")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$LSFKIT_TEST_ARGS\"\nprintf 'Job <42> is submitted to queue <normal>.\\n'\n"
	if err := os.WriteFile(bsub, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", tempDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("LSFKIT_TEST_ARGS", argsFile)

	job := Job{
		Name:        "argument-test",
		CommandArgs: []string{"printf", "%s\\n", "hello world"},
		MemoryGB:    1,
		MemoryUnits: "MB",
		Threads:     1,
	}
	id, err := job.Submit()
	if err != nil {
		t.Fatal(err)
	}
	if id != "42" {
		t.Fatalf("got job ID %q", id)
	}
	contents, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Split(strings.TrimSuffix(string(contents), "\n"), "\n")
	want := job.CommandArgs
	if command := got[len(got)-len(want):]; !slices.Equal(command, want) {
		t.Fatalf("command arguments changed: got %q, want %q", command, want)
	}
}
