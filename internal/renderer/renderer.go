package renderer

import (
	"fmt"
	"strings"

	"github.com/Akaiko1/file-tree-scanner/internal/scanner"
)

const (
	// Icons
	folderIcon = "📁"
	fileIcon   = "📄"

	// Tree drawing characters
	treeVertical   = "│"
	treeBranch     = "├──"
	treeLastBranch = "└──"
	treeSpacing    = "    "
	treeConnection = "│   "
)

// TreeRenderer defines the interface for rendering tree structures.
type TreeRenderer interface {
	RenderTree(root *scanner.TreeNode) string
}

// StandardTreeRenderer implements TreeRenderer for standard tree visualization.
type StandardTreeRenderer struct{}

// RenderTree renders a tree structure as a formatted string.
func (r *StandardTreeRenderer) RenderTree(root *scanner.TreeNode) string {
	if root == nil {
		return ""
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("File Tree for: %s\n", root.Path))
	builder.WriteString(strings.Repeat("=", 50) + "\n\n")

	r.renderNode(&builder, root, "", true)

	return builder.String()
}

// renderNode recursively renders a tree node.
func (r *StandardTreeRenderer) renderNode(builder *strings.Builder, node *scanner.TreeNode, prefix string, isRoot bool) {
	if !isRoot {
		icon := fileIcon
		name := node.Name
		if node.IsDir {
			icon = folderIcon
			name += "/"
		}
		builder.WriteString(fmt.Sprintf("%s %s\n", icon, name))
	}

	for i, child := range node.Children {
		isLast := i == len(node.Children)-1

		var connector, nextPrefix string
		if isRoot && i == 0 {
			connector = ""
			nextPrefix = ""
		} else if isLast {
			connector = treeLastBranch + " "
			nextPrefix = prefix + treeSpacing
		} else {
			connector = treeBranch + " "
			nextPrefix = prefix + treeConnection
		}

		builder.WriteString(prefix + connector)
		r.renderNode(builder, child, nextPrefix, false)
	}
}