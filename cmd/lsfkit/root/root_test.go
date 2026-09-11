package root

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/martinghunt/lsfkit/internal/buildinfo"
)

func executeCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var output bytes.Buffer
	command := newRootCommand()
	command.SetArgs(args)
	command.SetOut(&output)
	command.SetErr(&output)
	err := command.Execute()
	return output.String(), err
}

func TestRunNorunPreservesCommandArguments(t *testing.T) {
	output, err := executeCommand(
		t,
		"run", "--norun", "--memory-units", "MB",
		"1", "job'name", "printf", "%s\\n", "hello world",
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`-J 'job'"'"'name'`,
		`printf '%s\n' 'hello world'`,
	} {
		if !strings.Contains(output, want) {
			t.Errorf("preview does not contain %q:\n%s", want, output)
		}
	}
}

func TestVersionOutput(t *testing.T) {
	oldVersion := buildinfo.Version
	t.Cleanup(func() { buildinfo.Version = oldVersion })
	buildinfo.Version = "v1.2.3"

	output, err := executeCommand(t, "--version")
	if err != nil {
		t.Fatal(err)
	}
	if output != "lsfkit v1.2.3\n" {
		t.Fatalf("version output = %q", output)
	}
}

func TestRunAllowsFlagsInSubmittedCommand(t *testing.T) {
	output, err := executeCommand(
		t,
		"run", "--norun", "--memory-units", "MB",
		"1", "shell", "bash", "-lc", "echo hello",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, `bash -lc 'echo hello'`) {
		t.Fatalf("command flags or arguments changed:\n%s", output)
	}
}

func TestRunRejectsInteractiveCheckpointingBeforeSubmission(t *testing.T) {
	output, err := executeCommand(
		t,
		"run", "--norun", "--interactive", "--checkpoint", "--memory-units", "MB",
		"1", "shell", "bash",
	)
	if err == nil || !strings.Contains(err.Error(), "cannot be used with checkpointing") {
		t.Fatalf("output=%q err=%v", output, err)
	}
}

func TestOstatsSummary(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "job.o")
	contents := "Sender: LSF System <host>\nSuccessfully completed.\n"
	if err := os.WriteFile(filename, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	output, err := executeCommand(t, "ostats", "--summary", filename)
	if err != nil {
		t.Fatal(err)
	}
	if output != "exit_code\tcount\n0\t1\n" {
		t.Fatalf("unexpected summary:\n%s", output)
	}
}

func TestOstatsIncludesFilesWithoutLSFData(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "unlogged.o")
	if err := os.WriteFile(filename, []byte("application output only\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	output, err := executeCommand(t, "ostats", "--include-no-data", filename)
	if err != nil {
		t.Fatal(err)
	}
	want := "number_in_file\texit_code\tcpu_time\twall_clock_time\tmax_memory\trequested_memory\tfilename\n" +
		"1\t*\t*\t*\t*\t*\t" + filename + "\n"
	if output != want {
		t.Fatalf("unexpected output:\n%s", output)
	}
}

func TestOstatsNumbersReportsWithinFile(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "repeated.o")
	contents := "Sender: LSF System <first>\nSuccessfully completed.\n" +
		"Sender: LSF System <second>\nExited with exit code 2.\n"
	if err := os.WriteFile(filename, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	output, err := executeCommand(t, "ostats", filename)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3:\n%s", len(lines), output)
	}
	if !strings.HasPrefix(lines[1], "1\t0\t") || !strings.HasPrefix(lines[2], "2\t2\t") {
		t.Fatalf("reports were not numbered:\n%s", output)
	}
}

func TestOstatsRejectsBadTimeUnits(t *testing.T) {
	_, err := executeCommand(t, "ostats", "--time-units", "days", "job.o")
	if err == nil || !strings.Contains(err.Error(), "must be s, m, or h") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOstatsFailsAndOutputFile(t *testing.T) {
	tempDir := t.TempDir()
	input := filepath.Join(tempDir, "jobs.o")
	output := filepath.Join(tempDir, "stats.tsv")
	contents := `Sender: LSF System <first>
Successfully completed.
Sender: LSF System <second>
Exited with exit code 2.
`
	if err := os.WriteFile(input, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := executeCommand(t, "ostats", "--fails", "--outfile", output, input); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "\n0\t") || !strings.Contains(string(got), "\n2\t") {
		t.Fatalf("unexpected failed-job output:\n%s", got)
	}
}
