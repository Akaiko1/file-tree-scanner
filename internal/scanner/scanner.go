package scanner

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Akaiko1/file-tree-scanner/internal/config"
)

// TreeNode represents a node in the file tree structure.
type TreeNode struct {
	Path     string
	Name     string
	IsDir    bool
	Children []*TreeNode
	Parent   *TreeNode
}

// ScanResult contains the results of a directory scan operation.
type ScanResult struct {
	RootPath  string
	TreeText  string
	NodeCount int
	Error     error
	Root      *TreeNode // Root node of the scanned tree for UI rendering
}

// FileSystemScanner defines the interface for scanning file systems.
type FileSystemScanner interface {
	ScanDirectory(ctx context.Context, path string) (*ScanResult, error)
}

// FileTreeScanner implements FileSystemScanner for scanning directory structures.
type FileTreeScanner struct {
	config *config.Config
}

// NewFileTreeScanner creates a new FileTreeScanner with the given configuration.
func NewFileTreeScanner(cfg *config.Config) *FileTreeScanner {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	return &FileTreeScanner{
		config: cfg,
	}
}

// ScanDirectory recursively scans a directory structure and returns detailed results including node count and tree representation.
func (s *FileTreeScanner) ScanDirectory(ctx context.Context, path string) (*ScanResult, error) {
	if path == "" {
		return nil, fmt.Errorf("path cannot be empty")
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat path %q: %w", path, err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("path %q is not a directory", path)
	}

	root := &TreeNode{
		Path:  path,
		Name:  filepath.Base(path),
		IsDir: true,
	}

	nodeCount, err := s.scanNode(ctx, root, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to scan directory: %w", err)
	}

	return &ScanResult{
		RootPath:  path,
		NodeCount: nodeCount,
		Error:     nil,
		Root:      root,
	}, nil
}

// scanNode recursively scans a directory node, respecting depth limits and cancellation context.
func (s *FileTreeScanner) scanNode(ctx context.Context, node *TreeNode, depth int) (int, error) {
	// Check for cancellation more frequently
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}

	// Enforce depth limits to prevent infinite recursion
	if s.config.MaxDepth >= 0 && depth > s.config.MaxDepth {
		return 0, nil
	}

	// Add safety limit even when MaxDepth is unlimited
	if depth > 50 {
		log.Printf("Warning: stopping scan at depth %d for path %s", depth, node.Path)
		return 1, nil
	}

	entries, err := os.ReadDir(node.Path)
	if err != nil {
		log.Printf("Warning: failed to read directory %q: %v", node.Path, err)
		return 1, nil // Continue with partial results
	}

	// Limit number of entries to prevent memory issues
	if len(entries) > 10000 {
		log.Printf("Warning: directory %s has %d entries, limiting to first 1000", node.Path, len(entries))
		entries = entries[:1000]
	}

	// Filter hidden files if configured
	if !s.config.ShowHidden {
		entries = s.filterHiddenEntries(entries)
	}

	// Sort entries if configured
	if s.config.SortDirs {
		s.sortEntries(entries)
	}

	nodeCount := 1 // Count current node

	for i, entry := range entries {
		// Check for cancellation in the loop
		select {
		case <-ctx.Done():
			return nodeCount, ctx.Err()
		default:
		}

		// Limit processing time per directory
		if i > 0 && i%100 == 0 {
			// Brief pause every 100 entries to allow cancellation
			time.Sleep(1 * time.Millisecond)
		}

		childPath := filepath.Join(node.Path, entry.Name())

		// Skip problematic paths
		if s.isProblematicPath(childPath) {
			continue
		}

		child := &TreeNode{
			Path:   childPath,
			Name:   entry.Name(),
			IsDir:  entry.IsDir(),
			Parent: node,
		}

		node.Children = append(node.Children, child)

		if child.IsDir {
			childCount, err := s.scanNode(ctx, child, depth+1)
			if err != nil {
				if err == context.Canceled || err == context.DeadlineExceeded {
					return nodeCount, err
				}
				// Log error but continue
				log.Printf("Error scanning subdirectory %s: %v", childPath, err)
			}
			nodeCount += childCount
		} else {
			nodeCount++
		}
	}

	return nodeCount, nil
}

// isProblematicPath checks if a path might cause issues and should be skipped.
func (s *FileTreeScanner) isProblematicPath(path string) bool {
	// Skip Windows system paths that often cause permission issues
	problematicPaths := []string{
		"System Volume Information",
		"$Recycle.Bin",
		"$WINDOWS.~BT",
		"Recovery",
		"ProgramData\\Microsoft\\Windows Defender",
		"Windows\\System32\\config",
	}

	for _, problematic := range problematicPaths {
		if strings.Contains(path, problematic) {
			return true
		}
	}

	return false
}

// filterHiddenEntries filters out hidden files and directories.
func (s *FileTreeScanner) filterHiddenEntries(entries []os.DirEntry) []os.DirEntry {
	filtered := make([]os.DirEntry, 0, len(entries))
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), ".") {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

// sortEntries sorts directory entries with directories first, then files.
func (s *FileTreeScanner) sortEntries(entries []os.DirEntry) {
	sort.Slice(entries, func(i, j int) bool {
		// Directories come first
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		// Then sort alphabetically
		return entries[i].Name() < entries[j].Name()
	})
}