//go:build windows

package cli

import (
	"errors"
	"os/exec"
)

func runPTYCommand(cmd *exec.Cmd) (int, error) {
	return 1, errors.New("interactive ECS exec sessions are not supported on Windows")
}
