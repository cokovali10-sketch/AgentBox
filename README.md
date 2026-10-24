<div align="center">

# ⚡ AgentBox

**AI-Native Micro-Sandbox Runtime · Sub-Millisecond Spin-up (<1ms) · Native MCP Server**

*The lightweight, zero-overhead execution environment built specifically for LLM coding agents.*

[![CI](https://github.com/cokovali10-sketch/AgentBox/actions/workflows/ci.yml/badge.svg)](https://github.com/cokovali10-sketch/AgentBox/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/cokovali10-sketch/AgentBox)](https://goreportcard.com/report/github.com/cokovali10-sketch/AgentBox)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](./LICENSE)
[![MCP](https://img.shields.io/badge/MCP-2024--11--05-8b5cf6?style=flat)](https://modelcontextprotocol.io)
[![Speed](https://img.shields.io/badge/Startup-%3C1ms-00ADD8?style=flat)](#-performance--benchmarks)

</div>

---

## 💡 Why AgentBox?

Modern AI coding agents (Claude, Cursor, Copilot, Antigravity) generate code and terminal commands on the fly. Running untrusted AI code poses severe challenges:
- **Docker is too heavy:** Starting a Docker container takes **500ms – 2000ms**, consuming hundreds of megabytes of RAM per container.
- **Bare host execution is unsafe:** AI code can accidentally overwrite files, exhaust memory, or execute destructive commands (`rm -rf /`, fork bombs).

**AgentBox bridges this gap:** It provides an ultra-lightweight, ephemeral micro-sandbox with **sub-millisecond startup (<1ms)**, memory/CPU guards, destructive command filtering, and a native **Model Context Protocol (MCP)** server out of the box.

```
┌─────────────────────────────────────────────────────────────┐
│                    AI Coding Agent (LLM)                    │
└──────────────────────────────┬──────────────────────────────┘
                               │  stdio JSON-RPC (MCP)
                               ▼
┌─────────────────────────────────────────────────────────────┐
│                    ⚡ AgentBox Runtime                      │
├──────────────────────────────┬──────────────────────────────┤
│    🛡️ Security Policy Gate   │   ⏱️ Ephemeral Jail Engine   │
│  - Destructive regex filters │  - Sub-millisecond spin-up   │
│  - Fork bomb prevention      │  - Memory & CPU quotas       │
│  - File path confinement     │  - Automatic tree teardown   │
├──────────────────────────────┴──────────────────────────────┤
│                   Supported Runtimes                        │
│     Python 3.x · Node.js · Go · Shell/Bash · PowerShell     │
└─────────────────────────────────────────────────────────────┘
```

---

## ⚡ Performance & Benchmarks

Measured on reference hardware (`DESKTOP-VA18UL4`, Windows 11 / AMD 12-core):

| Metric | Docker Container | Traditional VM | **AgentBox** |
| :--- | :---: | :---: | :---: |
| **Startup Latency** | ~850 ms | ~4,500 ms | **0.63 ms (<1 ms)** |
| **Memory Footprint** | ~120 MB | ~1,024 MB | **< 8 MB** |
| **Teardown Overhead** | ~300 ms | ~1,500 ms | **Instant (defer os.RemoveAll)** |
| **AI Protocol** | Custom REST | SSH | **Native MCP (JSON-RPC 2.0)** |

---

## 🚀 Quick Start

### 1. Build Single Binary

```bash
git clone https://github.com/cokovali10-sketch/AgentBox.git
cd AgentBox
go build -o agentbox.exe ./cmd/agentbox
```

### 2. Run Code Snippets in Isolation

```bash
# Python
agentbox run -lang python -code "print('Calculated:', sum(range(100)))"

# Go
agentbox run -lang go -code 'fmt.Println("Hello from Go micro-jail")'

# Node.js
agentbox run -lang node -code "console.log(process.versions)"

# Output with JSON telemetry & microsecond metrics
agentbox run -lang python -code "print('Done')" -json
```

### 3. Safety Guard Example

Dangerous or destructive commands are rejected **before execution**:
```bash
agentbox run -lang command -cmd "rm -rf /"
# [AgentBox Warning] security policy violation: command matches dangerous pattern '\brm\s+-[rf]{1,2}\s+([/\\]|\*|~)'
```

---

## 🔌 Using as an MCP Server in Claude, Cursor, Antigravity

Add AgentBox to your MCP configuration (`mcp_config.json`):

```json
{
  "mcpServers": {
    "agentbox": {
      "command": "/path/to/agentbox",
      "args": ["mcp"]
    }
  }
}
```

### Exposed MCP Tools:
- **`agentbox.run_code`**: Execute code (Python, Node, Go, Bash, PowerShell) with injected files and timeouts.
- **`agentbox.exec_command`**: Execute safe terminal commands inside the isolated jail.
- **`agentbox.doctor`**: Inspect available runtimes and host sandbox capabilities.
- **`agentbox.bench`**: Benchmark micro-sandbox latency on the local machine.

---

## 🧪 Testing

```bash
go test -v ./...
```

---

## 📜 License

MIT License — see [LICENSE](./LICENSE).
