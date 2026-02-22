package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/anthropics/cx/internal/mcp"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	callList bool
	callPipe bool
)

var callCmd = &cobra.Command{
	Use:   "call [tool] [json-args]",
	Short: "Unified tool gateway for all CX operations",
	Long: `Call any CX tool with structured JSON input/output.

This is the single entry point for all CX operations. Tools accept JSON
arguments and return JSON results.

Modes:
  cx call --list                          List all tools and parameters
  cx call <tool> '{"key":"value"}'        Call a tool with JSON args
  cx call --pipe                          Read JSON lines from stdin

Tool names accept shorthand: "find" is equivalent to "cx_find".

Examples:
  cx call --list
  cx call find '{"pattern":"Store"}'
  cx call show '{"name":"Store","density":"dense"}'
  cx call context '{"smart":"auth","budget":4000}'
  cx call safe '{"target":"internal/mcp/server.go"}'
  cx call map '{}'
  echo '{"tool":"cx_find","args":{"pattern":"Store"}}' | cx call --pipe`,
	Args: cobra.MaximumNArgs(2),
	RunE: runCall,
}

func init() {
	rootCmd.AddCommand(callCmd)
	callCmd.Flags().BoolVar(&callList, "list", false, "List all available tools and their parameters")
	callCmd.Flags().BoolVar(&callPipe, "pipe", false, "Read JSON lines from stdin (pipe mode)")
}

func runCall(cmd *cobra.Command, args []string) error {
	if callList {
		return runCallList()
	}
	if callPipe {
		return runCallPipe()
	}
	if len(args) == 0 {
		return fmt.Errorf("tool name required (run 'cx call --list' to see available tools)")
	}
	return runCallSingle(args)
}

func runCallList() error {
	srv, err := mcp.New(mcp.Config{Tools: mcp.AllTools})
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}
	defer srv.Close()

	schemas := srv.GetToolSchemas()

	switch outputFormat {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(schemas)
	case "jsonl":
		enc := json.NewEncoder(os.Stdout)
		for _, s := range schemas {
			if err := enc.Encode(s); err != nil {
				return err
			}
		}
		return nil
	default: // yaml
		enc := yaml.NewEncoder(os.Stdout)
		enc.SetIndent(2)
		defer enc.Close()
		return enc.Encode(schemas)
	}
}

func runCallSingle(args []string) error {
	toolName := normalizeToolName(args[0])

	// Parse JSON args
	var toolArgs map[string]interface{}
	if len(args) >= 2 {
		if err := json.Unmarshal([]byte(args[1]), &toolArgs); err != nil {
			return fmt.Errorf("invalid JSON args: %w", err)
		}
	} else {
		toolArgs = make(map[string]interface{})
	}

	srv, err := mcp.New(mcp.Config{Tools: mcp.AllTools})
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}
	defer srv.Close()

	result, err := srv.CallTool(toolName, toolArgs)
	if err != nil {
		return err
	}

	fmt.Println(result)
	return nil
}

// pipeRequest is the JSON format for pipe mode input.
type pipeRequest struct {
	Tool string                 `json:"tool"`
	Args map[string]interface{} `json:"args"`
}

// pipeResponse is the JSON format for pipe mode output.
type pipeResponse struct {
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

func runCallPipe() error {
	srv, err := mcp.New(mcp.Config{Tools: mcp.AllTools})
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}
	defer srv.Close()

	enc := json.NewEncoder(os.Stdout)
	scanner := bufio.NewScanner(os.Stdin)
	// Allow larger lines (1MB)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var req pipeRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			enc.Encode(pipeResponse{Error: fmt.Sprintf("invalid JSON: %v", err)})
			continue
		}

		toolName := normalizeToolName(req.Tool)
		if req.Args == nil {
			req.Args = make(map[string]interface{})
		}

		result, err := srv.CallTool(toolName, req.Args)
		if err != nil {
			enc.Encode(pipeResponse{Error: err.Error()})
			continue
		}

		// Try to marshal result as raw JSON; if it's already JSON, use it directly
		var raw json.RawMessage
		if err := json.Unmarshal([]byte(result), &raw); err != nil {
			// Not valid JSON — wrap as string
			b, _ := json.Marshal(result)
			raw = b
		}
		enc.Encode(pipeResponse{Result: raw})
	}

	return scanner.Err()
}

// normalizeToolName converts shorthand names to full tool names.
// "find" -> "cx_find", "cx_find" -> "cx_find"
func normalizeToolName(name string) string {
	if !strings.HasPrefix(name, "cx_") {
		return "cx_" + name
	}
	return name
}
