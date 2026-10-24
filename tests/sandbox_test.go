package tests

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cokovali10-sketch/AgentBox/pkg/policy"
	"github.com/cokovali10-sketch/AgentBox/pkg/sandbox"
)

func TestPythonExecution(t *testing.T) {
	mgr, err := sandbox.NewManager("", nil)
	if err != nil {
		t.Fatalf("failed to create sandbox manager: %v", err)
	}

	resp, err := mgr.Execute(context.Background(), sandbox.ExecutionRequest{
		Language: sandbox.LangPython,
		Code:     "print('agentbox-test-ok')",
	})
	if err != nil {
		t.Fatalf("execution error: %v", err)
	}

	if resp.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d. Stderr: %s", resp.ExitCode, resp.Stderr)
	}

	if !strings.Contains(resp.Stdout, "agentbox-test-ok") {
		t.Errorf("expected output to contain 'agentbox-test-ok', got: %s", resp.Stdout)
	}

	if resp.Metrics.StartupTimeMicros > 50000 { // 50ms threshold
		t.Logf("Notice: startup time was %d us", resp.Metrics.StartupTimeMicros)
	}
}

func TestTimeoutEnforcement(t *testing.T) {
	mgr, err := sandbox.NewManager("", nil)
	if err != nil {
		t.Fatalf("failed to create sandbox manager: %v", err)
	}

	pol := policy.DefaultPolicy()
	pol.Timeout = 1 * time.Second

	resp, err := mgr.Execute(context.Background(), sandbox.ExecutionRequest{
		Language: sandbox.LangPython,
		Code:     "import time\ntime.sleep(5)",
		Policy:   &pol,
	})
	if err != nil {
		t.Fatalf("unexpected manager error: %v", err)
	}

	if resp.ExitCode != 124 {
		t.Errorf("expected exit code 124 (timeout), got: %d", resp.ExitCode)
	}
}

func TestFileInjection(t *testing.T) {
	mgr, err := sandbox.NewManager("", nil)
	if err != nil {
		t.Fatalf("failed to create sandbox manager: %v", err)
	}

	files := map[string]string{
		"data/input.txt": "42",
	}

	resp, err := mgr.Execute(context.Background(), sandbox.ExecutionRequest{
		Language: sandbox.LangPython,
		Code: `with open('data/input.txt') as f:
    print('read:', f.read().strip())
`,
		Files: files,
	})
	if err != nil {
		t.Fatalf("execution error: %v", err)
	}

	if !strings.Contains(resp.Stdout, "read: 42") {
		t.Errorf("expected output 'read: 42', got: %s", resp.Stdout)
	}
}
