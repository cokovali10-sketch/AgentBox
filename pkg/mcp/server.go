package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/cokovali10-sketch/AgentBox/pkg/policy"
	"github.com/cokovali10-sketch/AgentBox/pkg/sandbox"
)

const ProtocolVersion = "2024-11-05"

type Server struct {
	manager *sandbox.Manager
	reader  io.Reader
	writer  io.Writer
}

func NewServer(manager *sandbox.Manager, r io.Reader, w io.Writer) *Server {
	if r == nil {
		r = os.Stdin
	}
	if w == nil {
		w = os.Stdout
	}
	return &Server{
		manager: manager,
		reader:  r,
		writer:  w,
	}
}

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id"`
	Result  any    `json:"result,omitempty"`
	Error   *rpcErr `json:"error,omitempty"`
}

type rpcErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (s *Server) Serve() error {
	scanner := bufio.NewScanner(s.reader)
	// Buffer up to 10MB input lines for large code payloads
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var req jsonRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			s.sendError(nil, -32700, "Parse error: "+err.Error())
			continue
		}

		resp := s.handleRequest(req)
		if resp != nil {
			bytes, err := json.Marshal(resp)
			if err == nil {
				_, _ = fmt.Fprintln(s.writer, string(bytes))
			}
		}
	}

	return scanner.Err()
}

func (s *Server) handleRequest(req jsonRPCRequest) *jsonRPCResponse {
	switch req.Method {
	case "initialize":
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"protocolVersion": ProtocolVersion,
				"serverInfo": map[string]string{
					"name":    "agentbox-mcp",
					"version": "1.0.0",
				},
				"capabilities": map[string]any{
					"tools": map[string]any{},
				},
			},
		}

	case "notifications/initialized":
		return nil

	case "tools/list":
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"tools": s.getToolDefinitions(),
			},
		}

	case "tools/call":
		var callParams struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &callParams); err != nil {
			return s.errorResponse(req.ID, -32602, "Invalid params: "+err.Error())
		}

		result, err := s.callTool(callParams.Name, callParams.Arguments)
		if err != nil {
			return s.errorResponse(req.ID, -32000, err.Error())
		}

		resultJSON, _ := json.MarshalIndent(result, "", "  ")
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"content": []map[string]any{
					{
						"type": "text",
						"text": string(resultJSON),
					},
				},
			},
		}

	default:
		return s.errorResponse(req.ID, -32601, "Method not found: "+req.Method)
	}
}

func (s *Server) getToolDefinitions() []map[string]any {
	return []map[string]any{
		{
			"name":        "agentbox.run_code",
			"description": "Execute code snippet (Python, Go, Node.js, Shell, PowerShell) in a secure, isolated micro-sandbox with strict timeout and memory limits.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"language": map[string]any{
						"type":        "string",
						"description": "Programming language: python, node, go, bash, powershell",
					},
					"code": map[string]any{
						"type":        "string",
						"description": "Source code content to execute in the sandbox",
					},
					"files": map[string]any{
						"type":        "object",
						"description": "Optional mapping of relative file paths to string contents to pre-populate in the jail",
					},
					"timeout_sec": map[string]any{
						"type":        "integer",
						"description": "Execution timeout in seconds (default: 10)",
					},
				},
				"required": []string{"language", "code"},
			},
		},
		{
			"name":        "agentbox.exec_command",
			"description": "Execute a terminal command inside the isolated micro-sandbox with policy filtering.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"command": map[string]any{
						"type":        "string",
						"description": "Command string to execute (e.g., 'pytest', 'ls', 'go test')",
					},
					"files": map[string]any{
						"type":        "object",
						"description": "Optional mapping of file paths to content",
					},
					"timeout_sec": map[string]any{
						"type":        "integer",
						"description": "Execution timeout in seconds (default: 10)",
					},
				},
				"required": []string{"command"},
			},
		},
		{
			"name":        "agentbox.doctor",
			"description": "Inspect available language runtimes, compilers, and sandbox capabilities on the host system.",
			"inputSchema": map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			"name":        "agentbox.bench",
			"description": "Benchmark the micro-sandbox spin-up latency and round-trip execution performance.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"repeats": map[string]any{
						"type":        "integer",
						"description": "Number of executions to average (default: 5)",
					},
				},
			},
		},
	}
}

func (s *Server) CallToolDirect(name string, args json.RawMessage) (any, error) {
	return s.callTool(name, args)
}

func (s *Server) callTool(name string, args json.RawMessage) (any, error) {
	ctx := context.Background()

	switch name {
	case "agentbox.run_code":
		var p struct {
			Language   string            `json:"language"`
			Code       string            `json:"code"`
			Files      map[string]string `json:"files"`
			TimeoutSec int               `json:"timeout_sec"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, err
		}

		pol := policy.DefaultPolicy()
		if p.TimeoutSec > 0 {
			pol.Timeout = time.Duration(p.TimeoutSec) * time.Second
		}

		req := sandbox.ExecutionRequest{
			Language: sandbox.Language(p.Language),
			Code:     p.Code,
			Files:    p.Files,
			Policy:   &pol,
		}

		return s.manager.Execute(ctx, req)

	case "agentbox.exec_command":
		var p struct {
			Command    string            `json:"command"`
			Files      map[string]string `json:"files"`
			TimeoutSec int               `json:"timeout_sec"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, err
		}

		pol := policy.DefaultPolicy()
		if p.TimeoutSec > 0 {
			pol.Timeout = time.Duration(p.TimeoutSec) * time.Second
		}

		req := sandbox.ExecutionRequest{
			Language: sandbox.LangCommand,
			Command:  p.Command,
			Files:    p.Files,
			Policy:   &pol,
		}

		return s.manager.Execute(ctx, req)

	case "agentbox.doctor":
		return s.runDoctor(), nil

	case "agentbox.bench":
		var p struct {
			Repeats int `json:"repeats"`
		}
		_ = json.Unmarshal(args, &p)
		if p.Repeats <= 0 {
			p.Repeats = 5
		}
		return s.runBenchmark(p.Repeats), nil

	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

func (s *Server) runDoctor() map[string]any {
	checkRuntime := func(bin string, verArgs ...string) map[string]any {
		path, err := exec.LookPath(bin)
		if err != nil {
			return map[string]any{"available": false}
		}
		info := map[string]any{"available": true, "path": path}
		if len(verArgs) > 0 {
			out, err := exec.Command(bin, verArgs...).CombinedOutput()
			if err == nil {
				firstLine := strings.Split(strings.TrimSpace(string(out)), "\n")[0]
				info["version"] = firstLine
			}
		}
		return info
	}

	return map[string]any{
		"runtimes": map[string]any{
			"python":     checkRuntime("python", "--version"),
			"python3":    checkRuntime("python3", "--version"),
			"node":       checkRuntime("node", "--version"),
			"go":         checkRuntime("go", "version"),
			"powershell": checkRuntime("powershell", "-Command", "$PSVersionTable.PSVersion.ToString()"),
			"bash":       checkRuntime("bash", "--version"),
		},
		"sandbox": map[string]any{
			"isolation": "ephemeral-jail + process-group + policy-filtering",
			"status":    "ready",
		},
	}
}

func (s *Server) runBenchmark(repeats int) map[string]any {
	ctx := context.Background()
	var totalStartupMicros int64
	var totalDurationMicros int64

	code := "print('bench-ok')"
	for i := 0; i < repeats; i++ {
		res, err := s.manager.Execute(ctx, sandbox.ExecutionRequest{
			Language: sandbox.LangPython,
			Code:     code,
		})
		if err == nil && res != nil {
			totalStartupMicros += res.Metrics.StartupTimeMicros
			totalDurationMicros += res.Duration.Microseconds()
		}
	}

	avgStartupMs := float64(totalStartupMicros) / float64(repeats) / 1000.0
	avgDurationMs := float64(totalDurationMicros) / float64(repeats) / 1000.0

	return map[string]any{
		"repeats":             repeats,
		"avg_startup_time_ms": avgStartupMs,
		"avg_total_time_ms":   avgDurationMs,
		"sub_10ms_capable":    avgStartupMs < 10.0,
	}
}

func (s *Server) errorResponse(id any, code int, msg string) *jsonRPCResponse {
	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &rpcErr{
			Code:    code,
			Message: msg,
		},
	}
}

func (s *Server) sendError(id any, code int, msg string) {
	resp := s.errorResponse(id, code, msg)
	bytes, _ := json.Marshal(resp)
	_, _ = fmt.Fprintln(s.writer, string(bytes))
}
