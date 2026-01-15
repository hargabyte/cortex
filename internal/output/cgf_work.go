// Package output provides work item (bead) formatting for CGF output.
// DEPRECATED: CGF format is deprecated and will be removed in v2.0.
package output

import (
	"fmt"
	"strings"
)

// CGFWorkItem represents a bead (work item) for CGF output (DEPRECATED)
// DEPRECATED: CGF format is deprecated.
type CGFWorkItem struct {
	// ID is the bead identifier (e.g., "bd-a7c")
	ID string

	// Priority is the priority level 0-4
	// 0 = critical, 1 = high, 2 = medium, 3 = low, 4 = backlog
	Priority int

	// Type is the work item type: bug, feature, task, epic, chore
	Type string

	// Title is the work item title (will be quoted in output)
	Title string

	// Status is the current status: open, in_progress, blocked, closed
	Status string

	// Assignee is the assigned agent or human
	Assignee string

	// DueDate is the optional due date (YYYY-MM-DD or +Nd format)
	DueDate string
}

// WriteWorkItem writes a work item in CGF format.
//
// Basic format:
//
//	W bd-a7c P1 bug "Login fails with expired refresh"
//
// With properties:
//
//	W bd-a7c P1 bug "Login fails with expired refresh"
//	   s=in_progress a=claude
//
// Full example with edges:
//
//	W bd-a7c P1 bug "Login fails with expired refresh"
//	   s=in_progress a=claude d=2026-01-20
//	   ~F src/auth/login.go:89
//	   !bd-b2f
func (w *CGFWriter) WriteWorkItem(item *CGFWorkItem) error {
	// Escape any quotes in the title
	title := strings.ReplaceAll(item.Title, `"`, `\"`)

	// Write the main work item line
	// Format: W <id> P<priority> <type> "<title>"
	if err := w.writeLineRaw("W %s P%d %s \"%s\"", item.ID, item.Priority, item.Type, title); err != nil {
		return err
	}

	// Write properties if any exist
	if item.Status != "" || item.Assignee != "" || item.DueDate != "" {
		w.Indent()
		if err := w.writeWorkItemProperties(item); err != nil {
			w.Dedent()
			return err
		}
		w.Dedent()
	}

	return nil
}

// writeWorkItemProperties writes the property line for a work item
func (w *CGFWriter) writeWorkItemProperties(item *CGFWorkItem) error {
	var parts []string

	if item.Status != "" {
		parts = append(parts, fmt.Sprintf("s=%s", item.Status))
	}
	if item.Assignee != "" {
		parts = append(parts, fmt.Sprintf("a=%s", item.Assignee))
	}
	if item.DueDate != "" {
		parts = append(parts, fmt.Sprintf("d=%s", item.DueDate))
	}

	if len(parts) > 0 {
		return w.writeLineRaw("%s", strings.Join(parts, " "))
	}
	return nil
}

// WriteWorkItemWithEdges writes a work item along with its relationship edges.
// This is a convenience method that combines WriteWorkItem with edge writing.
func (w *CGFWriter) WriteWorkItemWithEdges(item *CGFWorkItem, edges []*CGFEdge) error {
	// Write the work item first
	if err := w.WriteWorkItem(item); err != nil {
		return err
	}

	// Write edges indented under the work item
	if len(edges) > 0 {
		w.Indent()
		for _, edge := range edges {
			if err := w.WriteEdge(edge); err != nil {
				w.Dedent()
				return err
			}
		}
		w.Dedent()
	}

	return nil
}

// CGFWorkItemPriorityString converts a priority int to the CGF priority string
// DEPRECATED: CGF format is deprecated.
func CGFWorkItemPriorityString(priority int) string {
	if priority < 0 {
		priority = 0
	}
	if priority > 4 {
		priority = 4
	}
	return fmt.Sprintf("P%d", priority)
}

// ParseCGFWorkItemPriority parses a priority string (P0-P4 or 0-4) to an int
// DEPRECATED: CGF format is deprecated.
func ParseCGFWorkItemPriority(s string) (int, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "P")
	s = strings.TrimPrefix(s, "p")

	var priority int
	if _, err := fmt.Sscanf(s, "%d", &priority); err != nil {
		return 0, fmt.Errorf("invalid priority: %q", s)
	}

	if priority < 0 || priority > 4 {
		return 0, fmt.Errorf("priority out of range: %d (expected 0-4)", priority)
	}

	return priority, nil
}

// ValidCGFWorkItemTypes returns the list of valid work item types
// DEPRECATED: CGF format is deprecated.
func ValidCGFWorkItemTypes() []string {
	return []string{"bug", "feature", "task", "epic", "chore"}
}

// ValidCGFWorkItemStatuses returns the list of valid work item statuses
// DEPRECATED: CGF format is deprecated.
func ValidCGFWorkItemStatuses() []string {
	return []string{"open", "in_progress", "blocked", "closed"}
}

// ValidateCGFWorkItemType checks if a type string is valid
// DEPRECATED: CGF format is deprecated.
func ValidateCGFWorkItemType(t string) bool {
	t = strings.ToLower(strings.TrimSpace(t))
	for _, valid := range ValidCGFWorkItemTypes() {
		if t == valid {
			return true
		}
	}
	return false
}

// ValidateCGFWorkItemStatus checks if a status string is valid
// DEPRECATED: CGF format is deprecated.
func ValidateCGFWorkItemStatus(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	for _, valid := range ValidCGFWorkItemStatuses() {
		if s == valid {
			return true
		}
	}
	return false
}
