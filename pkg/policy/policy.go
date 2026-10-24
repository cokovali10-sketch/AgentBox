package policy

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Policy defines the execution boundaries and security constraints for a sandbox run.
type Policy struct {
	MaxMemoryMB     int64         `json:"max_memory_mb"`
	MaxCPUTimeSec   int           `json:"max_cpu_time_sec"`
	Timeout         time.Duration `json:"timeout"`
	AllowNetwork    bool          `json:"allow_network"`
	MaxOutputBytes  int64         `json:"max_output_bytes"`
	BlockedCommands []string      `json:"blocked_commands"`
	ReadOnlyPaths   []string      `json:"read_only_paths"`
}

// DefaultPolicy returns strict default limits suitable for untrusted AI-generated code.
func DefaultPolicy() Policy {
	return Policy{
		MaxMemoryMB:    256,
		MaxCPUTimeSec:  5,
		Timeout:        10 * time.Second,
		AllowNetwork:   false,
		MaxOutputBytes: 1024 * 1024, // 1MB max stdout/stderr
		BlockedCommands: []string{
			"rm -rf /",
			"rm -rf /*",
			":(){ :|:& };:", // fork bomb
			"mkfs",
			"dd if=/dev",
			"format ",
			"shutdown",
			"reboot",
			"bcdedit",
			"reg delete",
			"net user",
		},
		ReadOnlyPaths: []string{},
	}
}

var destructivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\brm\s+-[rf]{1,2}\s+([/\\]|\*|~)`),
	regexp.MustCompile(`:\(\)\s*\{\s*:\|:&\s*\};:`),
	regexp.MustCompile(`(?i)\bformat\s+[a-z]:`),
	regexp.MustCompile(`(?i)\b(reg|bcdedit|vssadmin)\s+(delete|delete\s+shadows)`),
}

// ValidateCommand inspects a command against the security policy before execution.
func (p *Policy) ValidateCommand(cmd string) error {
	trimmed := strings.TrimSpace(cmd)
	if trimmed == "" {
		return errors.New("empty command is not allowed")
	}

	for _, pattern := range destructivePatterns {
		if pattern.MatchString(trimmed) {
			return fmt.Errorf("security policy violation: command matches dangerous pattern '%s'", pattern.String())
		}
	}

	for _, blocked := range p.BlockedCommands {
		if strings.Contains(strings.ToLower(trimmed), strings.ToLower(blocked)) {
			return fmt.Errorf("security policy violation: command contains blocked phrase '%s'", blocked)
		}
	}

	return nil
}
