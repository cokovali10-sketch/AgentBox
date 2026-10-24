package tests

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cokovali10-sketch/AgentBox/pkg/mcp"
	"github.com/cokovali10-sketch/AgentBox/pkg/sandbox"
)

func TestMCPInitializeAndList(t *testing.T) {
	mgr, _ := sandbox.NewManager("", nil)

	input := `{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {}}
{"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": {}}
`
	var out bytes.Buffer
	server := mcp.NewServer(mgr, strings.NewReader(input), &out)

	if err := server.Serve(); err != nil {
		t.Fatalf("serve error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(lines))
	}

	var initResp map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &initResp); err != nil {
		t.Fatalf("failed to parse init response: %v", err)
	}
	res := initResp["result"].(map[string]any)
	if res["protocolVersion"] != mcp.ProtocolVersion {
		t.Errorf("expected protocol version %s, got %v", mcp.ProtocolVersion, res["protocolVersion"])
	}

	var listResp map[string]any
	if err := json.Unmarshal([]byte(lines[1]), &listResp); err != nil {
		t.Fatalf("failed to parse list response: %v", err)
	}
	tools := listResp["result"].(map[string]any)["tools"].([]any)
	if len(tools) < 3 {
		t.Errorf("expected at least 3 tools, got %d", len(tools))
	}
}
