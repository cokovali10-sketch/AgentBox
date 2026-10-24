package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/cokovali10-sketch/AgentBox/pkg/mcp"
	"github.com/cokovali10-sketch/AgentBox/pkg/policy"
	"github.com/cokovali10-sketch/AgentBox/pkg/sandbox"
)

const (
	Version = "1.0.0"
	Banner  = `
   _____                    __  ____            
  /  _  \   ____   ____   _/  |_\______   _______  ___
 /  /_\  \ / ___\_/ __ \  \   __\|    |  _/  _ \  \/  /
/    |    / /_/  >  ___/   |  |  |    |   (  <_> >    < 
\____|__  \___  / \___  >  |__|  |______  /\____/__/\_ \
        \/_____/      \/                \/            \/
   AI-Native Micro-Sandbox Runtime · Sub-millisecond Execution
`
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	subcommand := os.Args[1]

	mgr, err := sandbox.NewManager("", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing sandbox manager: %v\n", err)
		os.Exit(1)
	}

	switch subcommand {
	case "mcp":
		server := mcp.NewServer(mgr, os.Stdin, os.Stdout)
		if err := server.Serve(); err != nil {
			fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
			os.Exit(1)
		}

	case "doctor":
		server := mcp.NewServer(mgr, nil, nil)
		doc, err := server.CallToolDirect("agentbox.doctor", nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Doctor failed: %v\n", err)
			os.Exit(1)
		}
		printJSON(doc)

	case "bench":
		benchCmd := flag.NewFlagSet("bench", flag.ExitOnError)
		repeats := benchCmd.Int("repeats", 5, "Number of benchmark iterations")
		_ = benchCmd.Parse(os.Args[2:])

		fmt.Printf("⚡ Benchmarking AgentBox micro-sandbox (%d runs)...\n", *repeats)
		server := mcp.NewServer(mgr, nil, nil)
		args, _ := json.Marshal(map[string]any{"repeats": *repeats})
		res, err := server.CallToolDirect("agentbox.bench", args)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Benchmark error: %v\n", err)
			os.Exit(1)
		}
		printJSON(res)

	case "run":
		runCmd := flag.NewFlagSet("run", flag.ExitOnError)
		lang := runCmd.String("lang", "python", "Language: python, node, go, bash, powershell, command")
		code := runCmd.String("code", "", "Inline code snippet to execute")
		file := runCmd.String("file", "", "Path to code file on host")
		cmdStr := runCmd.String("cmd", "", "Command to execute (for lang=command)")
		timeoutSec := runCmd.Int("timeout", 10, "Execution timeout in seconds")
		memMB := runCmd.Int64("mem", 256, "Memory limit in MB")
		jsonOut := runCmd.Bool("json", false, "Output results in JSON format")

		_ = runCmd.Parse(os.Args[2:])

		codeContent := *code
		if *file != "" {
			data, err := os.ReadFile(*file)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to read file: %v\n", err)
				os.Exit(1)
			}
			codeContent = string(data)
		}

		if codeContent == "" && *cmdStr == "" {
			fmt.Fprintln(os.Stderr, "Error: Either -code, -file, or -cmd must be specified.")
			runCmd.Usage()
			os.Exit(1)
		}

		pol := policy.DefaultPolicy()
		pol.Timeout = time.Duration(*timeoutSec) * time.Second
		pol.MaxMemoryMB = *memMB

		req := sandbox.ExecutionRequest{
			Language: sandbox.Language(*lang),
			Code:     codeContent,
			Command:  *cmdStr,
			Policy:   &pol,
		}

		resp, err := mgr.Execute(context.Background(), req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Execution failed: %v\n", err)
			os.Exit(1)
		}

		if *jsonOut {
			printJSON(resp)
		} else {
			if resp.Stdout != "" {
				fmt.Print(resp.Stdout)
			}
			if resp.Stderr != "" {
				fmt.Fprint(os.Stderr, resp.Stderr)
			}
			if resp.Error != "" {
				fmt.Fprintf(os.Stderr, "[AgentBox Warning] %s\n", resp.Error)
			}
			os.Exit(resp.ExitCode)
		}

	case "version", "-v", "--version":
		fmt.Printf("AgentBox version %s (Go runtime)\n", Version)

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", subcommand)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(Banner)
	fmt.Printf("Version %s\n\n", Version)
	fmt.Println("Usage: agentbox <command> [options]")
	fmt.Println("\nCommands:")
	fmt.Println("  mcp               Start stdio Model Context Protocol (MCP) server for AI agents")
	fmt.Println("  run               Execute code or command in an isolated micro-sandbox")
	fmt.Println("  bench             Benchmark micro-sandbox startup and execution latency")
	fmt.Println("  doctor            Inspect available compilers, runtimes and host features")
	fmt.Println("  version           Display version information")
	fmt.Println("\nRun 'agentbox run -help' for code execution flags.")
}

func printJSON(v any) {
	bytes, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(bytes))
}
