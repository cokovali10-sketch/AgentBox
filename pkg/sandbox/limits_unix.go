//go:build !windows

package sandbox

import (
	"os/exec"
	"syscall"

	"github.com/cokovali10-sketch/AgentBox/pkg/policy"
)

// attachPlatformLimits configures Unix process group isolation.
func attachPlatformLimits(cmd *exec.Cmd, p policy.Policy) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}
