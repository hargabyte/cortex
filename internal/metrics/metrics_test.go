package metrics

import (
	"testing"
	"time"
)

func TestClassifyImportance(t *testing.T) {
	tests := []struct {
		pr       float64
		expected Importance
	}{
		{0.50, Critical},
		{0.75, Critical},
		{1.00, Critical},
		{0.30, High},
		{0.49, High},
		{0.10, Medium},
		{0.29, Medium},
		{0.09, Low},
		{0.01, Low},
		{0.00, Low},
	}

	for _, tt := range tests {
		result := ClassifyImportance(tt.pr)
		if result != tt.expected {
			t.Errorf("ClassifyImportance(%f) = %s, expected %s", tt.pr, result, tt.expected)
		}
	}
}

func TestIsKeystone(t *testing.T) {
	tests := []struct {
		pr       float64
		deps     int
		expected bool
	}{
		// Keystones (pr >= 0.30 AND deps >= 5)
		{0.30, 5, true},
		{0.50, 10, true},
		{0.35, 6, true},

		// Not keystones
		{0.29, 5, false},  // PR too low
		{0.30, 4, false},  // Not enough deps
		{0.50, 4, false},  // High PR but not enough deps
		{0.20, 10, false}, // Many deps but low PR
		{0.00, 0, false},
	}

	for _, tt := range tests {
		result := IsKeystone(tt.pr, tt.deps)
		if result != tt.expected {
			t.Errorf("IsKeystone(%f, %d) = %v, expected %v", tt.pr, tt.deps, result, tt.expected)
		}
	}
}

func TestIsBottleneck(t *testing.T) {
	tests := []struct {
		betweenness float64
		expected    bool
	}{
		{0.20, true},
		{0.50, true},
		{1.00, true},
		{0.19, false},
		{0.10, false},
		{0.00, false},
	}

	for _, tt := range tests {
		result := IsBottleneck(tt.betweenness)
		if result != tt.expected {
			t.Errorf("IsBottleneck(%f) = %v, expected %v", tt.betweenness, result, tt.expected)
		}
	}
}

func TestDefaultThresholds(t *testing.T) {
	thresholds := DefaultThresholds()

	if thresholds.Critical != 0.50 {
		t.Errorf("expected Critical threshold 0.50, got %f", thresholds.Critical)
	}
	if thresholds.High != 0.30 {
		t.Errorf("expected High threshold 0.30, got %f", thresholds.High)
	}
	if thresholds.Medium != 0.10 {
		t.Errorf("expected Medium threshold 0.10, got %f", thresholds.Medium)
	}
	if thresholds.KeystonePR != 0.30 {
		t.Errorf("expected KeystonePR threshold 0.30, got %f", thresholds.KeystonePR)
	}
	if thresholds.KeystoneDep != 5 {
		t.Errorf("expected KeystoneDep threshold 5, got %d", thresholds.KeystoneDep)
	}
	if thresholds.Bottleneck != 0.20 {
		t.Errorf("expected Bottleneck threshold 0.20, got %f", thresholds.Bottleneck)
	}
}

func TestClassifyWithThresholds(t *testing.T) {
	// Custom thresholds
	customThresholds := ImportanceThresholds{
		Critical: 0.80,
		High:     0.50,
		Medium:   0.20,
	}

	tests := []struct {
		pr       float64
		expected Importance
	}{
		{0.80, Critical},
		{0.90, Critical},
		{0.50, High},
		{0.79, High},
		{0.20, Medium},
		{0.49, Medium},
		{0.19, Low},
		{0.00, Low},
	}

	for _, tt := range tests {
		result := ClassifyWithThresholds(tt.pr, customThresholds)
		if result != tt.expected {
			t.Errorf("ClassifyWithThresholds(%f) = %s, expected %s with custom thresholds",
				tt.pr, result, tt.expected)
		}
	}
}

func TestIsKeystoneWithThresholds(t *testing.T) {
	customThresholds := ImportanceThresholds{
		KeystonePR:  0.50,
		KeystoneDep: 10,
	}

	tests := []struct {
		pr       float64
		deps     int
		expected bool
	}{
		{0.50, 10, true},
		{0.60, 15, true},
		{0.49, 10, false},
		{0.50, 9, false},
		{0.30, 5, false}, // Would pass default thresholds but not custom
	}

	for _, tt := range tests {
		result := IsKeystoneWithThresholds(tt.pr, tt.deps, customThresholds)
		if result != tt.expected {
			t.Errorf("IsKeystoneWithThresholds(%f, %d) = %v, expected %v with custom thresholds",
				tt.pr, tt.deps, result, tt.expected)
		}
	}
}

func TestIsBottleneckWithThreshold(t *testing.T) {
	tests := []struct {
		betweenness float64
		threshold   float64
		expected    bool
	}{
		{0.30, 0.25, true},
		{0.25, 0.25, true},
		{0.24, 0.25, false},
		{0.50, 0.50, true},
		{0.49, 0.50, false},
	}

	for _, tt := range tests {
		result := IsBottleneckWithThreshold(tt.betweenness, tt.threshold)
		if result != tt.expected {
			t.Errorf("IsBottleneckWithThreshold(%f, %f) = %v, expected %v",
				tt.betweenness, tt.threshold, result, tt.expected)
		}
	}
}

func TestMetricsStruct(t *testing.T) {
	now := time.Now()
	m := Metrics{
		EntityID:    "test-entity",
		PageRank:    0.35,
		InDegree:    5,
		OutDegree:   3,
		Betweenness: 0.25,
		ComputedAt:  now,
	}

	if m.EntityID != "test-entity" {
		t.Errorf("expected EntityID 'test-entity', got %s", m.EntityID)
	}
	if m.PageRank != 0.35 {
		t.Errorf("expected PageRank 0.35, got %f", m.PageRank)
	}
	if m.InDegree != 5 {
		t.Errorf("expected InDegree 5, got %d", m.InDegree)
	}
	if m.OutDegree != 3 {
		t.Errorf("expected OutDegree 3, got %d", m.OutDegree)
	}
	if m.Betweenness != 0.25 {
		t.Errorf("expected Betweenness 0.25, got %f", m.Betweenness)
	}
	if !m.ComputedAt.Equal(now) {
		t.Errorf("expected ComputedAt %v, got %v", now, m.ComputedAt)
	}
}

func TestImportanceConstants(t *testing.T) {
	// Verify string values
	if Critical != "critical" {
		t.Errorf("expected Critical = 'critical', got %s", Critical)
	}
	if High != "high" {
		t.Errorf("expected High = 'high', got %s", High)
	}
	if Medium != "medium" {
		t.Errorf("expected Medium = 'medium', got %s", Medium)
	}
	if Low != "low" {
		t.Errorf("expected Low = 'low', got %s", Low)
	}
}
