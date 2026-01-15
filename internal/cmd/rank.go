package cmd

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/anthropics/cx/internal/config"
	"github.com/anthropics/cx/internal/graph"
	"github.com/anthropics/cx/internal/metrics"
	"github.com/anthropics/cx/internal/output"
	"github.com/anthropics/cx/internal/store"
	"github.com/spf13/cobra"
)

// rankCmd represents the rank command
var rankCmd = &cobra.Command{
	Use:   "rank",
	Short: "Rank symbols by importance using PageRank",
	Long: `Compute and display importance metrics in YAML format.

Identifies keystones (highly depended-on) and bottlenecks (central to paths).

The ranking considers:
  - PageRank:    Importance based on dependency graph structure
  - Betweenness: How often the entity lies on shortest paths
  - In-degree:   Number of entities that depend on this one
  - Importance:  Categorized as: keystone | bottleneck | normal | leaf

Output Structure:
  results:  Map of ranked entities with metrics
  count:    Total entities returned

Filtering Modes:
  (default)      Top N entities by PageRank
  --keystones    Show only keystones (pr > 0.30)
  --bottlenecks  Show only bottlenecks (betweenness > 0.20)
  --leaves       Show only leaf nodes (in-degree = 0)

Metrics Included:
  pagerank:      Importance score (0.0 - 1.0+)
  in_degree:     Number of dependents
  out_degree:    Number of dependencies
  importance:    Category (keystone|bottleneck|normal|leaf)
  betweenness:   Centrality measure (dense output only)

Density Effects:
  sparse:   Type, location, PageRank
  medium:   Add all metrics (default)
  dense:    Add extended analysis

Examples:
  cx rank                                     # Show top 20 by PageRank
  cx rank --top 50                            # Show top 50 entities
  cx rank --keystones                         # Critical entities (high in-degree)
  cx rank --bottlenecks                       # Bottleneck entities (high betweenness)
  cx rank --leaves                            # Leaf nodes (no dependents)
  cx rank --density=sparse                    # Minimal metrics
  cx rank --density=dense                     # Full analysis
  cx rank --format=json                       # JSON output
  cx rank --recompute                         # Force recompute all metrics`,
	RunE: runRank,
}

var (
	rankRecompute   bool
	rankTop         int
	rankKeystones   bool
	rankBottlenecks bool
	rankLeaves      bool
)

func init() {
	rootCmd.AddCommand(rankCmd)

	// Rank-specific flags
	rankCmd.Flags().BoolVar(&rankRecompute, "recompute", false, "Force recompute all metrics")
	rankCmd.Flags().IntVar(&rankTop, "top", 20, "Show top N by PageRank")
	rankCmd.Flags().BoolVar(&rankKeystones, "keystones", false, "Show only keystones (pr > 0.30)")
	rankCmd.Flags().BoolVar(&rankBottlenecks, "bottlenecks", false, "Show only high betweenness (betw > 0.20)")
	rankCmd.Flags().BoolVar(&rankLeaves, "leaves", false, "Show only leaf nodes (no dependents)")
}

// rankedEntity holds an entity with its computed metrics
type rankedEntity struct {
	entity      *store.Entity
	pagerank    float64
	betweenness float64
	inDegree    int
	outDegree   int
}

func runRank(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := config.Load(".")
	if err != nil {
		cfg = config.DefaultConfig()
	}

	// Open store
	cxDir, err := config.FindConfigDir(".")
	if err != nil {
		return fmt.Errorf("cx not initialized: run 'cx scan' first")
	}

	storeDB, err := store.Open(cxDir)
	if err != nil {
		return fmt.Errorf("failed to open store: %w", err)
	}
	defer storeDB.Close()

	// Get all active entities
	entities, err := storeDB.QueryEntities(store.EntityFilter{Status: "active"})
	if err != nil {
		return fmt.Errorf("failed to query entities: %w", err)
	}

	if len(entities) == 0 {
		return fmt.Errorf("no entities found - run `cx scan` first")
	}

	// Check if recompute needed
	needRecompute := rankRecompute
	if !needRecompute {
		for _, e := range entities {
			if m, _ := storeDB.GetMetrics(e.ID); m == nil {
				needRecompute = true
				break
			}
		}
	}

	if needRecompute {
		fmt.Fprintf(os.Stderr, "Computing metrics for %d entities...\n", len(entities))

		// Build graph from store
		g, err := graph.BuildFromStore(storeDB)
		if err != nil {
			return fmt.Errorf("failed to build graph: %w", err)
		}

		// Create adjacency map for existing metrics functions
		adjacency := g.Edges

		// Compute PageRank
		prConfig := metrics.PageRankConfig{
			Damping:       cfg.Metrics.PageRankDamping,
			MaxIterations: cfg.Metrics.PageRankIterations,
			Tolerance:     0.0001,
		}
		pagerank := metrics.ComputePageRank(adjacency, prConfig)

		// Compute betweenness
		betweenness := metrics.ComputeBetweenness(adjacency)

		// Compute degrees
		inDegree, outDegree := metrics.ComputeInOutDegree(adjacency)

		// Save to store
		bulkMetrics := make([]*store.Metrics, 0, len(entities))
		for _, e := range entities {
			m := &store.Metrics{
				EntityID:    e.ID,
				PageRank:    pagerank[e.ID],
				Betweenness: betweenness[e.ID],
				InDegree:    inDegree[e.ID],
				OutDegree:   outDegree[e.ID],
				ComputedAt:  time.Now(),
			}
			bulkMetrics = append(bulkMetrics, m)
		}

		if err := storeDB.SaveBulkMetrics(bulkMetrics); err != nil {
			return fmt.Errorf("failed to save metrics: %w", err)
		}

		fmt.Fprintf(os.Stderr, "Metrics computed and saved.\n")
	}

	// Build ranked list
	ranked := make([]rankedEntity, 0, len(entities))
	for _, e := range entities {
		m, err := storeDB.GetMetrics(e.ID)
		if err != nil || m == nil {
			continue
		}

		ranked = append(ranked, rankedEntity{
			entity:      e,
			pagerank:    m.PageRank,
			betweenness: m.Betweenness,
			inDegree:    m.InDegree,
			outDegree:   m.OutDegree,
		})
	}

	// Filter based on flags
	if rankKeystones {
		filtered := []rankedEntity{}
		for _, r := range ranked {
			if r.pagerank >= cfg.Metrics.KeystoneThreshold {
				filtered = append(filtered, r)
			}
		}
		ranked = filtered
	} else if rankBottlenecks {
		filtered := []rankedEntity{}
		for _, r := range ranked {
			if r.betweenness >= cfg.Metrics.BottleneckThreshold {
				filtered = append(filtered, r)
			}
		}
		ranked = filtered
	} else if rankLeaves {
		filtered := []rankedEntity{}
		for _, r := range ranked {
			// Leaf nodes have no dependents (in-degree = 0)
			if r.inDegree == 0 {
				filtered = append(filtered, r)
			}
		}
		ranked = filtered
	}

	// Sort by PageRank descending
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].pagerank > ranked[j].pagerank
	})

	// Limit to top N (only when not using filter flags)
	if len(ranked) > rankTop && !rankKeystones && !rankBottlenecks && !rankLeaves {
		ranked = ranked[:rankTop]
	}

	// Parse format and density
	format, err := output.ParseFormat(outputFormat)
	if err != nil {
		return fmt.Errorf("invalid format: %w", err)
	}

	density, err := output.ParseDensity(outputDensity)
	if err != nil {
		return fmt.Errorf("invalid density: %w", err)
	}

	// Build ListOutput with ranked entities
	listOutput := &output.ListOutput{
		Results: make(map[string]*output.EntityOutput),
		Count:   0,
	}

	for _, r := range ranked {
		name := r.entity.Name

		// Determine importance classification based on metrics
		importance := "normal"
		if r.pagerank >= cfg.Metrics.KeystoneThreshold {
			importance = "keystone"
		} else if r.betweenness >= cfg.Metrics.BottleneckThreshold {
			importance = "bottleneck"
		} else if r.inDegree == 0 {
			importance = "leaf"
		}

		// Build EntityOutput for ranked entity
		entityOut := &output.EntityOutput{
			Type:     mapStoreEntityTypeToString(r.entity.EntityType),
			Location: formatStoreLocation(r.entity),
		}

		// Add metrics (always included for rank command)
		entityOut.Metrics = &output.Metrics{
			PageRank:    r.pagerank,
			InDegree:    r.inDegree,
			OutDegree:   r.outDegree,
			Importance:  importance,
			Betweenness: r.betweenness,
		}

		// Add signature if available and density includes it
		if density.IncludesSignature() && r.entity.Signature != "" {
			entityOut.Signature = r.entity.Signature
		}

		// Add visibility
		if density.IncludesSignature() {
			entityOut.Visibility = inferVisibility(r.entity.Name)
		}

		listOutput.Results[name] = entityOut
		listOutput.Count++
	}

	// Get formatter and output
	formatter, err := output.GetFormatter(format)
	if err != nil {
		return fmt.Errorf("failed to get formatter: %w", err)
	}

	return formatter.FormatToWriter(cmd.OutOrStdout(), listOutput, density)
}
