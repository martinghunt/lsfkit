package root

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func executeBatchInput(t *testing.T, input string, args ...string) (string, error) {
	t.Helper()
	t.Setenv("LSFKIT_LSF_MEMORY_UNITS", "MB")
	command := newRootCommand()
	command.SetArgs(append([]string{"batch"}, args...))
	command.SetIn(strings.NewReader(input))
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&output)
	err := command.Execute()
	return output.String(), err
}

func TestBatchPreview(t *testing.T) {
	input := "\n  # ignored\nfirst . 4 --threads 4 --tmp-space 10 --queue long --out 'first log.o' printf '%s\\n' 'hello world'\r\n" +
		"second first 2 bash -lc 'echo hello'\nthird first 2 echo third\nfourth second,ended:third 8 --done external echo fourth"
	output, err := executeBatchInput(t, input, "--norun", "-")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"-n 4", "tmp=10000", "-q long", "-o 'first log.o'", "-M 4000",
		`printf '%s\n' 'hello world'`, `bash -lc 'echo hello'`,
		`done("first")`, `done("external") && done("second") && ended("third")`,
	} {
		if !strings.Contains(output, want) {
			t.Errorf("missing %q in:\n%s", want, output)
		}
	}
	if strings.Count(output, "bsub ") != 4 || strings.Contains(output, "submitted") {
		t.Fatalf("unexpected preview:\n%s", output)
	}
}

func TestBatchMatchesRunOptions(t *testing.T) {
	tail := []string{"--memory-units", "MB", "--threads", "3", "--tmp-space", "6", "--queue", "long", "--err", "custom.e", "--tokens-name", "license", "--tokens-number", "2", "--start", "1", "--end", "3", "--array-limit", "2", "--ended", "external"}
	runArgs := append([]string{"run", "--norun"}, tail...)
	runArgs = append(runArgs, "1.5", "job", "echo", "INDEX", "--command-option")
	want, err := executeCommand(t, runArgs...)
	if err != nil {
		t.Fatal(err)
	}
	got, err := executeBatchInput(t, "job . 1.5 "+strings.Join(tail, " ")+" echo INDEX --command-option", "--norun")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("batch:\n%s\nrun:\n%s", got, want)
	}
}

func TestBatchInvalidInput(t *testing.T) {
	tests := []struct{ name, input, want string }{
		{"empty", " # comment\n\n", "no jobs"},
		{"missing fields", "first . 1", "expected <name>"},
		{"missing command", "first . 1   ", "no command"},
		{"duplicate", "first . 1 echo ok\nfirst . 1 echo bad", "duplicate"},
		{"unknown", "first missing 1 echo bad", "earlier job"},
		{"forward", "first later 1 echo bad\nlater . 1 echo ok", "earlier job"},
		{"self", "first first 1 echo bad", "earlier job"},
		{"empty dependency", "first . 1 echo ok\nsecond first, 1 echo bad", "earlier job"},
		{"ended unknown", "first ended:missing 1 echo bad", "earlier job"},
		{"reserved name", "bad:name . 1 echo bad", "job name"},
		{"negative memory", "first . -1 echo bad", "memory"},
		{"bad memory", "first . wrong echo bad", "memory"},
		{"infinite memory", "first . Inf echo bad", "finite"},
		{"NaN memory", "first . NaN echo bad", "finite"},
		{"bad flag", "first . 1 --bad echo bad", "unknown flag"},
		{"bad threads", "first . 1 --threads 0 echo bad", "threads"},
		{"bad logs", "first . 1 --out /nonexistent-lsfkit-batch-dir/log echo bad", "log directory"},
		{"bad array", "first . 1 --start 1 echo bad", "--start and --end"},
		{"interactive", "first . 1 --interactive echo bad", "interactive"},
		{"row preview", "first . 1 --norun echo bad", "batch --norun"},
		{"missing command after flags", "first . 1 --threads 2", "no command"},
		{"unclosed quote", "first . 1 echo 'bad", "unterminated"},
		{"trailing escape", "first . 1 echo \\", "trailing escape"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			output, err := executeBatchInput(t, test.input, "--norun")
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
			if strings.Contains(output, "bsub ") {
				t.Fatalf("printed a job before validation completed: %s", output)
			}
		})
	}
}

func TestBatchFileAndLongLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jobs.txt")
	word := strings.Repeat("x", 100000)
	if err := os.WriteFile(path, []byte("job . 1 echo "+word), 0600); err != nil {
		t.Fatal(err)
	}
	output, err := executeBatchInput(t, "", "--norun", path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, word) {
		t.Fatal("long command was truncated")
	}
	if _, err := executeBatchInput(t, "", path+".missing"); err == nil {
		t.Fatal("missing file accepted")
	}
}

// fakeBatchBsub records complete argv vectors and returns distinct job IDs.
func fakeBatchBsub(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	script := `#!/bin/sh
count=0
if [ -f "$BATCH_TEST_DIR/count" ]; then read count < "$BATCH_TEST_DIR/count"; fi
count=$((count + 1))
printf '%s\n' "$count" > "$BATCH_TEST_DIR/count"
printf '%s\n' "$@" > "$BATCH_TEST_DIR/args.$count"
if [ "$count" = "$BATCH_TEST_FAIL" ]; then echo 'submission rejected'; exit 1; fi
printf 'Job <%s> is submitted to queue <normal>.\n' "$((100 + count))"
`
	if err := os.WriteFile(filepath.Join(directory, "bsub"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("BATCH_TEST_DIR", directory)
	t.Setenv("BATCH_TEST_FAIL", "")
	return directory
}

func TestBatchSubmitsWithJobIDs(t *testing.T) {
	directory := fakeBatchBsub(t)
	input := "first . 4 echo one\nsecond first 2 echo two\nthird ended:first 2 echo three\nfourth second,ended:third 8 --done external echo four\n"
	output, err := executeBatchInput(t, input)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []string{"", "done(101)", "ended(101)", `done("external") && done(102) && ended(103)`} {
		argv, err := os.ReadFile(filepath.Join(directory, "args."+strconv.Itoa(i+1)))
		if err != nil {
			t.Fatal(err)
		}
		if want != "" && !strings.Contains(string(argv), "\n-w\n"+want+"\n") {
			t.Fatalf("wrong dependency in %s", argv)
		}
	}
	for _, want := range []string{"101 first submitted", "102 second submitted", "103 third submitted", "104 fourth submitted"} {
		if !strings.Contains(output, want) {
			t.Errorf("missing %q in %s", want, output)
		}
	}
}

func TestBatchValidatesBeforeAnySubmission(t *testing.T) {
	directory := fakeBatchBsub(t)
	_, err := executeBatchInput(t, "first . 1 echo ok\nsecond first 1 --threads 0 echo bad")
	if err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(directory, "count")); !os.IsNotExist(err) {
		t.Fatalf("bsub ran before validation: %v", err)
	}
}

func TestBatchStopsOnSubmissionFailure(t *testing.T) {
	directory := fakeBatchBsub(t)
	t.Setenv("BATCH_TEST_FAIL", "2")
	output, err := executeBatchInput(t, "first . 1 echo ok\nsecond first 1 echo fail\nthird second 1 echo skipped")
	if err == nil || !strings.Contains(err.Error(), "line 2 (second), stopped after 1 submissions") || !strings.Contains(err.Error(), "submission rejected") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "101 first submitted") {
		t.Fatalf("missing submitted job ID: %s", output)
	}
	if _, err := os.Stat(filepath.Join(directory, "args.3")); !os.IsNotExist(err) {
		t.Fatalf("continued after failure: %v", err)
	}
}

func TestBatchPreviewDoesNotSubmit(t *testing.T) {
	directory := fakeBatchBsub(t)
	_, err := executeBatchInput(t, "first . 1 echo ok\nsecond first 2 echo ok", "--norun")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(directory, "count")); !os.IsNotExist(err) {
		t.Fatalf("preview submitted jobs: %v", err)
	}
}

func TestBatchPreservesEscapedTrailingSpace(t *testing.T) {
	output, err := executeBatchInput(t, "job . 1 echo trailing\\ \n", "--norun")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "echo 'trailing '") {
		t.Fatalf("trailing space changed: %s", output)
	}
}
