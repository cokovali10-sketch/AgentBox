package sandbox

import (
	"time"

	"github.com/cokovali10-sketch/AgentBox/pkg/policy"
)

// Language defines supported runtime languages in the micro-sandbox.
type Language string

const (
	LangPython     Language = "python"
	LangJavaScript Language = "javascript"
	LangNode       Language = "node"
	LangGo         Language = "go"
	LangBash       Language = "bash"
	LangSh         Language = "sh"
	LangPowerShell Language = "powershell"
	LangCommand    Language = "command"
)

// ExecutionRequest specifies what code to execute and under what policy.
type ExecutionRequest struct {
	Language Language          `json:"language"`
	Code     string            `json:"code,omitempty"`
	Command  string            `json:"command,omitempty"`
	Args     []string          `json:"args,omitempty"`
	Env      map[string]string `json:"env,omitempty"`
	Files    map[string]string `json:"files,omitempty"` // path -> content to inject in jail
	Policy   *policy.Policy    `json:"policy,omitempty"`
}

// ExecutionResponse holds the execution outcome, stdout, stderr, and performance metrics.
type ExecutionResponse struct {
	ExitCode int           `json:"exit_code"`
	Stdout   string        `json:"stdout"`
	Stderr   string        `json:"stderr"`
	Duration time.Duration `json:"duration"`
	Metrics  Metrics       `json:"metrics"`
	Error    string        `json:"error,omitempty"`
}

// Metrics tracks sub-millisecond execution telemetries.
type Metrics struct {
	StartupTimeMicros int64 `json:"startup_time_us"` // Sandbox creation time in microseconds
	ExecTimeMicros    int64 `json:"exec_time_us"`    // Actual process execution time in microseconds
	PeakMemoryBytes   int64 `json:"peak_memory_bytes"`
	IsolatedFiles     int   `json:"isolated_files"`
}
