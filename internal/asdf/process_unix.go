//go:build unix

package asdf

import (
	"os/exec"
	"syscall"
	"time"
)

// configureProcessGroup starts asdf in its own process group so a cancel can
// terminate the whole tree of installer scripts it may have spawned.
func configureProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		pid := cmd.Process.Pid
		_ = syscall.Kill(-pid, syscall.SIGTERM)
		go func() {
			time.Sleep(3 * time.Second)
			_ = syscall.Kill(-pid, syscall.SIGKILL)
		}()
		return nil
	}
	cmd.WaitDelay = 4 * time.Second
}
