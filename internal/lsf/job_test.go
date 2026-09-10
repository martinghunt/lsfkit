package lsf

import (
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
