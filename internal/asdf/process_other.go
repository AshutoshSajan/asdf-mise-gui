//go:build !unix

package asdf

import "os/exec"

func configureProcessGroup(cmd *exec.Cmd) {
	cmd.WaitDelay = 4 * 1e9
}
