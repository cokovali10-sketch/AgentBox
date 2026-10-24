//go:build windows

package sandbox

import (
	"os/exec"
	"syscall"

	"github.com/cokovali10-sketch/AgentBox/pkg/policy"
)

// attachPlatformLimits configures Windows-specific process isolation flags.
func attachPlatformLimits(cmd *exec.Cmd, p policy.Policy) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
		HideWindow:    true,
	}
}
