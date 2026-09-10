package root

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
