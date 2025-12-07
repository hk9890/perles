// Package tree provides tree visualization components for issue dependency graphs.
package tree

import (
	"fmt"

	"perles/internal/beads"
)

// Direction controls tree traversal direction.
type Direction string

const (
	// DirectionDown traverses Children + Blocks (what depends on this issue).
	DirectionDown Direction = "down"
	// DirectionUp traverses ParentID + BlockedBy (what this issue depends on).
	DirectionUp Direction = "up"
)

// TreeMode controls which relationships the tree shows.
type TreeMode string

const (
	// ModeDeps shows dependency relationships (blocks/blocked-by + parent/children).
	ModeDeps TreeMode = "deps"
	// ModeChildren shows only parent-child hierarchy (no dependencies).
	ModeChildren TreeMode = "children"
)

// String returns the mode as a display string.
func (m TreeMode) String() string {
	return string(m)
}

// String returns the direction as a string.
func (d Direction) String() string {
	return string(d)
}

// TreeNode represents a node in the dependency tree.
type TreeNode struct {
	Issue    beads.Issue // Issue data
	Children []*TreeNode // Child nodes in tree
	Depth    int         // Nesting level (0 = root)
	Parent   *TreeNode   // Parent node in tree (nil for root)
}

// BuildTree constructs a TreeNode hierarchy from an issue map.
// The issueMap contains issues returned by BQL expand query.
// Direction determines which relationships to traverse:
//   - DirectionDown: children + blocked issues (what depends on this)
//   - DirectionUp: parent + blocking issues (what this depends on)
//
// Mode determines which relationship types to include:
//   - ModeDeps: all relationships (parent/child + blocks/blocked-by)
//   - ModeChildren: only parent-child hierarchy
func BuildTree(issueMap map[string]*beads.Issue, rootID string, dir Direction, mode TreeMode) (*TreeNode, error) {
	rootIssue, ok := issueMap[rootID]
	if !ok {
		return nil, fmt.Errorf("root issue %s not found", rootID)
	}

	seen := make(map[string]bool) // Prevent cycles
	return buildNode(rootIssue, issueMap, dir, mode, 0, seen, nil), nil
}

func buildNode(
	issue *beads.Issue,
	issueMap map[string]*beads.Issue,
	dir Direction,
	mode TreeMode,
	depth int,
	seen map[string]bool,
	parent *TreeNode,
) *TreeNode {
	if seen[issue.ID] {
		return nil // Cycle detected
	}
	seen[issue.ID] = true

	node := &TreeNode{
		Issue:  *issue,
		Depth:  depth,
		Parent: parent,
	}

	// Get related IDs based on direction and mode
	var relatedIDs []string
	if dir == DirectionDown {
		// Down direction
		relatedIDs = append(relatedIDs, issue.Children...)
		if mode == ModeDeps {
			// Include dependency relationships
			relatedIDs = append(relatedIDs, issue.Blocks...)
		}
	} else {
		// Up direction
		if issue.ParentID != "" {
			relatedIDs = append(relatedIDs, issue.ParentID)
		}
		if mode == ModeDeps {
			// Include dependency relationships
			relatedIDs = append(relatedIDs, issue.BlockedBy...)
		}
	}

	// Build child nodes (only for issues that exist in the map)
	for _, relatedID := range relatedIDs {
		if relatedIssue, ok := issueMap[relatedID]; ok {
			if child := buildNode(relatedIssue, issueMap, dir, mode, depth+1, seen, node); child != nil {
				node.Children = append(node.Children, child)
			}
		}
	}

	return node
}

// Flatten returns a slice of all nodes in tree order.
func (n *TreeNode) Flatten() []*TreeNode {
	var result []*TreeNode
	result = append(result, n)

	for _, child := range n.Children {
		result = append(result, child.Flatten()...)
	}

	return result
}

// CalculateProgress returns the count of closed issues and total issues
// in this subtree (including this node).
func (n *TreeNode) CalculateProgress() (closed, total int) {
	if n.Issue.Status == beads.StatusClosed {
		closed++
	}
	total++

	for _, child := range n.Children {
		c, t := child.CalculateProgress()
		closed += c
		total += t
	}

	return closed, total
}
