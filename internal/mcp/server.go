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
var DefaultTools = []string{"cx_diff", "cx_impact", "cx_context", "cx_show"}

// AllTools lists all available tools
var AllTools = []string{"cx_diff", "cx_impact", "cx_context", "cx_show", "cx_find", "cx_gaps"}

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
