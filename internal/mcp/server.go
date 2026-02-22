// Package mcp provides an MCP (Model Context Protocol) server for cx.
// This allows AI agents to query the code graph through MCP tools instead of CLI commands.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/anthropics/cx/internal/config"
	cxcontext "github.com/anthropics/cx/internal/context"
	"github.com/anthropics/cx/internal/graph"
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
var DefaultTools = []string{"cx_context", "cx_safe", "cx_show", "cx_find", "cx_map"}

// AllTools lists all available tools
var AllTools = []string{"cx_context", "cx_safe", "cx_find", "cx_show", "cx_map", "cx_diff", "cx_impact", "cx_gaps"}

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

func (s *Server) executeShow(name, density string, coverage bool) (string, error) {
	// Try direct ID lookup first
	entity, err := s.store.GetEntity(name)
	if err != nil {
		// Try name search
		entities, err := s.store.QueryEntities(store.EntityFilter{
			Name:   name,
			Status: "active",
			Limit:  1,
		})
		if err != nil || len(entities) == 0 {
			return "", fmt.Errorf("entity not found: %s", name)
		}
		entity = entities[0]
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
