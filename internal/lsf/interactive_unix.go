//go:build !windows

package lsf

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// ExecInteractiveArgs replaces this process with an interactive bsub command.
// It returns only if locating or executing bsub fails.
func ExecInteractiveArgs(args []string) error {
	bsubPath, err := exec.LookPath("bsub")
	if err != nil {
		return err
	}
	return syscall.Exec(bsubPath, append([]string{"bsub"}, args...), os.Environ())
}

// ExecInteractive validates the job and replaces this process with bsub.
func (j Job) ExecInteractive() error {
	if !j.Interactive {
		return fmt.Errorf("interactive execution requires an interactive job")
	}
	args, err := j.Args()
	if err != nil {
		return err
	}
	return ExecInteractiveArgs(args)
}
