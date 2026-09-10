//go:build !windows

package lsf

import (
	"os"
	"os/exec"
	"syscall"
)

// ExecInteractive replaces this process with bsub for an interactive job.
// It only returns when preparing the command or exec itself fails.
func (j Job) ExecInteractive() error {
	args, err := j.InteractiveArgs()
	if err != nil {
		return err
	}
	bsubPath, err := exec.LookPath("bsub")
	if err != nil {
		return err
	}
	return syscall.Exec(bsubPath, append([]string{"bsub"}, args...), os.Environ())
}
