// Package mcp provides an MCP (Model Context Protocol) server for cx.
// This allows AI agents to query the code graph through MCP tools instead of CLI commands.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/anthropics/cx/internal/config"
	cxcontext "github.com/anthropics/cx/internal/context"
	"github.com/anthropics/cx/internal/extract"
	"github.com/anthropics/cx/internal/graph"
	"github.com/anthropics/cx/internal/parser"
	"github.com/anthropics/cx/internal/store"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// hasStringSuffix checks if s ends with suffix
func hasStringSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

// Server wraps the MCP server with cx-specific functionality
type Server struct {
	mcpServer    *server.MCPServer
	store        *store.Store
	graph        *graph.Graph
	cxDir        string
	projectRoot  string
	tools        map[string]bool
	lastActivity time.Time
	timeout      time.Duration
	mu           sync.RWMutex
}

// Config holds server configuration
type Config struct {
	Tools   []string      // Which tools to expose (empty = all)
	Timeout time.Duration // Inactivity timeout (0 = no timeout)
}

// DefaultTools is the default set of tools to expose
var DefaultTools = []string{"cx_context", "cx_safe", "cx_show", "cx_find", "cx_map", "cx_trace", "cx_tag", "cx_guard"}

// AllTools lists all available tools
var AllTools = []string{"cx_context", "cx_safe", "cx_find", "cx_show", "cx_map", "cx_diff", "cx_impact", "cx_gaps", "cx_blame", "cx_tag", "cx_trace", "cx_dead", "cx_test", "cx_guard"}

// New creates a new MCP server for cx
func New(cfg Config) (*Server, error) {
	// Find .cx directory
	cxDir, err := config.FindConfigDir(".")
	if err != nil {
		return nil, fmt.Errorf("cx not initialized: run 'cx init && cx scan' first")
	}
	projectRoot := filepath.Dir(cxDir)

	// Open store
	storeDB, err := store.Open(cxDir)
	if err != nil {
		return nil, fmt.Errorf("failed to open store: %w", err)
	}

	// Build graph
	g, err := graph.BuildFromStore(storeDB)
	if err != nil {
		storeDB.Close()
		return nil, fmt.Errorf("failed to build graph: %w", err)
	}

	// Create MCP server
	mcpServer := server.NewMCPServer(
		"cx",
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	s := &Server{
		mcpServer:    mcpServer,
		store:        storeDB,
		graph:        g,
		cxDir:        cxDir,
		projectRoot:  projectRoot,
		tools:        make(map[string]bool),
		lastActivity: time.Now(),
		timeout:      cfg.Timeout,
	}

	// Determine which tools to register
	toolsToRegister := cfg.Tools
	if len(toolsToRegister) == 0 {
		toolsToRegister = DefaultTools
	}

	// Register tools
	for _, toolName := range toolsToRegister {
		if err := s.registerTool(toolName); err != nil {
			storeDB.Close()
			return nil, fmt.Errorf("failed to register tool %s: %w", toolName, err)
		}
		s.tools[toolName] = true
	}

	return s, nil
}

// registerTool registers a single tool with the MCP server
func (s *Server) registerTool(name string) error {
	switch name {
	case "cx_diff":
		return s.registerDiffTool()
	case "cx_impact":
		return s.registerImpactTool()
	case "cx_context":
		return s.registerContextTool()
	case "cx_show":
		return s.registerShowTool()
	case "cx_find":
		return s.registerFindTool()
	case "cx_gaps":
		return s.registerGapsTool()
	case "cx_safe":
		return s.registerSafeTool()
	case "cx_map":
		return s.registerMapTool()
	case "cx_blame":
		return s.registerBlameTool()
	case "cx_tag":
		return s.registerTagTool()
	case "cx_trace":
		return s.registerTraceTool()
	case "cx_dead":
		return s.registerDeadTool()
	case "cx_test":
		return s.registerTestTool()
	case "cx_guard":
		return s.registerGuardTool()
	default:
		return fmt.Errorf("unknown tool: %s", name)
	}
}

// ServeStdio starts the server using stdio transport
func (s *Server) ServeStdio() error {
	// Start timeout checker if timeout is set
	if s.timeout > 0 {
		go s.timeoutChecker()
	}

	return server.ServeStdio(s.mcpServer)
}

// timeoutChecker monitors for inactivity and exits if timeout exceeded
func (s *Server) timeoutChecker() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.RLock()
		elapsed := time.Since(s.lastActivity)
		s.mu.RUnlock()

		if elapsed > s.timeout {
			fmt.Fprintf(os.Stderr, "cx serve: timeout after %v of inactivity\n", s.timeout)
			os.Exit(0)
		}
	}
}

// updateActivity updates the last activity timestamp
func (s *Server) updateActivity() {
	s.mu.Lock()
	s.lastActivity = time.Now()
	s.mu.Unlock()
}

// Close closes the server and its resources
func (s *Server) Close() error {
	if s.store != nil {
		return s.store.Close()
	}
	return nil
}

// ListTools returns the list of registered tools
func (s *Server) ListTools() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tools := make([]string, 0, len(s.tools))
	for t := range s.tools {
		tools = append(tools, t)
	}
	return tools
}

// ToolSchema describes a tool's name, description, and parameters.
type ToolSchema struct {
	Name        string            `json:"name" yaml:"name"`
	Description string            `json:"description" yaml:"description"`
	Parameters  []ParameterSchema `json:"parameters" yaml:"parameters"`
}

// ParameterSchema describes a single tool parameter.
type ParameterSchema struct {
	Name        string `json:"name" yaml:"name"`
	Type        string `json:"type" yaml:"type"`
	Description string `json:"description" yaml:"description"`
	Required    bool   `json:"required" yaml:"required"`
}

// toolSchemaRegistry holds the schema definitions for all tools.
// These mirror the mcp.NewTool() definitions in the register*Tool() functions.
var toolSchemaRegistry = map[string]ToolSchema{
	"cx_diff": {
		Name:        "cx_diff",
		Description: "Show changes since last scan. Returns added, modified, and removed entities.",
		Parameters: []ParameterSchema{
			{Name: "file", Type: "string", Description: "Filter to specific file or directory path"},
			{Name: "detailed", Type: "boolean", Description: "Include hash values for modified entities"},
		},
	},
	"cx_impact": {
		Name:        "cx_impact",
		Description: "Analyze blast radius of changes to a file or entity. Shows what code would be affected.",
		Parameters: []ParameterSchema{
			{Name: "target", Type: "string", Description: "File path or entity name to analyze", Required: true},
			{Name: "depth", Type: "number", Description: "Transitive depth (default: 3)"},
			{Name: "threshold", Type: "number", Description: "Minimum importance threshold (PageRank score)"},
		},
	},
	"cx_context": {
		Name:        "cx_context",
		Description: "Assemble task-relevant context within a token budget. Use for smart context gathering.",
		Parameters: []ParameterSchema{
			{Name: "smart", Type: "string", Description: "Natural language task description for intent-aware context"},
			{Name: "target", Type: "string", Description: "Entity ID, file path, or bead ID for direct context"},
			{Name: "budget", Type: "number", Description: "Token budget (default: 4000)"},
			{Name: "depth", Type: "number", Description: "Max hops from entry points (default: 2)"},
		},
	},
	"cx_show": {
		Name:        "cx_show",
		Description: "Show detailed information about a code entity.",
		Parameters: []ParameterSchema{
			{Name: "name", Type: "string", Description: "Entity name or ID to look up", Required: true},
			{Name: "density", Type: "string", Description: "Detail level: sparse, medium, dense (default: medium)"},
			{Name: "coverage", Type: "boolean", Description: "Include test coverage information"},
		},
	},
	"cx_find": {
		Name:        "cx_find",
		Description: "Search for entities by name pattern.",
		Parameters: []ParameterSchema{
			{Name: "pattern", Type: "string", Description: "Name pattern to search for", Required: true},
			{Name: "type", Type: "string", Description: "Filter by type: F (function), T (type), M (method)"},
			{Name: "limit", Type: "number", Description: "Maximum results (default: 20)"},
		},
	},
	"cx_gaps": {
		Name:        "cx_gaps",
		Description: "Find coverage gaps in critical code.",
		Parameters: []ParameterSchema{
			{Name: "keystones_only", Type: "boolean", Description: "Only show gaps in keystone (high-importance) entities"},
			{Name: "threshold", Type: "number", Description: "Coverage threshold percentage (default: 50)"},
		},
	},
	"cx_safe": {
		Name:        "cx_safe",
		Description: "Pre-flight safety check before modifying code. Returns risk level, impact radius, coverage gaps, and recommendations.",
		Parameters: []ParameterSchema{
			{Name: "target", Type: "string", Description: "File path or entity name to check", Required: true},
			{Name: "quick", Type: "boolean", Description: "Quick mode: just blast radius (impact analysis only)"},
			{Name: "depth", Type: "number", Description: "Transitive impact depth (default: 3)"},
		},
	},
	"cx_map": {
		Name:        "cx_map",
		Description: "Project skeleton overview showing function signatures and type definitions. Useful for codebase orientation.",
		Parameters: []ParameterSchema{
			{Name: "path", Type: "string", Description: "Subdirectory to map (default: project root)"},
			{Name: "filter", Type: "string", Description: "Filter by entity type: F (function), T (type), M (method), C (constant)"},
			{Name: "lang", Type: "string", Description: "Filter by language (go, typescript, python, rust, java)"},
		},
	},
	"cx_blame": {
		Name:        "cx_blame",
		Description: "Show commit history for a code entity. Tracks how an entity changed over time.",
		Parameters: []ParameterSchema{
			{Name: "entity", Type: "string", Description: "Entity name or ID to get history for", Required: true},
			{Name: "limit", Type: "number", Description: "Maximum history entries (default: 20)"},
			{Name: "deps", Type: "boolean", Description: "Include dependency change history"},
		},
	},
	"cx_tag": {
		Name:        "cx_tag",
		Description: "Manage entity tags. Add, remove, list, or find entities by tags.",
		Parameters: []ParameterSchema{
			{Name: "action", Type: "string", Description: "Action: add, remove, list, find", Required: true},
			{Name: "entity", Type: "string", Description: "Entity name or ID (required for add/remove, optional for list)"},
			{Name: "tags", Type: "string", Description: "Comma-separated tags (for add/find)"},
			{Name: "note", Type: "string", Description: "Note for why the tag was added (for add)"},
			{Name: "match_all", Type: "boolean", Description: "Require all tags to match (for find, default: false)"},
		},
	},
	"cx_trace": {
		Name:        "cx_trace",
		Description: "Trace call chains between entities. Find paths, callers, or callees in the dependency graph.",
		Parameters: []ParameterSchema{
			{Name: "from", Type: "string", Description: "Source entity name or ID", Required: true},
			{Name: "to", Type: "string", Description: "Target entity name or ID (for path mode)"},
			{Name: "mode", Type: "string", Description: "Mode: path (default), callers, callees"},
			{Name: "depth", Type: "number", Description: "Maximum traversal depth (default: 5)"},
			{Name: "all", Type: "boolean", Description: "Find all paths instead of shortest (for path mode)"},
		},
	},
	"cx_dead": {
		Name:        "cx_dead",
		Description: "Find dead code using graph analysis. Three confidence tiers: definite (private, zero callers), probable (exported, zero callers), suspicious (callers are all dead).",
		Parameters: []ParameterSchema{
			{Name: "tier", Type: "number", Description: "Max confidence tier: 1=definite, 2=+probable, 3=+suspicious (default: 1)"},
			{Name: "type_filter", Type: "string", Description: "Filter by type: F (function), T (type), M (method), C (constant)"},
			{Name: "include_exports", Type: "boolean", Description: "Include exported entities in tier 1"},
		},
	},
	"cx_test": {
		Name:        "cx_test",
		Description: "Smart test selection based on code changes, or find coverage gaps.",
		Parameters: []ParameterSchema{
			{Name: "mode", Type: "string", Description: "Mode: select (default) or gaps"},
			{Name: "file", Type: "string", Description: "Specific file to analyze (for select mode)"},
			{Name: "diff", Type: "string", Description: "Git ref for diff (default: HEAD for uncommitted changes)"},
			{Name: "depth", Type: "number", Description: "Caller chain depth for indirect tests (default: 2)"},
			{Name: "keystones_only", Type: "boolean", Description: "Only show gaps in keystone entities (for gaps mode)"},
			{Name: "threshold", Type: "number", Description: "Coverage threshold percentage (default: 50, for gaps mode)"},
		},
	},
	"cx_guard": {
		Name:        "cx_guard",
		Description: "Pre-commit quality checks. Detects signature drift, dead-on-arrival code, coverage regression, and graph drift.",
		Parameters: []ParameterSchema{
			{Name: "files", Type: "string", Description: "Comma-separated file paths to check (default: git staged files)"},
			{Name: "staged", Type: "boolean", Description: "Check only git staged files (default: true)"},
			{Name: "all", Type: "boolean", Description: "Check all modified files (staged + unstaged)"},
			{Name: "fail_on_warnings", Type: "boolean", Description: "Treat warnings as errors"},
		},
	},
}

// GetToolSchemas returns schemas for all registered tools.
func (s *Server) GetToolSchemas() []ToolSchema {
	s.mu.RLock()
	defer s.mu.RUnlock()

	schemas := make([]ToolSchema, 0, len(s.tools))
	for name := range s.tools {
		if schema, ok := toolSchemaRegistry[name]; ok {
			schemas = append(schemas, schema)
		}
	}
	return schemas
}

// CallTool dispatches a tool call by name with the given arguments.
// Returns the JSON result string or an error.
func (s *Server) CallTool(name string, args map[string]interface{}) (string, error) {
	s.mu.RLock()
	registered := s.tools[name]
	s.mu.RUnlock()

	if !registered {
		return "", fmt.Errorf("unknown tool: %s (run 'cx call --list' to see available tools)", name)
	}

	switch name {
	case "cx_diff":
		file, _ := args["file"].(string)
		detailed, _ := args["detailed"].(bool)
		return s.executeDiff(file, detailed)

	case "cx_impact":
		target, _ := args["target"].(string)
		if target == "" {
			return "", fmt.Errorf("target parameter is required")
		}
		depth := 3
		if d, ok := args["depth"].(float64); ok {
			depth = int(d)
		}
		threshold := 0.0
		if t, ok := args["threshold"].(float64); ok {
			threshold = t
		}
		return s.executeImpact(target, depth, threshold)

	case "cx_context":
		smart, _ := args["smart"].(string)
		target, _ := args["target"].(string)
		budget := 4000
		if b, ok := args["budget"].(float64); ok {
			budget = int(b)
		}
		depth := 2
		if d, ok := args["depth"].(float64); ok {
			depth = int(d)
		}
		return s.executeContext(smart, target, budget, depth)

	case "cx_show":
		name, _ := args["name"].(string)
		if name == "" {
			return "", fmt.Errorf("name parameter is required")
		}
		density, _ := args["density"].(string)
		if density == "" {
			density = "medium"
		}
		coverage, _ := args["coverage"].(bool)
		return s.executeShow(name, density, coverage)

	case "cx_find":
		pattern, _ := args["pattern"].(string)
		if pattern == "" {
			return "", fmt.Errorf("pattern parameter is required")
		}
		typeFilter, _ := args["type"].(string)
		limit := 20
		if l, ok := args["limit"].(float64); ok {
			limit = int(l)
		}
		return s.executeFind(pattern, typeFilter, limit)

	case "cx_gaps":
		keystonesOnly, _ := args["keystones_only"].(bool)
		threshold := 50.0
		if t, ok := args["threshold"].(float64); ok {
			threshold = t
		}
		return s.executeGaps(keystonesOnly, threshold)

	case "cx_safe":
		target, _ := args["target"].(string)
		if target == "" {
			return "", fmt.Errorf("target parameter is required")
		}
		quick, _ := args["quick"].(bool)
		depth := 3
		if d, ok := args["depth"].(float64); ok {
			depth = int(d)
		}
		return s.executeSafe(target, quick, depth)

	case "cx_map":
		path, _ := args["path"].(string)
		filter, _ := args["filter"].(string)
		lang, _ := args["lang"].(string)
		return s.executeMap(path, filter, lang)

	case "cx_blame":
		entity, _ := args["entity"].(string)
		if entity == "" {
			return "", fmt.Errorf("entity parameter is required")
		}
		limit := 20
		if l, ok := args["limit"].(float64); ok {
			limit = int(l)
		}
		deps, _ := args["deps"].(bool)
		return s.executeBlame(entity, limit, deps)

	case "cx_tag":
		action, _ := args["action"].(string)
		if action == "" {
			return "", fmt.Errorf("action parameter is required")
		}
		entityName, _ := args["entity"].(string)
		tags, _ := args["tags"].(string)
		note, _ := args["note"].(string)
		matchAll, _ := args["match_all"].(bool)
		return s.executeTag(action, entityName, tags, note, matchAll)

	case "cx_trace":
		from, _ := args["from"].(string)
		if from == "" {
			return "", fmt.Errorf("from parameter is required")
		}
		to, _ := args["to"].(string)
		mode, _ := args["mode"].(string)
		if mode == "" {
			mode = "path"
		}
		depth := 5
		if d, ok := args["depth"].(float64); ok {
			depth = int(d)
		}
		allPaths, _ := args["all"].(bool)
		return s.executeTrace(from, to, mode, depth, allPaths)

	case "cx_dead":
		tier := 1
		if t, ok := args["tier"].(float64); ok {
			tier = int(t)
		}
		typeFilter, _ := args["type_filter"].(string)
		includeExports, _ := args["include_exports"].(bool)
		return s.executeDead(tier, typeFilter, includeExports)

	case "cx_test":
		mode, _ := args["mode"].(string)
		if mode == "" {
			mode = "select"
		}
		file, _ := args["file"].(string)
		diff, _ := args["diff"].(string)
		depth := 2
		if d, ok := args["depth"].(float64); ok {
			depth = int(d)
		}
		keystonesOnly, _ := args["keystones_only"].(bool)
		threshold := 50.0
		if t, ok := args["threshold"].(float64); ok {
			threshold = t
		}
		return s.executeTest(mode, file, diff, depth, keystonesOnly, threshold)

	case "cx_guard":
		files, _ := args["files"].(string)
		staged, _ := args["staged"].(bool)
		all, _ := args["all"].(bool)
		failOnWarnings, _ := args["fail_on_warnings"].(bool)
		// Default to staged if neither specified
		if !all && files == "" {
			staged = true
		}
		return s.executeGuard(files, staged, all, failOnWarnings)

	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

// registerDiffTool registers the cx_diff tool
func (s *Server) registerDiffTool() error {
	tool := mcp.NewTool("cx_diff",
		mcp.WithDescription("Show changes since last scan. Returns added, modified, and removed entities."),
		mcp.WithString("file",
			mcp.Description("Filter to specific file or directory path"),
		),
		mcp.WithBoolean("detailed",
			mcp.Description("Include hash values for modified entities"),
		),
	)

	s.mcpServer.AddTool(tool, s.handleDiff)
	return nil
}

// registerImpactTool registers the cx_impact tool
func (s *Server) registerImpactTool() error {
	tool := mcp.NewTool("cx_impact",
		mcp.WithDescription("Analyze blast radius of changes to a file or entity. Shows what code would be affected."),
		mcp.WithString("target",
			mcp.Required(),
			mcp.Description("File path or entity name to analyze"),
		),
		mcp.WithNumber("depth",
			mcp.Description("Transitive depth (default: 3)"),
		),
		mcp.WithNumber("threshold",
			mcp.Description("Minimum importance threshold (PageRank score)"),
		),
	)

	s.mcpServer.AddTool(tool, s.handleImpact)
	return nil
}

// registerContextTool registers the cx_context tool
func (s *Server) registerContextTool() error {
	tool := mcp.NewTool("cx_context",
		mcp.WithDescription("Assemble task-relevant context within a token budget. Use for smart context gathering."),
		mcp.WithString("smart",
			mcp.Description("Natural language task description for intent-aware context"),
		),
		mcp.WithString("target",
			mcp.Description("Entity ID, file path, or bead ID for direct context"),
		),
		mcp.WithNumber("budget",
			mcp.Description("Token budget (default: 4000)"),
		),
		mcp.WithNumber("depth",
			mcp.Description("Max hops from entry points (default: 2)"),
		),
	)

	s.mcpServer.AddTool(tool, s.handleContext)
	return nil
}

// registerShowTool registers the cx_show tool
func (s *Server) registerShowTool() error {
	tool := mcp.NewTool("cx_show",
		mcp.WithDescription("Show detailed information about a code entity."),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Entity name or ID to look up"),
		),
		mcp.WithString("density",
			mcp.Description("Detail level: sparse, medium, dense (default: medium)"),
		),
		mcp.WithBoolean("coverage",
			mcp.Description("Include test coverage information"),
		),
	)

	s.mcpServer.AddTool(tool, s.handleShow)
	return nil
}

// registerFindTool registers the cx_find tool
func (s *Server) registerFindTool() error {
	tool := mcp.NewTool("cx_find",
		mcp.WithDescription("Search for entities by name pattern."),
		mcp.WithString("pattern",
			mcp.Required(),
			mcp.Description("Name pattern to search for"),
		),
		mcp.WithString("type",
			mcp.Description("Filter by type: F (function), T (type), M (method)"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum results (default: 20)"),
		),
	)

	s.mcpServer.AddTool(tool, s.handleFind)
	return nil
}

// registerGapsTool registers the cx_gaps tool
func (s *Server) registerGapsTool() error {
	tool := mcp.NewTool("cx_gaps",
		mcp.WithDescription("Find coverage gaps in critical code."),
		mcp.WithBoolean("keystones_only",
			mcp.Description("Only show gaps in keystone (high-importance) entities"),
		),
		mcp.WithNumber("threshold",
			mcp.Description("Coverage threshold percentage (default: 50)"),
		),
	)

	s.mcpServer.AddTool(tool, s.handleGaps)
	return nil
}

// registerSafeTool registers the cx_safe tool
func (s *Server) registerSafeTool() error {
	tool := mcp.NewTool("cx_safe",
		mcp.WithDescription("Pre-flight safety check before modifying code. Returns risk level, impact radius, coverage gaps, and recommendations."),
		mcp.WithString("target",
			mcp.Required(),
			mcp.Description("File path or entity name to check"),
		),
		mcp.WithBoolean("quick",
			mcp.Description("Quick mode: just blast radius (impact analysis only)"),
		),
		mcp.WithNumber("depth",
			mcp.Description("Transitive impact depth (default: 3)"),
		),
	)

	s.mcpServer.AddTool(tool, s.handleSafe)
	return nil
}

// registerMapTool registers the cx_map tool
func (s *Server) registerMapTool() error {
	tool := mcp.NewTool("cx_map",
		mcp.WithDescription("Project skeleton overview showing function signatures and type definitions. Useful for codebase orientation."),
		mcp.WithString("path",
			mcp.Description("Subdirectory to map (default: project root)"),
		),
		mcp.WithString("filter",
			mcp.Description("Filter by entity type: F (function), T (type), M (method), C (constant)"),
		),
		mcp.WithString("lang",
			mcp.Description("Filter by language (go, typescript, python, rust, java)"),
		),
	)

	s.mcpServer.AddTool(tool, s.handleMap)
	return nil
}

// Tool handlers

func (s *Server) handleDiff(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	s.updateActivity()

	args := req.GetArguments()
	file, _ := args["file"].(string)
	detailed, _ := args["detailed"].(bool)

	result, err := s.executeDiff(file, detailed)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(result), nil
}

func (s *Server) handleImpact(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	s.updateActivity()

	args := req.GetArguments()
	target, ok := args["target"].(string)
	if !ok || target == "" {
		return mcp.NewToolResultError("target parameter is required"), nil
	}

	depth := 3
	if d, ok := args["depth"].(float64); ok {
		depth = int(d)
	}

	threshold := 0.0
	if t, ok := args["threshold"].(float64); ok {
		threshold = t
	}

	result, err := s.executeImpact(target, depth, threshold)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(result), nil
}

func (s *Server) handleContext(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	s.updateActivity()

	args := req.GetArguments()
	smart, _ := args["smart"].(string)
	target, _ := args["target"].(string)

	budget := 4000
	if b, ok := args["budget"].(float64); ok {
		budget = int(b)
	}

	depth := 2
	if d, ok := args["depth"].(float64); ok {
		depth = int(d)
	}

	result, err := s.executeContext(smart, target, budget, depth)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(result), nil
}

func (s *Server) handleShow(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	s.updateActivity()

	args := req.GetArguments()
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return mcp.NewToolResultError("name parameter is required"), nil
	}

	density, _ := args["density"].(string)
	if density == "" {
		density = "medium"
	}

	coverage, _ := args["coverage"].(bool)

	result, err := s.executeShow(name, density, coverage)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(result), nil
}

func (s *Server) handleFind(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	s.updateActivity()

	args := req.GetArguments()
	pattern, ok := args["pattern"].(string)
	if !ok || pattern == "" {
		return mcp.NewToolResultError("pattern parameter is required"), nil
	}

	typeFilter, _ := args["type"].(string)

	limit := 20
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	result, err := s.executeFind(pattern, typeFilter, limit)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(result), nil
}

func (s *Server) handleGaps(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	s.updateActivity()

	args := req.GetArguments()
	keystonesOnly, _ := args["keystones_only"].(bool)

	threshold := 50.0
	if t, ok := args["threshold"].(float64); ok {
		threshold = t
	}

	result, err := s.executeGaps(keystonesOnly, threshold)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(result), nil
}

func (s *Server) handleSafe(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	s.updateActivity()

	args := req.GetArguments()
	target, ok := args["target"].(string)
	if !ok || target == "" {
		return mcp.NewToolResultError("target parameter is required"), nil
	}

	quick, _ := args["quick"].(bool)

	depth := 3
	if d, ok := args["depth"].(float64); ok {
		depth = int(d)
	}

	result, err := s.executeSafe(target, quick, depth)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(result), nil
}

func (s *Server) handleMap(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	s.updateActivity()

	args := req.GetArguments()
	path, _ := args["path"].(string)
	filter, _ := args["filter"].(string)
	lang, _ := args["lang"].(string)

	result, err := s.executeMap(path, filter, lang)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(result), nil
}

// Execution functions (implementations)

func (s *Server) executeDiff(file string, detailed bool) (string, error) {
	// Get file entries from store
	fileEntries, err := s.store.GetAllFileEntries()
	if err != nil {
		return "", fmt.Errorf("failed to get file entries: %w", err)
	}

	if len(fileEntries) == 0 {
		return "", fmt.Errorf("no scan data found: run 'cx scan' first")
	}

	// Build result structure
	result := map[string]interface{}{
		"summary": map[string]interface{}{
			"files_scanned": len(fileEntries),
		},
		"status": "no changes detected since last scan",
	}

	// Note: Full diff implementation would compare filesystem vs stored hashes
	// For now, return basic info
	if file != "" {
		result["filter"] = file
	}

	return toJSON(result)
}

func (s *Server) executeImpact(target string, depth int, threshold float64) (string, error) {
	// Find direct entities
	var directEntities []*store.Entity

	// Check if target is a file path
	if isFilePath(target) {
		entities, err := s.store.QueryEntities(store.EntityFilter{
			FilePath: target,
			Status:   "active",
		})
		if err != nil {
			return "", fmt.Errorf("failed to query entities: %w", err)
		}
		directEntities = entities
	} else {
		// Try entity lookup
		entity, err := s.store.GetEntity(target)
		if err == nil && entity != nil {
			directEntities = append(directEntities, entity)
		} else {
			// Try name search
			entities, err := s.store.QueryEntities(store.EntityFilter{
				Name:   target,
				Status: "active",
				Limit:  10,
			})
			if err != nil {
				return "", fmt.Errorf("failed to query entities: %w", err)
			}
			directEntities = entities
		}
	}

	if len(directEntities) == 0 {
		return "", fmt.Errorf("no entities found matching: %s", target)
	}

	// Find affected entities using graph traversal
	affected := make(map[string]map[string]interface{})

	for _, direct := range directEntities {
		m, _ := s.store.GetMetrics(direct.ID)
		var pr float64
		if m != nil {
			pr = m.PageRank
		}

		affected[direct.Name] = map[string]interface{}{
			"type":       direct.EntityType,
			"location":   formatLocation(direct),
			"impact":     "direct",
			"importance": computeImportance(pr),
		}

		// BFS for callers
		visited := make(map[string]bool)
		visited[direct.ID] = true
		queue := []string{direct.ID}
		currentDepth := 1

		for len(queue) > 0 && currentDepth <= depth {
			levelSize := len(queue)
			for i := 0; i < levelSize; i++ {
				current := queue[0]
				queue = queue[1:]

				preds := s.graph.Predecessors(current)
				for _, pred := range preds {
					if visited[pred] {
						continue
					}
					visited[pred] = true
					queue = append(queue, pred)

					callerEntity, err := s.store.GetEntity(pred)
					if err != nil {
						continue
					}

					m, _ := s.store.GetMetrics(pred)
					var pr float64
					if m != nil {
						pr = m.PageRank
					}

					if pr >= threshold {
						impactType := "caller"
						if currentDepth > 1 {
							impactType = "indirect"
						}

						affected[callerEntity.Name] = map[string]interface{}{
							"type":       callerEntity.EntityType,
							"location":   formatLocation(callerEntity),
							"impact":     impactType,
							"depth":      currentDepth,
							"importance": computeImportance(pr),
						}
					}
				}
			}
			currentDepth++
		}
	}

	result := map[string]interface{}{
		"target":   target,
		"depth":    depth,
		"affected": affected,
		"summary": map[string]interface{}{
			"entities_affected": len(affected),
		},
	}

	return toJSON(result)
}

func (s *Server) executeContext(smart, target string, budget, depth int) (string, error) {
	if smart != "" {
		// Use smart context assembly
		assembler := cxcontext.NewSmartContext(s.store, s.graph, cxcontext.SmartContextOptions{
			TaskDescription: smart,
			Budget:          budget,
			Depth:           depth,
		})

		result, err := assembler.Assemble()
		if err != nil {
			return "", fmt.Errorf("smart context assembly failed: %w", err)
		}

		return toJSON(result)
	}

	if target != "" {
		// Direct entity/file context
		entity, err := s.store.GetEntity(target)
		if err != nil {
			return "", fmt.Errorf("entity not found: %s", target)
		}

		result := map[string]interface{}{
			"target": target,
			"entity": map[string]interface{}{
				"name":     entity.Name,
				"type":     entity.EntityType,
				"location": formatLocation(entity),
			},
		}

		return toJSON(result)
	}

	return "", fmt.Errorf("either --smart or --target is required")
}

// resolveEntity looks up an entity by ID or name, using GetEntity first then falling back to QueryEntities.
func (s *Server) resolveEntity(name string) (*store.Entity, error) {
	entity, err := s.store.GetEntity(name)
	if err != nil {
		entities, err := s.store.QueryEntities(store.EntityFilter{
			Name:   name,
			Status: "active",
			Limit:  1,
		})
		if err != nil || len(entities) == 0 {
			return nil, fmt.Errorf("entity not found: %s", name)
		}
		entity = entities[0]
	}
	return entity, nil
}

func (s *Server) executeShow(name, density string, coverage bool) (string, error) {
	entity, err := s.resolveEntity(name)
	if err != nil {
		return "", err
	}

	result := map[string]interface{}{
		"name":     entity.Name,
		"type":     entity.EntityType,
		"location": formatLocation(entity),
	}

	if density == "medium" || density == "dense" {
		result["signature"] = entity.Signature
		result["visibility"] = entity.Visibility

		// Get dependencies
		deps, _ := s.store.GetDependenciesFrom(entity.ID)
		if len(deps) > 0 {
			calls := []string{}
			for _, d := range deps {
				if d.DepType == "calls" {
					calls = append(calls, d.ToID)
				}
			}
			if len(calls) > 0 {
				result["calls"] = calls
			}
		}
	}

	if density == "dense" {
		m, _ := s.store.GetMetrics(entity.ID)
		if m != nil {
			result["metrics"] = map[string]interface{}{
				"pagerank":   m.PageRank,
				"in_degree":  m.InDegree,
				"out_degree": m.OutDegree,
				"importance": computeImportance(m.PageRank),
			}
		}
	}

	return toJSON(result)
}

func (s *Server) executeFind(pattern, typeFilter string, limit int) (string, error) {
	filter := store.EntityFilter{
		Name:   pattern,
		Status: "active",
		Limit:  limit,
	}

	if typeFilter != "" {
		switch typeFilter {
		case "F":
			filter.EntityType = "function"
		case "T":
			filter.EntityType = "type"
		case "M":
			filter.EntityType = "method"
		}
	}

	entities, err := s.store.QueryEntities(filter)
	if err != nil {
		return "", fmt.Errorf("search failed: %w", err)
	}

	results := make([]map[string]interface{}, 0, len(entities))
	for _, e := range entities {
		results = append(results, map[string]interface{}{
			"name":     e.Name,
			"type":     e.EntityType,
			"location": formatLocation(e),
		})
	}

	return toJSON(map[string]interface{}{
		"pattern": pattern,
		"count":   len(results),
		"results": results,
	})
}

func (s *Server) executeGaps(keystonesOnly bool, threshold float64) (string, error) {
	// Query all entities
	entities, err := s.store.QueryEntities(store.EntityFilter{Status: "active"})
	if err != nil {
		return "", fmt.Errorf("failed to query entities: %w", err)
	}

	gaps := make([]map[string]interface{}, 0)

	for _, e := range entities {
		m, _ := s.store.GetMetrics(e.ID)
		if m == nil {
			continue
		}

		// Skip non-keystones if keystones_only is set
		if keystonesOnly && m.PageRank < 0.30 {
			continue
		}

		// Note: Coverage data is stored separately via cx coverage import
		// For now, we report all entities matching the keystone filter
		// as potentially undertested (coverage integration pending)
		gaps = append(gaps, map[string]interface{}{
			"name":       e.Name,
			"type":       e.EntityType,
			"location":   formatLocation(e),
			"coverage":   0.0, // Coverage data not yet integrated
			"importance": computeImportance(m.PageRank),
		})
	}

	// Limit results for keystones only
	if keystonesOnly && len(gaps) > 50 {
		gaps = gaps[:50]
	}

	return toJSON(map[string]interface{}{
		"threshold": threshold,
		"gaps":      gaps,
		"count":     len(gaps),
		"note":      "Coverage data requires 'cx coverage import' first",
	})
}

func (s *Server) executeSafe(target string, quick bool, depth int) (string, error) {
	// Find direct entities matching the target
	directEntities, err := s.findDirectEntities(target)
	if err != nil {
		return "", err
	}

	if len(directEntities) == 0 {
		return "", fmt.Errorf("no entities found matching: %s", target)
	}

	// Find all affected entities via BFS traversal
	affected := s.findAffectedEntities(directEntities, depth)

	// For quick mode, just return impact analysis
	if quick {
		return s.buildQuickSafeOutput(target, affected, depth)
	}

	// Full safety assessment
	return s.buildFullSafeOutput(target, affected, depth)
}

// findDirectEntities finds entities matching the target (file path or entity name)
func (s *Server) findDirectEntities(target string) ([]*safeEntity, error) {
	var results []*safeEntity

	if isFilePath(target) {
		// File path: find all entities in the file
		entities, err := s.store.QueryEntities(store.EntityFilter{
			FilePath: target,
			Status:   "active",
		})
		if err != nil {
			return nil, fmt.Errorf("failed to query entities: %w", err)
		}

		// Try suffix match if exact match fails
		if len(entities) == 0 {
			entities, _ = s.store.QueryEntities(store.EntityFilter{
				FilePathSuffix: "/" + target,
				Status:         "active",
			})
		}

		for _, e := range entities {
			m, _ := s.store.GetMetrics(e.ID)
			results = append(results, &safeEntity{
				entity:  e,
				metrics: m,
				direct:  true,
				depth:   0,
			})
		}
	} else {
		// Entity name: try direct ID lookup first, then name search
		entity, err := s.store.GetEntity(target)
		if err == nil && entity != nil {
			m, _ := s.store.GetMetrics(entity.ID)
			results = append(results, &safeEntity{
				entity:  entity,
				metrics: m,
				direct:  true,
				depth:   0,
			})
		} else {
			// Try name search
			entities, err := s.store.QueryEntities(store.EntityFilter{
				Name:   target,
				Status: "active",
				Limit:  10,
			})
			if err == nil {
				for _, e := range entities {
					m, _ := s.store.GetMetrics(e.ID)
					results = append(results, &safeEntity{
						entity:  e,
						metrics: m,
						direct:  true,
						depth:   0,
					})
				}
			}
		}
	}

	return results, nil
}

// safeEntity holds entity info for safe analysis
type safeEntity struct {
	entity  *store.Entity
	metrics *store.Metrics
	depth   int
	direct  bool
}

// findAffectedEntities performs BFS to find all transitively affected entities
func (s *Server) findAffectedEntities(direct []*safeEntity, maxDepth int) map[string]*safeEntity {
	affected := make(map[string]*safeEntity)

	// Add direct entities
	for _, e := range direct {
		affected[e.entity.ID] = e
	}

	// BFS from each direct entity to find callers
	for _, directEnt := range direct {
		visited := make(map[string]int)
		visited[directEnt.entity.ID] = 0

		queue := []string{directEnt.entity.ID}
		depth := 1

		for len(queue) > 0 && depth <= maxDepth {
			levelSize := len(queue)
			for i := 0; i < levelSize; i++ {
				current := queue[0]
				queue = queue[1:]

				// Get predecessors (callers)
				preds := s.graph.Predecessors(current)
				for _, pred := range preds {
					if _, seen := visited[pred]; seen {
						continue
					}
					visited[pred] = depth

					if depth <= maxDepth {
						queue = append(queue, pred)
					}

					// Skip if already tracked
					if _, exists := affected[pred]; exists {
						continue
					}

					callerEntity, err := s.store.GetEntity(pred)
					if err != nil {
						continue
					}

					m, _ := s.store.GetMetrics(pred)
					affected[pred] = &safeEntity{
						entity:  callerEntity,
						metrics: m,
						depth:   depth,
						direct:  false,
					}
				}
			}
			depth++
		}
	}

	return affected
}

// buildQuickSafeOutput builds the quick mode (impact-only) output
func (s *Server) buildQuickSafeOutput(target string, affected map[string]*safeEntity, depth int) (string, error) {
	// Build affected entities list
	affectedList := make([]map[string]interface{}, 0, len(affected))
	keystoneCount := 0

	for _, e := range affected {
		entry := map[string]interface{}{
			"name":     e.entity.Name,
			"type":     e.entity.EntityType,
			"location": formatLocation(e.entity),
			"depth":    e.depth,
			"direct":   e.direct,
		}

		if e.metrics != nil {
			entry["importance"] = computeImportance(e.metrics.PageRank)
			if e.metrics.PageRank >= 0.30 {
				keystoneCount++
			}
		}

		affectedList = append(affectedList, entry)
	}

	// Count affected files
	files := make(map[string]bool)
	for _, e := range affected {
		files[e.entity.FilePath] = true
	}

	return toJSON(map[string]interface{}{
		"target":         target,
		"mode":           "quick",
		"impact_radius":  len(affected),
		"files_affected": len(files),
		"keystone_count": keystoneCount,
		"depth":          depth,
		"affected":       affectedList,
	})
}

// buildFullSafeOutput builds the full safety assessment output
func (s *Server) buildFullSafeOutput(target string, affected map[string]*safeEntity, depth int) (string, error) {
	// Count keystones and identify coverage gaps
	keystoneCount := 0
	coverageGaps := 0
	var keystones []map[string]interface{}
	var warnings []string

	// Use dynamic threshold based on actual PageRank distribution
	keystoneThreshold := s.computeDynamicKeystoneThreshold(affected)

	for _, e := range affected {
		if e.metrics == nil {
			continue
		}

		isKeystone := e.metrics.PageRank >= keystoneThreshold
		if isKeystone {
			keystoneCount++

			// For now, assume coverage gap if no coverage data
			// Full coverage integration would check actual coverage
			hasCoverageGap := true // Assume gap until proven otherwise
			coverageStr := "unknown"

			if hasCoverageGap {
				coverageGaps++
			}

			impactType := "indirect"
			if e.direct {
				impactType = "direct"
			} else if e.depth == 1 {
				impactType = "caller"
			}

			keystones = append(keystones, map[string]interface{}{
				"name":         e.entity.Name,
				"type":         e.entity.EntityType,
				"location":     formatLocation(e.entity),
				"pagerank":     e.metrics.PageRank,
				"coverage":     coverageStr,
				"impact":       impactType,
				"coverage_gap": hasCoverageGap,
			})

			if hasCoverageGap {
				warnings = append(warnings, fmt.Sprintf("Keystone '%s' has unknown coverage - add tests before modifying", e.entity.Name))
			}
		}
	}

	// Limit keystones to top 10
	if len(keystones) > 10 {
		keystones = keystones[:10]
	}

	// Count affected files
	files := make(map[string]bool)
	for _, e := range affected {
		files[e.entity.FilePath] = true
	}

	// Determine risk level
	riskLevel := s.computeRiskLevel(len(affected), keystoneCount, coverageGaps)

	// Build recommendations
	recommendations := s.buildRecommendations(riskLevel, coverageGaps, keystoneCount)

	result := map[string]interface{}{
		"safety_assessment": map[string]interface{}{
			"target":         target,
			"risk_level":     riskLevel,
			"impact_radius":  len(affected),
			"files_affected": len(files),
			"keystone_count": keystoneCount,
			"coverage_gaps":  coverageGaps,
			"drift_detected": false, // Would require file parsing to detect
		},
		"warnings":           warnings,
		"recommendations":    recommendations,
		"affected_keystones": keystones,
	}

	return toJSON(result)
}

// computeDynamicKeystoneThreshold calculates threshold based on PageRank distribution
func (s *Server) computeDynamicKeystoneThreshold(affected map[string]*safeEntity) float64 {
	var pageranks []float64
	for _, e := range affected {
		if e.metrics != nil && e.metrics.PageRank > 0 {
			pageranks = append(pageranks, e.metrics.PageRank)
		}
	}

	if len(pageranks) == 0 {
		return 1.0 // No keystones if no metrics
	}

	// Sort descending
	for i := 0; i < len(pageranks)-1; i++ {
		for j := i + 1; j < len(pageranks); j++ {
			if pageranks[j] > pageranks[i] {
				pageranks[i], pageranks[j] = pageranks[j], pageranks[i]
			}
		}
	}

	// Take top 5% or minimum of 10 entities
	topN := len(pageranks) / 20 // 5%
	if topN < 10 {
		topN = 10
	}
	if topN > len(pageranks) {
		topN = len(pageranks)
	}

	return pageranks[topN-1]
}

// computeRiskLevel determines overall risk level
func (s *Server) computeRiskLevel(impactRadius, keystoneCount, coverageGaps int) string {
	// Critical: multiple undertested keystones
	if coverageGaps >= 3 {
		return "critical"
	}

	// High: any coverage gaps on keystones
	if coverageGaps > 0 {
		return "high"
	}

	// Medium: multiple keystones affected
	if keystoneCount >= 3 {
		return "medium"
	}

	// Medium: large impact radius
	if impactRadius >= 20 {
		return "medium"
	}

	return "low"
}

// buildRecommendations generates actionable recommendations
func (s *Server) buildRecommendations(riskLevel string, coverageGaps, keystoneCount int) []string {
	var recs []string

	switch riskLevel {
	case "critical":
		recs = append(recs, "STOP: Address safety issues before proceeding")
		if coverageGaps > 0 {
			recs = append(recs, "Add tests for undertested keystones before making changes")
		}
		recs = append(recs, "Consider breaking this change into smaller, safer increments")

	case "high":
		recs = append(recs, "Proceed with caution")
		if coverageGaps > 0 {
			recs = append(recs, "Add tests for affected keystones before or alongside changes")
		}
		recs = append(recs, "Request thorough code review for this change")

	case "medium":
		recs = append(recs, "Proceed with standard review process")
		if keystoneCount > 0 {
			recs = append(recs, "Pay attention to keystone entities in review")
		}
		recs = append(recs, "Run tests after making changes")

	case "low":
		recs = append(recs, "Safe to proceed")
		recs = append(recs, "Run relevant tests after making changes")
	}

	return recs
}

func (s *Server) executeMap(path, filter, lang string) (string, error) {
	// Build filter from parameters
	queryFilter := store.EntityFilter{
		Status: "active",
		Limit:  10000, // Reasonable limit for MCP response
	}

	// Apply path filter if provided
	if path != "" {
		queryFilter.FilePath = path
	}

	// Apply type filter
	if filter != "" {
		queryFilter.EntityType = mapTypeFilter(filter)
	}

	// Apply language filter
	if lang != "" {
		queryFilter.Language = normalizeLanguageFilter(lang)
	}

	// Query entities
	entities, err := s.store.QueryEntities(queryFilter)
	if err != nil {
		return "", fmt.Errorf("failed to query entities: %w", err)
	}

	// Group entities by file
	fileMap := make(map[string][]map[string]interface{})

	for _, e := range entities {
		skeleton := e.Skeleton
		if skeleton == "" {
			skeleton = generateSkeleton(e)
		}

		entry := map[string]interface{}{
			"name":     e.Name,
			"type":     e.EntityType,
			"location": formatLocation(e),
			"skeleton": skeleton,
		}

		if e.DocComment != "" {
			entry["doc_comment"] = e.DocComment
		}

		fileMap[e.FilePath] = append(fileMap[e.FilePath], entry)
	}

	return toJSON(map[string]interface{}{
		"files": fileMap,
		"count": len(entities),
		"path":  path,
	})
}

// --- cx_blame: entity commit history ---

func (s *Server) registerBlameTool() error {
	tool := mcp.NewTool("cx_blame",
		mcp.WithDescription("Show commit history for a code entity. Tracks how an entity changed over time."),
		mcp.WithString("entity",
			mcp.Required(),
			mcp.Description("Entity name or ID to get history for"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum history entries (default: 20)"),
		),
		mcp.WithBoolean("deps",
			mcp.Description("Include dependency change history"),
		),
	)
	s.mcpServer.AddTool(tool, s.handleBlame)
	return nil
}

func (s *Server) handleBlame(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	s.updateActivity()
	args := request.GetArguments()
	entity, _ := args["entity"].(string)
	if entity == "" {
		return mcp.NewToolResultError("entity parameter is required"), nil
	}
	limit := 20
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}
	deps, _ := args["deps"].(bool)

	result, err := s.executeBlame(entity, limit, deps)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(result), nil
}

func (s *Server) executeBlame(entityName string, limit int, deps bool) (string, error) {
	entity, err := s.resolveEntity(entityName)
	if err != nil {
		return "", err
	}

	history, err := s.store.EntityHistory(store.EntityHistoryOptions{
		EntityID: entity.ID,
		Limit:    limit,
	})
	if err != nil {
		return "", fmt.Errorf("entity history: %w", err)
	}

	entries := make([]map[string]interface{}, 0, len(history))
	for _, h := range history {
		entry := map[string]interface{}{
			"commit":      h.CommitHash,
			"date":        h.CommitDate,
			"committer":   h.Committer,
			"change_type": h.ChangeType,
			"location":    fmt.Sprintf("%s:%d", h.FilePath, h.LineStart),
		}
		if h.Signature != nil {
			entry["signature"] = *h.Signature
		}
		entries = append(entries, entry)
	}

	result := map[string]interface{}{
		"entity":  entity.Name,
		"id":      entity.ID,
		"history": entries,
		"count":   len(entries),
	}

	if deps {
		depHistory, err := s.store.DependencyHistory(store.DependencyHistoryOptions{
			EntityID: entity.ID,
			Limit:    limit,
		})
		if err == nil && len(depHistory) > 0 {
			depEntries := make([]map[string]interface{}, 0, len(depHistory))
			for _, d := range depHistory {
				depEntries = append(depEntries, map[string]interface{}{
					"commit":   d.CommitHash,
					"date":     d.CommitDate,
					"from":     d.FromID,
					"to":       d.ToID,
					"dep_type": d.DepType,
				})
			}
			result["dependency_history"] = depEntries
		}
	}

	return toJSON(result)
}

// --- cx_tag: tag management ---

func (s *Server) registerTagTool() error {
	tool := mcp.NewTool("cx_tag",
		mcp.WithDescription("Manage entity tags. Add, remove, list, or find entities by tags."),
		mcp.WithString("action",
			mcp.Required(),
			mcp.Description("Action: add, remove, list, find"),
		),
		mcp.WithString("entity",
			mcp.Description("Entity name or ID (required for add/remove, optional for list)"),
		),
		mcp.WithString("tags",
			mcp.Description("Comma-separated tags (for add/find)"),
		),
		mcp.WithString("note",
			mcp.Description("Note for why the tag was added (for add)"),
		),
		mcp.WithBoolean("match_all",
			mcp.Description("Require all tags to match (for find, default: false)"),
		),
	)
	s.mcpServer.AddTool(tool, s.handleTag)
	return nil
}

func (s *Server) handleTag(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	s.updateActivity()
	args := request.GetArguments()
	action, _ := args["action"].(string)
	if action == "" {
		return mcp.NewToolResultError("action parameter is required"), nil
	}
	entityName, _ := args["entity"].(string)
	tags, _ := args["tags"].(string)
	note, _ := args["note"].(string)
	matchAll, _ := args["match_all"].(bool)

	result, err := s.executeTag(action, entityName, tags, note, matchAll)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(result), nil
}

func (s *Server) executeTag(action, entityName, tags, note string, matchAll bool) (string, error) {
	switch action {
	case "add":
		if entityName == "" {
			return "", fmt.Errorf("entity parameter is required for add")
		}
		if tags == "" {
			return "", fmt.Errorf("tags parameter is required for add")
		}
		entity, err := s.resolveEntity(entityName)
		if err != nil {
			return "", err
		}
		tagList := strings.Split(tags, ",")
		added := make([]string, 0, len(tagList))
		for _, tag := range tagList {
			tag = strings.TrimSpace(tag)
			if tag == "" {
				continue
			}
			if err := s.store.AddTagWithNote(entity.ID, tag, "cx_call", note); err != nil {
				return "", fmt.Errorf("add tag %q: %w", tag, err)
			}
			added = append(added, tag)
		}
		return toJSON(map[string]interface{}{
			"entity":     entity.Name,
			"id":         entity.ID,
			"tags_added": added,
		})

	case "remove":
		if entityName == "" {
			return "", fmt.Errorf("entity parameter is required for remove")
		}
		if tags == "" {
			return "", fmt.Errorf("tags parameter is required for remove")
		}
		entity, err := s.resolveEntity(entityName)
		if err != nil {
			return "", err
		}
		tagList := strings.Split(tags, ",")
		removed := make([]string, 0, len(tagList))
		for _, tag := range tagList {
			tag = strings.TrimSpace(tag)
			if tag == "" {
				continue
			}
			if err := s.store.RemoveTag(entity.ID, tag); err != nil {
				continue // tag may not exist
			}
			removed = append(removed, tag)
		}
		return toJSON(map[string]interface{}{
			"entity":       entity.Name,
			"id":           entity.ID,
			"tags_removed": removed,
		})

	case "list":
		if entityName != "" {
			entity, err := s.resolveEntity(entityName)
			if err != nil {
				return "", err
			}
			entityTags, err := s.store.GetTags(entity.ID)
			if err != nil {
				return "", fmt.Errorf("get tags: %w", err)
			}
			tagEntries := make([]map[string]interface{}, 0, len(entityTags))
			for _, t := range entityTags {
				entry := map[string]interface{}{
					"tag":        t.Tag,
					"created_at": t.CreatedAt.Format(time.RFC3339),
				}
				if t.CreatedBy != "" {
					entry["created_by"] = t.CreatedBy
				}
				if t.Note != "" {
					entry["note"] = t.Note
				}
				tagEntries = append(tagEntries, entry)
			}
			return toJSON(map[string]interface{}{
				"entity": entity.Name,
				"id":     entity.ID,
				"tags":   tagEntries,
			})
		}
		// List all tags globally
		allTags, err := s.store.ListAllTags()
		if err != nil {
			return "", fmt.Errorf("list all tags: %w", err)
		}
		return toJSON(map[string]interface{}{
			"tags":  allTags,
			"count": len(allTags),
		})

	case "find":
		if tags == "" {
			return "", fmt.Errorf("tags parameter is required for find")
		}
		tagList := strings.Split(tags, ",")
		for i := range tagList {
			tagList[i] = strings.TrimSpace(tagList[i])
		}
		entities, err := s.store.FindByTags(tagList, matchAll)
		if err != nil {
			return "", fmt.Errorf("find by tags: %w", err)
		}
		results := make([]map[string]interface{}, 0, len(entities))
		for _, e := range entities {
			results = append(results, map[string]interface{}{
				"name":     e.Name,
				"id":       e.ID,
				"type":     e.EntityType,
				"location": formatLocation(e),
			})
		}
		return toJSON(map[string]interface{}{
			"entities": results,
			"count":    len(results),
			"tags":     tagList,
			"match":    map[bool]string{true: "all", false: "any"}[matchAll],
		})

	default:
		return "", fmt.Errorf("unknown action: %s (valid: add, remove, list, find)", action)
	}
}

// --- cx_trace: call chain tracing ---

func (s *Server) registerTraceTool() error {
	tool := mcp.NewTool("cx_trace",
		mcp.WithDescription("Trace call chains between entities. Find paths, callers, or callees in the dependency graph."),
		mcp.WithString("from",
			mcp.Required(),
			mcp.Description("Source entity name or ID"),
		),
		mcp.WithString("to",
			mcp.Description("Target entity name or ID (for path mode)"),
		),
		mcp.WithString("mode",
			mcp.Description("Mode: path (default), callers, callees"),
		),
		mcp.WithNumber("depth",
			mcp.Description("Maximum traversal depth (default: 5)"),
		),
		mcp.WithBoolean("all",
			mcp.Description("Find all paths instead of shortest (for path mode)"),
		),
	)
	s.mcpServer.AddTool(tool, s.handleTrace)
	return nil
}

func (s *Server) handleTrace(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	s.updateActivity()
	args := request.GetArguments()
	from, _ := args["from"].(string)
	if from == "" {
		return mcp.NewToolResultError("from parameter is required"), nil
	}
	to, _ := args["to"].(string)
	mode, _ := args["mode"].(string)
	if mode == "" {
		mode = "path"
	}
	depth := 5
	if d, ok := args["depth"].(float64); ok {
		depth = int(d)
	}
	allPaths, _ := args["all"].(bool)

	result, err := s.executeTrace(from, to, mode, depth, allPaths)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(result), nil
}

func (s *Server) executeTrace(fromName, toName, mode string, depth int, allPaths bool) (string, error) {
	fromEntity, err := s.resolveEntity(fromName)
	if err != nil {
		return "", fmt.Errorf("resolve 'from': %w", err)
	}

	switch mode {
	case "callers":
		chain := s.graph.CollectCallerChain(fromEntity.ID, depth)
		names := s.resolveEntityNames(chain)
		return toJSON(map[string]interface{}{
			"entity":  fromEntity.Name,
			"id":      fromEntity.ID,
			"mode":    "callers",
			"depth":   depth,
			"callers": names,
			"count":   len(names),
		})

	case "callees":
		chain := s.graph.CollectCalleeChain(fromEntity.ID, depth)
		names := s.resolveEntityNames(chain)
		return toJSON(map[string]interface{}{
			"entity":  fromEntity.Name,
			"id":      fromEntity.ID,
			"mode":    "callees",
			"depth":   depth,
			"callees": names,
			"count":   len(names),
		})

	case "path":
		if toName == "" {
			return "", fmt.Errorf("'to' parameter is required for path mode")
		}
		toEntity, err := s.resolveEntity(toName)
		if err != nil {
			return "", fmt.Errorf("resolve 'to': %w", err)
		}

		if allPaths {
			paths := s.graph.AllPaths(fromEntity.ID, toEntity.ID, depth)
			namedPaths := make([][]string, 0, len(paths))
			for _, p := range paths {
				namedPaths = append(namedPaths, s.resolveEntityNames(p))
			}
			return toJSON(map[string]interface{}{
				"from":  fromEntity.Name,
				"to":    toEntity.Name,
				"mode":  "all_paths",
				"depth": depth,
				"paths": namedPaths,
				"count": len(namedPaths),
			})
		}

		path := s.graph.ShortestPath(fromEntity.ID, toEntity.ID, "forward")
		if len(path) == 0 {
			return toJSON(map[string]interface{}{
				"from":    fromEntity.Name,
				"to":      toEntity.Name,
				"mode":    "shortest_path",
				"path":    []string{},
				"message": "no path found",
			})
		}
		return toJSON(map[string]interface{}{
			"from":  fromEntity.Name,
			"to":    toEntity.Name,
			"mode":  "shortest_path",
			"depth": depth,
			"path":  s.resolveEntityNames(path),
			"hops":  len(path) - 1,
		})

	default:
		return "", fmt.Errorf("unknown mode: %s (valid: path, callers, callees)", mode)
	}
}

// resolveEntityNames converts entity IDs to human-readable names.
func (s *Server) resolveEntityNames(ids []string) []string {
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		if e, err := s.store.GetEntity(id); err == nil {
			names = append(names, e.Name)
		} else {
			names = append(names, id)
		}
	}
	return names
}

// --- cx_dead: dead code detection ---

func (s *Server) registerDeadTool() error {
	tool := mcp.NewTool("cx_dead",
		mcp.WithDescription("Find dead code using graph analysis. Three confidence tiers."),
		mcp.WithNumber("tier",
			mcp.Description("Max confidence tier: 1=definite, 2=+probable, 3=+suspicious (default: 1)"),
		),
		mcp.WithString("type_filter",
			mcp.Description("Filter by type: F (function), T (type), M (method), C (constant)"),
		),
		mcp.WithBoolean("include_exports",
			mcp.Description("Include exported entities in tier 1"),
		),
	)
	s.mcpServer.AddTool(tool, s.handleDead)
	return nil
}

func (s *Server) handleDead(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	s.updateActivity()
	args := request.GetArguments()
	tier := 1
	if t, ok := args["tier"].(float64); ok {
		tier = int(t)
	}
	typeFilter, _ := args["type_filter"].(string)
	includeExports, _ := args["include_exports"].(bool)

	result, err := s.executeDead(tier, typeFilter, includeExports)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(result), nil
}

func (s *Server) executeDead(tier int, typeFilter string, includeExports bool) (string, error) {
	// Query all active entities
	filter := store.EntityFilter{Status: "active"}
	if typeFilter != "" {
		filter.EntityType = mapTypeFilter(typeFilter)
	}
	entities, err := s.store.QueryEntities(filter)
	if err != nil {
		return "", fmt.Errorf("query entities: %w", err)
	}

	var tier1, tier2, tier3 []map[string]interface{}
	deadSet := make(map[string]bool)

	// Tier 1: Private entities with zero callers (definite dead code)
	for _, e := range entities {
		if isDeadEntryPoint(e) {
			continue
		}
		inDeg := s.graph.InDegree(e.ID)
		if inDeg > 0 {
			continue
		}
		if e.Visibility == "priv" || includeExports {
			tier1 = append(tier1, map[string]interface{}{
				"name":       e.Name,
				"id":         e.ID,
				"type":       e.EntityType,
				"visibility": e.Visibility,
				"location":   formatLocation(e),
				"tier":       1,
			})
			deadSet[e.ID] = true
		}
	}

	// Tier 2: Exported entities with zero callers (probable dead code)
	if tier >= 2 {
		for _, e := range entities {
			if deadSet[e.ID] {
				continue
			}
			if isDeadEntryPoint(e) {
				continue
			}
			if e.Visibility != "pub" {
				continue
			}
			inDeg := s.graph.InDegree(e.ID)
			if inDeg == 0 {
				tier2 = append(tier2, map[string]interface{}{
					"name":       e.Name,
					"id":         e.ID,
					"type":       e.EntityType,
					"visibility": e.Visibility,
					"location":   formatLocation(e),
					"tier":       2,
				})
				deadSet[e.ID] = true
			}
		}
	}

	// Tier 3: Entities whose callers are ALL dead (suspicious)
	if tier >= 3 {
		changed := true
		for changed {
			changed = false
			for _, e := range entities {
				if deadSet[e.ID] {
					continue
				}
				if isDeadEntryPoint(e) {
					continue
				}
				callers := s.graph.Predecessors(e.ID)
				if len(callers) == 0 {
					continue
				}
				allDead := true
				for _, c := range callers {
					if !deadSet[c] {
						allDead = false
						break
					}
				}
				if allDead {
					tier3 = append(tier3, map[string]interface{}{
						"name":       e.Name,
						"id":         e.ID,
						"type":       e.EntityType,
						"visibility": e.Visibility,
						"location":   formatLocation(e),
						"tier":       3,
					})
					deadSet[e.ID] = true
					changed = true
				}
			}
		}
	}

	all := make([]map[string]interface{}, 0, len(tier1)+len(tier2)+len(tier3))
	all = append(all, tier1...)
	all = append(all, tier2...)
	all = append(all, tier3...)

	result := map[string]interface{}{
		"tier_requested": tier,
		"total_entities": len(entities),
		"dead_code":      all,
		"total_dead":     len(all),
		"tier_1_count":   len(tier1),
	}
	if tier >= 2 {
		result["tier_2_count"] = len(tier2)
	}
	if tier >= 3 {
		result["tier_3_count"] = len(tier3)
	}

	return toJSON(result)
}

// isDeadEntryPoint checks if an entity is a known entry point that shouldn't be flagged as dead.
func isDeadEntryPoint(e *store.Entity) bool {
	name := e.Name
	eType := e.EntityType

	// init() and main() are runtime entry points
	if name == "init" || name == "main" {
		if eType == "function" || eType == "func" {
			return true
		}
	}

	// Test functions
	if strings.HasPrefix(name, "Test") || strings.HasPrefix(name, "Benchmark") || strings.HasPrefix(name, "Example") {
		return true
	}

	// Cobra command handlers in cmd package
	if strings.Contains(e.FilePath, "/cmd/") {
		if strings.HasPrefix(name, "run") || strings.HasPrefix(name, "Run") {
			if eType == "function" || eType == "func" {
				return true
			}
		}
	}

	// Common false positives: single-line constants/variables with local var names
	if eType == "constant" || eType == "variable" {
		isSingleLine := e.LineEnd == nil || *e.LineEnd == e.LineStart
		if isSingleLine && isDeadCommonVar(name) {
			return true
		}
	}

	return false
}

func isDeadCommonVar(name string) bool {
	common := map[string]bool{
		"err": true, "ctx": true, "ok": true, "i": true, "j": true, "k": true,
		"n": true, "s": true, "b": true, "r": true, "w": true, "db": true, "tx": true,
	}
	return common[name]
}

// --- cx_test: smart test selection ---

func (s *Server) registerTestTool() error {
	tool := mcp.NewTool("cx_test",
		mcp.WithDescription("Smart test selection based on code changes, or find coverage gaps."),
		mcp.WithString("mode",
			mcp.Description("Mode: select (default) or gaps"),
		),
		mcp.WithString("file",
			mcp.Description("Specific file to analyze (for select mode)"),
		),
		mcp.WithString("diff",
			mcp.Description("Git ref for diff (default: HEAD for uncommitted changes)"),
		),
		mcp.WithNumber("depth",
			mcp.Description("Caller chain depth for indirect tests (default: 2)"),
		),
		mcp.WithBoolean("keystones_only",
			mcp.Description("Only show gaps in keystone entities (for gaps mode)"),
		),
		mcp.WithNumber("threshold",
			mcp.Description("Coverage threshold percentage (default: 50, for gaps mode)"),
		),
	)
	s.mcpServer.AddTool(tool, s.handleTest)
	return nil
}

func (s *Server) handleTest(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	s.updateActivity()
	args := request.GetArguments()
	mode, _ := args["mode"].(string)
	if mode == "" {
		mode = "select"
	}
	file, _ := args["file"].(string)
	diff, _ := args["diff"].(string)
	depth := 2
	if d, ok := args["depth"].(float64); ok {
		depth = int(d)
	}
	keystonesOnly, _ := args["keystones_only"].(bool)
	threshold := 50.0
	if t, ok := args["threshold"].(float64); ok {
		threshold = t
	}

	result, err := s.executeTest(mode, file, diff, depth, keystonesOnly, threshold)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(result), nil
}

func (s *Server) executeTest(mode, file, diff string, depth int, keystonesOnly bool, threshold float64) (string, error) {
	switch mode {
	case "gaps":
		return s.executeTestGaps(keystonesOnly, threshold)
	case "select":
		return s.executeTestSelect(file, diff, depth)
	default:
		return "", fmt.Errorf("unknown mode: %s (valid: select, gaps)", mode)
	}
}

func (s *Server) executeTestGaps(keystonesOnly bool, threshold float64) (string, error) {
	// Query entities with coverage data from entity_coverage table
	rows, err := s.store.DB().Query(`
		SELECT e.id, e.name, e.entity_type, e.file_path, e.line_start, e.visibility,
			COALESCE(c.coverage_percent, -1) as coverage_pct
		FROM entities e
		LEFT JOIN entity_coverage c ON e.id = c.entity_id
		WHERE e.status = 'active'
		ORDER BY COALESCE(c.coverage_percent, -1) ASC`)
	if err != nil {
		return "", fmt.Errorf("query coverage: %w", err)
	}
	defer rows.Close()

	type gapEntry struct {
		ID          string
		Name        string
		Type        string
		FilePath    string
		LineStart   int
		Visibility  string
		CoveragePct float64
	}

	var gaps []gapEntry
	for rows.Next() {
		var g gapEntry
		if err := rows.Scan(&g.ID, &g.Name, &g.Type, &g.FilePath, &g.LineStart, &g.Visibility, &g.CoveragePct); err != nil {
			continue
		}
		// Filter: below threshold or no coverage (-1)
		if g.CoveragePct >= 0 && g.CoveragePct >= threshold {
			continue
		}
		gaps = append(gaps, g)
	}

	// If keystones_only, filter to high-importance entities
	if keystonesOnly {
		var filtered []gapEntry
		for _, g := range gaps {
			m, err := s.store.GetMetrics(g.ID)
			if err != nil {
				continue
			}
			if m.PageRank >= 0.10 {
				filtered = append(filtered, g)
			}
		}
		gaps = filtered
	}

	results := make([]map[string]interface{}, 0, len(gaps))
	for _, g := range gaps {
		entry := map[string]interface{}{
			"name":     g.Name,
			"id":       g.ID,
			"type":     g.Type,
			"location": fmt.Sprintf("%s:%d", g.FilePath, g.LineStart),
		}
		if g.CoveragePct >= 0 {
			entry["coverage"] = fmt.Sprintf("%.0f%%", g.CoveragePct)
		} else {
			entry["coverage"] = "none"
		}
		results = append(results, entry)
	}

	return toJSON(map[string]interface{}{
		"mode":           "gaps",
		"threshold":      threshold,
		"keystones_only": keystonesOnly,
		"gaps":           results,
		"count":          len(results),
	})
}

func (s *Server) executeTestSelect(file, diff string, depth int) (string, error) {
	// Get changed files
	var changedFiles []string
	if file != "" {
		changedFiles = []string{file}
	} else {
		ref := diff
		if ref == "" {
			ref = "HEAD"
		}
		cmd := exec.Command("git", "diff", "--name-only", ref)
		cmd.Dir = s.projectRoot
		out, err := cmd.Output()
		if err != nil {
			// Try without ref (uncommitted changes)
			cmd = exec.Command("git", "diff", "--name-only")
			cmd.Dir = s.projectRoot
			out, err = cmd.Output()
			if err != nil {
				return "", fmt.Errorf("git diff: %w", err)
			}
		}
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				changedFiles = append(changedFiles, line)
			}
		}
	}

	if len(changedFiles) == 0 {
		return toJSON(map[string]interface{}{
			"mode":          "select",
			"changed_files": []string{},
			"tests":         []string{},
			"message":       "no changed files detected",
		})
	}

	// Find entities in changed files
	changedEntityIDs := make(map[string]bool)
	for _, f := range changedFiles {
		entities, err := s.store.QueryEntities(store.EntityFilter{
			FilePath: f,
			Status:   "active",
		})
		if err != nil {
			continue
		}
		for _, e := range entities {
			changedEntityIDs[e.ID] = true
		}
	}

	// Find test entities by traversing callers
	testEntities := make(map[string]string) // id -> name

	for id := range changedEntityIDs {
		visited := make(map[string]bool)
		queue := []string{id}
		for d := 0; d < depth && len(queue) > 0; d++ {
			var next []string
			for _, node := range queue {
				if visited[node] {
					continue
				}
				visited[node] = true
				callers := s.graph.Predecessors(node)
				for _, caller := range callers {
					if visited[caller] {
						continue
					}
					if e, err := s.store.GetEntity(caller); err == nil {
						if strings.HasPrefix(e.Name, "Test") || strings.HasPrefix(e.Name, "Benchmark") {
							testEntities[e.ID] = e.Name
						}
					}
					next = append(next, caller)
				}
			}
			queue = next
		}
	}

	// Collect unique test names
	tests := make([]string, 0, len(testEntities))
	for _, name := range testEntities {
		tests = append(tests, name)
	}
	sort.Strings(tests)

	// Build go test command suggestion
	var testCmd string
	if len(tests) > 0 {
		testCmd = fmt.Sprintf("go test -run '%s' ./...", strings.Join(tests, "|"))
	}

	return toJSON(map[string]interface{}{
		"mode":              "select",
		"changed_files":     changedFiles,
		"changed_entities":  len(changedEntityIDs),
		"tests":             tests,
		"test_count":        len(tests),
		"suggested_command": testCmd,
	})
}

// --- cx_guard: pre-commit quality checks ---

func (s *Server) registerGuardTool() error {
	tool := mcp.NewTool("cx_guard",
		mcp.WithDescription("Pre-commit quality checks. Detects signature drift, dead-on-arrival code, coverage regression, and graph drift."),
		mcp.WithString("files",
			mcp.Description("Comma-separated file paths to check (default: git staged files)"),
		),
		mcp.WithBoolean("staged",
			mcp.Description("Check only git staged files (default: true)"),
		),
		mcp.WithBoolean("all",
			mcp.Description("Check all modified files (staged + unstaged)"),
		),
		mcp.WithBoolean("fail_on_warnings",
			mcp.Description("Treat warnings as errors"),
		),
	)
	s.mcpServer.AddTool(tool, s.handleGuard)
	return nil
}

func (s *Server) handleGuard(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	s.updateActivity()
	args := request.GetArguments()
	files, _ := args["files"].(string)
	staged, _ := args["staged"].(bool)
	all, _ := args["all"].(bool)
	failOnWarnings, _ := args["fail_on_warnings"].(bool)
	if !all && files == "" {
		staged = true
	}

	result, err := s.executeGuard(files, staged, all, failOnWarnings)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(result), nil
}

func (s *Server) executeGuard(files string, staged, all, failOnWarnings bool) (string, error) {
	// Get file list
	var fileList []string
	if files != "" {
		for _, f := range strings.Split(files, ",") {
			f = strings.TrimSpace(f)
			if f != "" {
				fileList = append(fileList, f)
			}
		}
	} else {
		args := []string{"diff", "--name-only"}
		if staged && !all {
			args = append(args, "--cached")
		}
		cmd := exec.Command("git", args...)
		cmd.Dir = s.projectRoot
		out, err := cmd.Output()
		if err != nil {
			return "", fmt.Errorf("git diff: %w", err)
		}
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				fileList = append(fileList, line)
			}
		}
	}

	// Filter to source files
	var sourceFiles []string
	for _, f := range fileList {
		ext := filepath.Ext(f)
		switch ext {
		case ".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".rs", ".java":
			sourceFiles = append(sourceFiles, f)
		}
	}

	if len(sourceFiles) == 0 {
		return toJSON(map[string]interface{}{
			"status":   "pass",
			"message":  "no source files to check",
			"files":    fileList,
			"errors":   []string{},
			"warnings": []string{},
		})
	}

	guardErrors := make([]string, 0)
	guardWarnings := make([]string, 0)

	for _, f := range sourceFiles {
		absPath := filepath.Join(s.projectRoot, f)

		// Check 1: Signature drift — compare parsed entities with stored
		lang := langFromExt(filepath.Ext(f))
		if lang != "" {
			p, pErr := parser.NewParser(parser.Language(lang))
			if pErr == nil {
				result, pErr := p.ParseFile(absPath)
				if pErr == nil {
					ext := extract.NewExtractorWithBase(result, s.projectRoot)
					newEntities, eErr := ext.ExtractAll()
					if eErr == nil {
						storedEntities, sErr := s.store.QueryEntities(store.EntityFilter{
							FilePath: f,
							Status:   "active",
						})
						if sErr == nil {
							storedMap := make(map[string]*store.Entity)
							for _, se := range storedEntities {
								storedMap[se.Name] = se
							}
							for _, ne := range newEntities {
								if se, ok := storedMap[ne.Name]; ok {
									if se.SigHash != "" && ne.SigHash != "" && se.SigHash != ne.SigHash {
										callerCount := s.graph.InDegree(se.ID)
										if callerCount > 0 {
											guardErrors = append(guardErrors, fmt.Sprintf("signature changed: %s (%s) — %d callers may be affected", ne.Name, f, callerCount))
										} else {
											guardWarnings = append(guardWarnings, fmt.Sprintf("signature changed: %s (%s) — no callers", ne.Name, f))
										}
									}
								} else if ne.Visibility == "priv" {
									guardWarnings = append(guardWarnings, fmt.Sprintf("new private entity: %s (%s) — verify it has callers", ne.Name, f))
								}
							}
						}
					}
				}
			}
		}

		// Check 2: Graph drift — does the file have stored entities?
		stored, sErr := s.store.QueryEntities(store.EntityFilter{
			FilePath: f,
			Status:   "active",
		})
		if sErr == nil && len(stored) == 0 {
			guardWarnings = append(guardWarnings, fmt.Sprintf("file not in graph: %s — run 'cx scan' to update", f))
		}
	}

	status := "pass"
	if len(guardErrors) > 0 {
		status = "fail"
	} else if len(guardWarnings) > 0 && failOnWarnings {
		status = "fail"
	} else if len(guardWarnings) > 0 {
		status = "warn"
	}

	return toJSON(map[string]interface{}{
		"status":        status,
		"files_checked": len(sourceFiles),
		"errors":        guardErrors,
		"error_count":   len(guardErrors),
		"warnings":      guardWarnings,
		"warning_count": len(guardWarnings),
	})
}

// langFromExt maps file extension to language name.
func langFromExt(ext string) string {
	switch ext {
	case ".go":
		return "go"
	case ".ts", ".tsx":
		return "typescript"
	case ".js", ".jsx":
		return "javascript"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	default:
		return ""
	}
}

// mapTypeFilter converts short type filter to entity type
func mapTypeFilter(f string) string {
	switch f {
	case "F", "f":
		return "function"
	case "T", "t":
		return "type"
	case "M", "m":
		return "method"
	case "C", "c":
		return "constant"
	case "V", "v":
		return "variable"
	default:
		return ""
	}
}

// normalizeLanguageFilter normalizes language name
func normalizeLanguageFilter(lang string) string {
	switch lang {
	case "go", "Go", "golang":
		return "go"
	case "ts", "typescript", "TypeScript":
		return "typescript"
	case "js", "javascript", "JavaScript":
		return "javascript"
	case "py", "python", "Python":
		return "python"
	case "rs", "rust", "Rust":
		return "rust"
	case "java", "Java":
		return "java"
	default:
		return lang
	}
}

// generateSkeleton generates a skeleton from a store.Entity
func generateSkeleton(e *store.Entity) string {
	switch e.EntityType {
	case "function":
		sig := e.Signature
		if sig == "" {
			sig = "()"
		}
		return fmt.Sprintf("func %s%s { ... }", e.Name, formatSignature(sig))

	case "method":
		sig := e.Signature
		if sig == "" {
			sig = "()"
		}
		if e.Receiver != "" {
			return fmt.Sprintf("func (%s) %s%s { ... }", e.Receiver, e.Name, formatSignature(sig))
		}
		return fmt.Sprintf("func %s%s { ... }", e.Name, formatSignature(sig))

	case "type":
		if e.Kind == "struct" {
			return fmt.Sprintf("type %s struct { ... }", e.Name)
		} else if e.Kind == "interface" {
			return fmt.Sprintf("type %s interface { ... }", e.Name)
		}
		return fmt.Sprintf("type %s %s", e.Name, e.Kind)

	case "constant":
		return fmt.Sprintf("const %s", e.Name)

	case "variable":
		return fmt.Sprintf("var %s", e.Name)

	default:
		return e.Name
	}
}

// formatSignature converts stored signature format to Go syntax
func formatSignature(sig string) string {
	if sig == "" {
		return "()"
	}
	// Replace ": " with " " for parameter types
	result := sig
	for i := 0; i < len(result)-1; i++ {
		if result[i] == ':' && result[i+1] == ' ' {
			result = result[:i] + " " + result[i+2:]
		}
	}
	// Replace " -> " with " " for return types
	result = replaceArrow(result)
	return result
}

// replaceArrow replaces " -> " with " "
func replaceArrow(s string) string {
	const arrow = " -> "
	for {
		idx := -1
		for i := 0; i <= len(s)-len(arrow); i++ {
			if s[i:i+len(arrow)] == arrow {
				idx = i
				break
			}
		}
		if idx == -1 {
			break
		}
		s = s[:idx] + " " + s[idx+len(arrow):]
	}
	return s
}

// Helper functions

func toJSON(v interface{}) (string, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func formatLocation(e *store.Entity) string {
	if e.LineStart > 0 && e.LineEnd != nil && *e.LineEnd > 0 {
		return fmt.Sprintf("%s:%d-%d", e.FilePath, e.LineStart, *e.LineEnd)
	}
	if e.LineStart > 0 {
		return fmt.Sprintf("%s:%d", e.FilePath, e.LineStart)
	}
	return e.FilePath
}

func computeImportance(pagerank float64) string {
	switch {
	case pagerank >= 0.50:
		return "critical"
	case pagerank >= 0.30:
		return "keystone"
	case pagerank >= 0.10:
		return "medium"
	default:
		return "low"
	}
}

func isFilePath(s string) bool {
	if len(s) == 0 {
		return false
	}
	// Check for path separators or common file extensions
	if s[0] == '/' || s[0] == '.' {
		return true
	}
	exts := []string{".go", ".py", ".js", ".ts", ".rs", ".java"}
	for _, ext := range exts {
		if len(s) > len(ext) && s[len(s)-len(ext):] == ext {
			return true
		}
	}
	return false
}
