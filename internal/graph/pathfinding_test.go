package graph

import (
	"reflect"
	"sort"
	"testing"
)

func TestGraph_AllPaths(t *testing.T) {
	g := newTestGraph()
	// Create diamond graph: a -> b -> d
	//                       a -> c -> d
	g.addEdge("a", "b")
	g.addEdge("a", "c")
	g.addEdge("b", "d")
	g.addEdge("c", "d")

	paths := g.AllPaths("a", "d", 10)

	if len(paths) != 2 {
		t.Errorf("expected 2 paths, got %d: %v", len(paths), paths)
	}

	// Both paths should start with a and end with d
	for _, path := range paths {
		if len(path) != 3 {
			t.Errorf("expected path length 3, got %d: %v", len(path), path)
		}
		if path[0] != "a" || path[len(path)-1] != "d" {
			t.Errorf("path should start with 'a' and end with 'd': %v", path)
		}
	}
}

func TestGraph_AllPaths_SameNode(t *testing.T) {
	g := newTestGraph()
	g.addEdge("a", "b")

	paths := g.AllPaths("a", "a", 10)

	if len(paths) != 1 || len(paths[0]) != 1 || paths[0][0] != "a" {
		t.Errorf("expected [[a]] for same node, got %v", paths)
	}
}

func TestGraph_AllPaths_NoPath(t *testing.T) {
	g := newTestGraph()
	g.addEdge("a", "b")
	g.Edges["c"] = []string{}
	g.ReverseEdges["c"] = []string{}

	paths := g.AllPaths("a", "c", 10)

	if len(paths) != 0 {
		t.Errorf("expected no paths, got %v", paths)
	}
}

func TestGraph_AllPaths_LongChain(t *testing.T) {
	g := newTestGraph()
	// a -> b -> c -> d -> e
	g.addEdge("a", "b")
	g.addEdge("b", "c")
	g.addEdge("c", "d")
	g.addEdge("d", "e")

	// With maxDepth 3, should not reach e
	paths := g.AllPaths("a", "e", 3)
	if len(paths) != 0 {
		t.Errorf("expected no paths with maxDepth 3, got %v", paths)
	}

	// With maxDepth 5, should find path
	paths = g.AllPaths("a", "e", 5)
	if len(paths) != 1 {
		t.Errorf("expected 1 path with maxDepth 5, got %d", len(paths))
	}
}

func TestGraph_AllPaths_MultiplePaths(t *testing.T) {
	g := newTestGraph()
	// Create graph with multiple paths to same destination
	// a -> b -> e
	// a -> c -> e
	// a -> d -> e
	g.addEdge("a", "b")
	g.addEdge("a", "c")
	g.addEdge("a", "d")
	g.addEdge("b", "e")
	g.addEdge("c", "e")
	g.addEdge("d", "e")

	paths := g.AllPaths("a", "e", 10)

	if len(paths) != 3 {
		t.Errorf("expected 3 paths, got %d: %v", len(paths), paths)
	}
}

func TestGraph_AllPathsReverse(t *testing.T) {
	g := newTestGraph()
	// a -> b -> c
	g.addEdge("a", "b")
	g.addEdge("b", "c")

	// From c, trace back to a
	paths := g.AllPathsReverse("c", "a", 10)

	if len(paths) != 1 {
		t.Errorf("expected 1 path, got %d: %v", len(paths), paths)
	}

	if len(paths) > 0 {
		path := paths[0]
		if path[0] != "c" || path[len(path)-1] != "a" {
			t.Errorf("reverse path should go from c to a: %v", path)
		}
	}
}

func TestParsePattern(t *testing.T) {
	tests := []struct {
		pattern  string
		expected []PatternElement
	}{
		{
			"A -> B",
			[]PatternElement{
				{Type: PatternExact, Value: "A"},
				{Type: PatternExact, Value: "B"},
			},
		},
		{
			"A -> * -> B",
			[]PatternElement{
				{Type: PatternExact, Value: "A"},
				{Type: PatternWildcard},
				{Type: PatternExact, Value: "B"},
			},
		},
		{
			"A -> ** -> B",
			[]PatternElement{
				{Type: PatternExact, Value: "A"},
				{Type: PatternMultiWildcard},
				{Type: PatternExact, Value: "B"},
			},
		},
		{
			"Auth* -> *",
			[]PatternElement{
				{Type: PatternExact, Value: "Auth*"},
				{Type: PatternWildcard},
			},
		},
	}

	for _, tt := range tests {
		parsed := ParsePattern(tt.pattern)
		if len(parsed.Elements) != len(tt.expected) {
			t.Errorf("ParsePattern(%q): expected %d elements, got %d", tt.pattern, len(tt.expected), len(parsed.Elements))
			continue
		}
		for i, elem := range parsed.Elements {
			if elem.Type != tt.expected[i].Type || elem.Value != tt.expected[i].Value {
				t.Errorf("ParsePattern(%q)[%d]: expected %+v, got %+v", tt.pattern, i, tt.expected[i], elem)
			}
		}
	}
}

func TestGraph_PathMatch_Direct(t *testing.T) {
	g := newTestGraph()
	g.addEdge("a", "b")
	g.addEdge("b", "c")

	// Direct path a -> b
	paths := g.PathMatch("a -> b", 10, nil)

	if len(paths) != 1 {
		t.Errorf("expected 1 path, got %d: %v", len(paths), paths)
	}
	if len(paths) > 0 && !reflect.DeepEqual(paths[0], Path{"a", "b"}) {
		t.Errorf("expected [a, b], got %v", paths[0])
	}
}

func TestGraph_PathMatch_Wildcard(t *testing.T) {
	g := newTestGraph()
	g.addEdge("a", "b")
	g.addEdge("b", "c")

	// Pattern: a -> * -> c (a to c through any one node)
	paths := g.PathMatch("a -> * -> c", 10, nil)

	if len(paths) != 1 {
		t.Errorf("expected 1 path, got %d: %v", len(paths), paths)
	}
	if len(paths) > 0 && !reflect.DeepEqual(paths[0], Path{"a", "b", "c"}) {
		t.Errorf("expected [a, b, c], got %v", paths[0])
	}
}

func TestGraph_PathMatch_MultiWildcard(t *testing.T) {
	g := newTestGraph()
	g.addEdge("a", "b")
	g.addEdge("b", "c")
	g.addEdge("c", "d")

	// Pattern: a -> ** -> d (a to d through any number of nodes)
	paths := g.PathMatch("a -> ** -> d", 10, nil)

	// Should find at least one path
	if len(paths) == 0 {
		t.Error("expected at least 1 path for multi-wildcard pattern")
	}

	// Verify at least one path goes a -> ... -> d
	found := false
	for _, path := range paths {
		if len(path) >= 2 && path[0] == "a" && path[len(path)-1] == "d" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected path from a to d, got: %v", paths)
	}
}

func TestGraph_PathMatch_NoMatch(t *testing.T) {
	g := newTestGraph()
	g.addEdge("a", "b")
	g.addEdge("b", "c")

	// Pattern: x -> y (no such nodes)
	paths := g.PathMatch("x -> y", 10, nil)

	if len(paths) != 0 {
		t.Errorf("expected no paths for non-existent pattern, got %v", paths)
	}
}

func TestGraph_PathMatch_PrefixWildcard(t *testing.T) {
	g := newTestGraph()
	g.addEdge("AuthService", "UserRepo")
	g.addEdge("AuthHandler", "AuthService")

	// Pattern: Auth* -> * (anything starting with Auth to anything)
	paths := g.PathMatch("Auth* -> *", 10, nil)

	if len(paths) < 2 {
		t.Errorf("expected at least 2 paths (AuthService->UserRepo, AuthHandler->AuthService), got %d: %v", len(paths), paths)
	}
}

func TestGraph_TraceCallers(t *testing.T) {
	g := newTestGraph()
	// caller1 -> target
	// caller2 -> target
	g.addEdge("caller1", "target")
	g.addEdge("caller2", "target")

	paths := g.TraceCallers("target", 1)

	if len(paths) != 2 {
		t.Errorf("expected 2 caller paths, got %d: %v", len(paths), paths)
	}

	// Each path should end with target
	for _, path := range paths {
		if path[len(path)-1] != "target" {
			t.Errorf("caller path should end with target: %v", path)
		}
	}
}

func TestGraph_TraceCallees(t *testing.T) {
	g := newTestGraph()
	// source -> callee1
	// source -> callee2
	g.addEdge("source", "callee1")
	g.addEdge("source", "callee2")

	paths := g.TraceCallees("source", 1)

	if len(paths) != 2 {
		t.Errorf("expected 2 callee paths, got %d: %v", len(paths), paths)
	}

	// Each path should start with source
	for _, path := range paths {
		if path[0] != "source" {
			t.Errorf("callee path should start with source: %v", path)
		}
	}
}

func TestGraph_TraceCallers_MultiLevel(t *testing.T) {
	g := newTestGraph()
	// grandparent -> parent -> target
	g.addEdge("grandparent", "parent")
	g.addEdge("parent", "target")

	paths := g.TraceCallers("target", 2)

	// Should have path from parent and from grandparent
	if len(paths) < 2 {
		t.Errorf("expected at least 2 paths (parent->target, grandparent->parent->target), got %d: %v", len(paths), paths)
	}
}

func TestGraph_CollectCallerChain(t *testing.T) {
	g := newTestGraph()
	g.addEdge("a", "b")
	g.addEdge("b", "c")
	g.addEdge("c", "d")

	chain := g.CollectCallerChain("d", 10)

	// Should include d, c, b, a in some BFS order starting from d
	if len(chain) != 4 {
		t.Errorf("expected 4 nodes in chain, got %d: %v", len(chain), chain)
	}

	if chain[0] != "d" {
		t.Errorf("chain should start with 'd', got %s", chain[0])
	}
}

func TestGraph_CollectCalleeChain(t *testing.T) {
	g := newTestGraph()
	g.addEdge("a", "b")
	g.addEdge("b", "c")
	g.addEdge("c", "d")

	chain := g.CollectCalleeChain("a", 10)

	// Should include a, b, c, d in BFS order
	if len(chain) != 4 {
		t.Errorf("expected 4 nodes in chain, got %d: %v", len(chain), chain)
	}

	if chain[0] != "a" {
		t.Errorf("chain should start with 'a', got %s", chain[0])
	}
}

func TestGraph_CollectCallerChain_MaxDepth(t *testing.T) {
	g := newTestGraph()
	// Create long chain: a -> b -> c -> d -> e -> f
	g.addEdge("a", "b")
	g.addEdge("b", "c")
	g.addEdge("c", "d")
	g.addEdge("d", "e")
	g.addEdge("e", "f")

	chain := g.CollectCallerChain("f", 3)

	// Should be limited to 4 nodes (f + 3 levels)
	if len(chain) > 4 {
		t.Errorf("expected at most 4 nodes with maxDepth 3, got %d: %v", len(chain), chain)
	}
}

func TestGraph_AllPaths_AvoidsCycles(t *testing.T) {
	g := newTestGraph()
	// Create a graph with a cycle: a -> b -> c -> a
	g.addEdge("a", "b")
	g.addEdge("b", "c")
	g.addEdge("c", "a")
	g.addEdge("b", "d") // d is the target

	paths := g.AllPaths("a", "d", 10)

	// Should find path a -> b -> d without infinite loop
	if len(paths) != 1 {
		t.Errorf("expected 1 path, got %d: %v", len(paths), paths)
	}

	// Verify path doesn't contain duplicates
	if len(paths) > 0 {
		seen := make(map[string]bool)
		for _, node := range paths[0] {
			if seen[node] {
				t.Errorf("path contains duplicate node %s: %v", node, paths[0])
				break
			}
			seen[node] = true
		}
	}
}

func TestGraph_TracePath(t *testing.T) {
	g := newTestGraph()
	g.addEdge("a", "b")
	g.addEdge("b", "c")

	path := g.TracePath("a", "c", "forward")

	if len(path) != 3 {
		t.Errorf("expected path length 3, got %d: %v", len(path), path)
	}

	if path[0] != "a" || path[len(path)-1] != "c" {
		t.Errorf("path should go from a to c: %v", path)
	}
}

// Benchmark tests

func BenchmarkAllPaths_Diamond(b *testing.B) {
	g := newTestGraph()
	g.addEdge("a", "b")
	g.addEdge("a", "c")
	g.addEdge("b", "d")
	g.addEdge("c", "d")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.AllPaths("a", "d", 10)
	}
}

func BenchmarkAllPaths_Wide(b *testing.B) {
	g := newTestGraph()
	// Create wide graph: start -> n intermediate nodes -> end
	for i := 0; i < 100; i++ {
		mid := string(rune('a' + i%26))
		g.addEdge("start", mid)
		g.addEdge(mid, "end")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.AllPaths("start", "end", 5)
	}
}

func BenchmarkPathMatch_Pattern(b *testing.B) {
	g := newTestGraph()
	for i := 0; i < 100; i++ {
		from := string(rune('a' + i%26))
		to := string(rune('a' + (i+1)%26))
		g.addEdge(from, to)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.PathMatch("a -> * -> c", 10, nil)
	}
}

// Helper to sort paths for comparison
func sortPaths(paths []Path) {
	sort.Slice(paths, func(i, j int) bool {
		pi, pj := paths[i], paths[j]
		minLen := len(pi)
		if len(pj) < minLen {
			minLen = len(pj)
		}
		for k := 0; k < minLen; k++ {
			if pi[k] != pj[k] {
				return pi[k] < pj[k]
			}
		}
		return len(pi) < len(pj)
	})
}
