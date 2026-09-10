//go:build windows

package lsf

import "fmt"

// ExecInteractive is unavailable on Windows because LSF interactive jobs use
// Unix terminal semantics.
func (j Job) ExecInteractive() error {
	return fmt.Errorf("interactive LSF jobs are not supported on Windows")
}
