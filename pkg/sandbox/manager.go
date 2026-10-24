package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/cokovali10-sketch/AgentBox/pkg/policy"
)

// Manager coordinates sandbox lifecycle, scratch file systems, and process sandboxing.
type Manager struct {
	baseDir string
	policy  policy.Policy
}

// NewManager creates a sandbox manager with a dedicated base temporary directory.
func NewManager(baseDir string, defaultPolicy *policy.Policy) (*Manager, error) {
	if baseDir == "" {
		baseDir = filepath.Join(os.TempDir(), "agentbox_sandboxes")
	}
	if err := os.MkdirAll(baseDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create sandbox base dir: %w", err)
	}

	p := policy.DefaultPolicy()
	if defaultPolicy != nil {
		p = *defaultPolicy
	}

	return &Manager{
		baseDir: baseDir,
		policy:  p,
	}, nil
}

// Execute runs the code or command in a fresh, isolated ephemeral jail.
func (m *Manager) Execute(ctx context.Context, req ExecutionRequest) (*ExecutionResponse, error) {
	start := time.Now()

	// Apply policy override if provided
	pol := m.policy
	if req.Policy != nil {
		pol = *req.Policy
	}

	// 1. Setup isolated ephemeral jail directory
	setupStart := time.Now()
	jailDir, err := os.MkdirTemp(m.baseDir, "box_*")
	if err != nil {
		return nil, fmt.Errorf("failed to create jail dir: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(jailDir) // Auto-cleanup ephemeral environment
	}()

	// 2. Inject files into jail
	fileCount := 0
	for path, content := range req.Files {
		cleanPath := filepath.Clean(path)
		if strings.HasPrefix(cleanPath, "..") || filepath.IsAbs(cleanPath) {
			continue // Prevent directory traversal
		}
		targetPath := filepath.Join(jailDir, cleanPath)
		if err := os.MkdirAll(filepath.Dir(targetPath), 0700); err != nil {
			return nil, fmt.Errorf("failed to create directory for %s: %w", path, err)
		}
		if err := os.WriteFile(targetPath, []byte(content), 0600); err != nil {
			return nil, fmt.Errorf("failed to write isolated file %s: %w", path, err)
		}
		fileCount++
	}

	// 3. Prepare command & script
	cmdName, cmdArgs, err := m.resolveCommand(jailDir, req)
	if err != nil {
		return nil, err
	}

	// Validate security policy against the command
	fullCmdString := cmdName + " " + strings.Join(cmdArgs, " ")
	if err := pol.ValidateCommand(fullCmdString); err != nil {
		return &ExecutionResponse{
			ExitCode: 126, // Command invoked cannot execute
			Stderr:   err.Error(),
			Duration: time.Since(start),
			Error:    err.Error(),
		}, nil
	}

	startupMicros := time.Since(setupStart).Microseconds()

	// 4. Configure execution context with timeout
	timeout := pol.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, cmdName, cmdArgs...)
	cmd.Dir = jailDir

	// Environment isolation: strip dangerous host environment variables
	cmd.Env = m.buildSafeEnvironment(jailDir, req.Env, pol)

	// Apply platform-specific job/cgroup limits
	attachPlatformLimits(cmd, pol)

	var stdoutBuf, stderrBuf bytes.Buffer
	var stdoutWriter io.Writer = &stdoutBuf
	var stderrWriter io.Writer = &stderrBuf

	if pol.MaxOutputBytes > 0 {
		stdoutWriter = &limitWriter{w: &stdoutBuf, remaining: pol.MaxOutputBytes}
		stderrWriter = &limitWriter{w: &stderrBuf, remaining: pol.MaxOutputBytes}
	}

	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderrWriter

	// 5. Run process
	execStart := time.Now()
	runErr := cmd.Run()
	execMicros := time.Since(execStart).Microseconds()

	exitCode := 0
	var errMsg string
	if runErr != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			exitCode = 124 // Standard timeout exit code
			errMsg = fmt.Sprintf("execution timed out after %s", timeout)
		} else if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
			errMsg = runErr.Error()
		}
	}

	return &ExecutionResponse{
		ExitCode: exitCode,
		Stdout:   stdoutBuf.String(),
		Stderr:   stderrBuf.String(),
		Duration: time.Since(start),
		Metrics: Metrics{
			StartupTimeMicros: startupMicros,
			ExecTimeMicros:    execMicros,
			IsolatedFiles:     fileCount,
		},
		Error: errMsg,
	}, nil
}

func (m *Manager) resolveCommand(jailDir string, req ExecutionRequest) (string, []string, error) {
	lang := strings.ToLower(string(req.Language))

	switch lang {
	case "python", "py":
		scriptPath := filepath.Join(jailDir, "main.py")
		if err := os.WriteFile(scriptPath, []byte(req.Code), 0600); err != nil {
			return "", nil, err
		}
		// Prefer active python executable
		pyExe := "python"
		if runtime.GOOS != "windows" {
			pyExe = "python3"
		}
		return pyExe, []string{"-u", scriptPath}, nil

	case "node", "javascript", "js":
		scriptPath := filepath.Join(jailDir, "index.js")
		if err := os.WriteFile(scriptPath, []byte(req.Code), 0600); err != nil {
			return "", nil, err
		}
		return "node", []string{scriptPath}, nil

	case "go", "golang":
		mainPath := filepath.Join(jailDir, "main.go")
		code := req.Code
		if !strings.Contains(code, "package main") {
			code = "package main\n\nimport \"fmt\"\n\nfunc main() {\n" + code + "\n}\n"
		}
		if err := os.WriteFile(mainPath, []byte(code), 0600); err != nil {
			return "", nil, err
		}
		return "go", []string{"run", mainPath}, nil

	case "bash", "sh":
		scriptPath := filepath.Join(jailDir, "script.sh")
		if err := os.WriteFile(scriptPath, []byte(req.Code), 0700); err != nil {
			return "", nil, err
		}
		shBin := "bash"
		if _, err := exec.LookPath("bash"); err != nil {
			shBin = "sh"
		}
		return shBin, []string{scriptPath}, nil

	case "powershell", "ps1":
		scriptPath := filepath.Join(jailDir, "script.ps1")
		if err := os.WriteFile(scriptPath, []byte(req.Code), 0600); err != nil {
			return "", nil, err
		}
		return "powershell", []string{"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", scriptPath}, nil

	case "command", "cmd", "":
		if req.Command != "" {
			parts := strings.Fields(req.Command)
			if len(parts) == 0 {
				return "", nil, fmt.Errorf("empty command")
			}
			return parts[0], append(parts[1:], req.Args...), nil
		}
		return "", nil, fmt.Errorf("unsupported language or empty command: %s", req.Language)

	default:
		return "", nil, fmt.Errorf("unknown runtime language: %s", req.Language)
	}
}

func (m *Manager) buildSafeEnvironment(jailDir string, userEnv map[string]string, p policy.Policy) []string {
	safeVars := []string{
		"SYSTEMROOT=" + os.Getenv("SYSTEMROOT"),
		"WINDIR=" + os.Getenv("WINDIR"),
		"PATH=" + os.Getenv("PATH"),
		"TEMP=" + jailDir,
		"TMP=" + jailDir,
		"HOME=" + jailDir,
		"USERPROFILE=" + jailDir,
		"AGENTBOX_SANDBOX=1",
	}

	if !p.AllowNetwork {
		safeVars = append(safeVars, "HTTP_PROXY=http://127.0.0.1:0", "HTTPS_PROXY=http://127.0.0.1:0")
	}

	for k, v := range userEnv {
		safeVars = append(safeVars, fmt.Sprintf("%s=%s", k, v))
	}

	return safeVars
}

// limitWriter truncates output if it exceeds max allowed bytes.
type limitWriter struct {
	w         io.Writer
	remaining int64
}

func (l *limitWriter) Write(p []byte) (n int, err error) {
	if l.remaining <= 0 {
		return len(p), nil // drop bytes silently after limit
	}
	if int64(len(p)) > l.remaining {
		p = p[:l.remaining]
	}
	n, err = l.w.Write(p)
	l.remaining -= int64(n)
	return n, err
}
