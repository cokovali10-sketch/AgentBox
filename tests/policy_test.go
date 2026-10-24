package tests

import (
	"testing"

	"github.com/cokovali10-sketch/AgentBox/pkg/policy"
)

func TestPolicyValidation(t *testing.T) {
	pol := policy.DefaultPolicy()

	// Safe commands
	safeCmds := []string{
		"python main.py",
		"go run .",
		"node index.js",
		"ls -la",
		"dir",
		"echo hello",
	}

	for _, cmd := range safeCmds {
		if err := pol.ValidateCommand(cmd); err != nil {
			t.Errorf("expected '%s' to be valid, got: %v", cmd, err)
		}
	}

	// Dangerous commands
	dangerousCmds := []string{
		"rm -rf /",
		"rm -rf /*",
		":(){ :|:& };:",
		"format c:",
		"reg delete HKLM\\Software",
	}

	for _, cmd := range dangerousCmds {
		if err := pol.ValidateCommand(cmd); err == nil {
			t.Errorf("expected '%s' to be blocked by policy", cmd)
		}
	}
}
