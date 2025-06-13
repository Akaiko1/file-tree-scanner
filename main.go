// Package main implements a cross-platform GUI application for scanning and visualizing
// directory structures using the Fyne framework.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const (
	// UI Constants
	appTitle     = "File Tree Scanner: AI Agent helper"
	windowWidth  = 800
	windowHeight = 600

	// Icons
	folderIcon = "📁"
	fileIcon   = "📄"

	// Tree drawing characters
	treeVertical   = "│"
	treeBranch     = "├──"
	treeLastBranch = "└──"
	treeSpacing    = "    "
	treeConnection = "│   "

	// File operations
	defaultFileExt = ".txt"
	timeFormat     = "2006-01-02_15-04-05"

	// Messages
	msgNoData      = "Please scan a directory first."
	msgScanSuccess = "Directory scanned successfully!"
	msgSaveSuccess = "File tree saved successfully!"
	msgCopySuccess = "File tree copied to clipboard!"
	msgScanning    = "Scanning directory..."
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
	Root      *TreeNode // Add root TreeNode for UI
}

// FileSystemScanner defines the interface for scanning file systems.
type FileSystemScanner interface {
	ScanDirectory(ctx context.Context, path string) (*ScanResult, error)
}

// TreeRenderer defines the interface for rendering tree structures.
type TreeRenderer interface {
	RenderTree(root *TreeNode) string
}

// ClipboardManager defines the interface for clipboard operations.
type ClipboardManager interface {
	SetContent(content string) error
}

// Config holds application configuration.
type Config struct {
	MaxDepth      int
	ShowHidden    bool
	SortDirs      bool
	ShowSize      bool
	ConcurrentOps int
}

// DefaultConfig returns a default configuration.
func DefaultConfig() *Config {
	return &Config{
		MaxDepth:      15, // Reasonable depth limit to prevent hangs
		ShowHidden:    false,
		SortDirs:      true,
		ShowSize:      false,
		ConcurrentOps: 5, // Reduced for stability
	}
}

// FileTreeScanner implements FileSystemScanner for scanning directory structures.
type FileTreeScanner struct {
	config *Config
}

// NewFileTreeScanner creates a new FileTreeScanner with the given configuration.
func NewFileTreeScanner(config *Config) *FileTreeScanner {
	if config == nil {
		config = DefaultConfig()
	}
	return &FileTreeScanner{
		config: config,
	}
}

// ScanDirectory scans a directory and returns a ScanResult.
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

	renderer := &StandardTreeRenderer{}
	treeText := renderer.RenderTree(root)

	return &ScanResult{
		RootPath:  path,
		TreeText:  treeText,
		NodeCount: nodeCount,
		Error:     nil,
		Root:      root, // Include the root TreeNode
	}, nil
}

// scanNode recursively scans a directory node.
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

// StandardTreeRenderer implements TreeRenderer for standard tree visualization.
type StandardTreeRenderer struct{}

// RenderTree renders a tree structure as a formatted string.
func (r *StandardTreeRenderer) RenderTree(root *TreeNode) string {
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
func (r *StandardTreeRenderer) renderNode(builder *strings.Builder, node *TreeNode, prefix string, isRoot bool) {
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

// FyneClipboardManager implements ClipboardManager using Fyne's clipboard.
type FyneClipboardManager struct {
	clipboard fyne.Clipboard
}

// NewFyneClipboardManager creates a new FyneClipboardManager.
func NewFyneClipboardManager(clipboard fyne.Clipboard) *FyneClipboardManager {
	return &FyneClipboardManager{clipboard: clipboard}
}

// SetContent sets the clipboard content.
func (c *FyneClipboardManager) SetContent(content string) error {
	if c.clipboard == nil {
		return fmt.Errorf("clipboard is not available")
	}
	c.clipboard.SetContent(content)
	return nil
}

// FileTreeApp represents the main application.
type FileTreeApp struct {
	// Core components
	app    fyne.App
	window fyne.Window
	config *Config

	// Services
	scanner   FileSystemScanner
	clipboard ClipboardManager

	// UI components
	tree        *widget.Tree
	statusLabel *widget.Label

	// State (NO mutex)
	treeData      map[string][]string
	currentResult *ScanResult

	// Context for cancelling operations
	cancelFunc context.CancelFunc
}

// NewFileTreeApp creates a new FileTreeApp with the given configuration.
func NewFileTreeApp(config *Config) *FileTreeApp {
	if config == nil {
		config = DefaultConfig()
	}

	fyneApp := app.New()
	fyneApp.SetIcon(theme.FolderIcon())

	window := fyneApp.NewWindow(appTitle)
	window.Resize(fyne.NewSize(windowWidth, windowHeight))

	scanner := NewFileTreeScanner(config)
	clipboard := NewFyneClipboardManager(fyneApp.Clipboard())

	return &FileTreeApp{
		app:         fyneApp,
		window:      window,
		config:      config,
		scanner:     scanner,
		clipboard:   clipboard,
		treeData:    make(map[string][]string),
		statusLabel: widget.NewLabel("Application started. Ready to scan"),
	}
}

// Run starts the application.
func (app *FileTreeApp) Run() {
	content := app.createMainContent()
	app.window.SetContent(content)
	app.enableDragDrop()
	app.window.ShowAndRun()
}

// createMainContent creates the main UI content.
func (app *FileTreeApp) createMainContent() fyne.CanvasObject {
	// Header
	title := widget.NewLabel("Main Menu")
	title.TextStyle.Bold = true

	// Buttons
	selectBtn := widget.NewButton(folderIcon+" Select Folder", app.handleSelectFolder)
	saveBtn := widget.NewButton("💾 Save to File", app.handleSaveToFile)
	copyBtn := widget.NewButton("📋 Copy to Clipboard", app.handleCopyToClipboard)

	buttonContainer := container.NewGridWithColumns(3,
		selectBtn,
		saveBtn,
		copyBtn,
	)

	// Initialize tree
	app.tree = app.createTree()

	// Main layout
	header := container.NewVBox(title, buttonContainer, app.statusLabel)
	content := container.NewBorder(header, nil, nil, nil, app.tree)

	return content
}

// createTree creates the tree widget.
func (app *FileTreeApp) createTree() *widget.Tree {
	return widget.NewTree(
		app.childUIDs,
		app.isBranch,
		app.createTreeNode,
		app.updateTreeNode,
	)
}

// childUIDs returns child UIDs for the tree widget.
func (app *FileTreeApp) childUIDs(uid string) []string {
	// Always on UI thread, safe
	if uid == "" && app.currentResult != nil {
		return []string{app.currentResult.RootPath}
	}
	return app.treeData[uid]
}

// isBranch determines if a tree node is a branch (directory).
func (app *FileTreeApp) isBranch(uid string) bool {
	if uid == "" {
		return true
	}
	children, exists := app.treeData[uid]
	return exists && len(children) > 0
}

// createTreeNode creates a new tree node widget.
func (app *FileTreeApp) createTreeNode(branch bool) fyne.CanvasObject {
	icon := fileIcon
	if branch {
		icon = folderIcon
	}
	return widget.NewLabel(icon + " Item")
}

// updateTreeNode updates a tree node widget.
func (app *FileTreeApp) updateTreeNode(uid string, branch bool, obj fyne.CanvasObject) {
	label, ok := obj.(*widget.Label)
	if !ok {
		return
	}

	name := filepath.Base(uid)
	if uid == app.getCurrentRootPath() {
		name = uid // Show full path for root
	}

	icon := fileIcon
	if branch {
		icon = folderIcon
	}

	label.SetText(icon + " " + name)
}

// getCurrentRootPath returns the current root path.
func (app *FileTreeApp) getCurrentRootPath() string {
	if app.currentResult != nil {
		return app.currentResult.RootPath
	}
	return ""
}

// handleSelectFolder handles folder selection.
func (app *FileTreeApp) handleSelectFolder() {
	folderDialog := dialog.NewFolderOpen(func(folder fyne.ListableURI, err error) {
		if err != nil {
			app.showError("Folder Selection Error", err)
			return
		}
		if folder == nil {
			return // User cancelled
		}

		app.scanDirectoryAsync(folder.Path())
	}, app.window)

	folderDialog.Show()
}

// scanDirectoryAsync scans a directory asynchronously.
func (app *FileTreeApp) scanDirectoryAsync(path string) {
	// Cancel any ongoing operation
	if app.cancelFunc != nil {
		app.cancelFunc()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	app.cancelFunc = cancel

	// Create progress dialog
	progress := dialog.NewProgressInfinite("Scanning", msgScanning, app.window)

	// Initial UI updates using fyne.Do() - the correct Fyne v2.6+ API
	fyne.Do(func() {
		progress.Show()
		app.statusLabel.SetText("Scanning: " + path)
	})

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Panic during scan: %v", r)
				// UI updates must use fyne.Do()
				fyne.Do(func() {
					app.statusLabel.SetText("Scan failed due to panic")
				})
			}
			// UI updates must use fyne.Do()
			fyne.Do(func() {
				progress.Hide()
			})
			cancel()
		}()

		result, err := app.scanner.ScanDirectory(ctx, path)

		// ALL UI updates using correct Fyne v2.6+ API
		fyne.Do(func() {
			if err != nil {
				if err == context.Canceled {
					app.statusLabel.SetText("Scan cancelled")
					return
				}
				if err == context.DeadlineExceeded {
					app.statusLabel.SetText("Scan timed out (directory too large)")
					return
				}
				app.showError("Scan Error", err)
				app.statusLabel.SetText("Scan failed")
				return
			}

			// Update tree data and UI (no locks!)
			app.updateTreeDataSimple(result)
			app.statusLabel.SetText(fmt.Sprintf("Scanned %d items from: %s", result.NodeCount, path))
			dialog.ShowInformation("Success", msgScanSuccess, app.window)
		})
	}()
}

// updateTreeDataSimple updates the tree data with scan results using a simpler approach.
func (app *FileTreeApp) updateTreeDataSimple(result *ScanResult) {
	app.currentResult = result
	app.treeData = make(map[string][]string)

	// Build tree data from the complete TreeNode structure
	if result.Root != nil {
		app.buildTreeDataFromTreeNode(result.Root)
	}

	// Refresh tree on UI thread
	if app.tree != nil {
		app.tree.Refresh()
	}
}

// buildTreeDataFromTreeNode recursively builds tree data from TreeNode structure.
func (app *FileTreeApp) buildTreeDataFromTreeNode(node *TreeNode) {
	if node == nil {
		return
	}

	var children []string
	for _, child := range node.Children {
		children = append(children, child.Path)
		// Recursively process children
		app.buildTreeDataFromTreeNode(child)
	}
	app.treeData[node.Path] = children
}

// handleSaveToFile handles saving tree to file.
func (app *FileTreeApp) handleSaveToFile() {
	result := app.getCurrentResult()
	if result == nil {
		dialog.ShowInformation("No Data", msgNoData, app.window)
		return
	}

	timestamp := time.Now().Format(timeFormat)
	defaultName := fmt.Sprintf("file_tree_%s%s", timestamp, defaultFileExt)

	// Async dialog
	saveDialog := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil {
			app.showError("Save Error", err)
			return
		}
		if writer == nil {
			return // User cancelled
		}
		defer writer.Close()

		_, werr := writer.Write([]byte(result.TreeText))
		if werr != nil {
			app.showError("Save Error", werr)
			return
		}

		dialog.ShowInformation("Success", msgSaveSuccess, app.window)
	}, app.window)

	saveDialog.SetFileName(defaultName)
	saveDialog.Show()
}

// handleCopyToClipboard handles copying tree to clipboard.
func (app *FileTreeApp) handleCopyToClipboard() {
	result := app.getCurrentResult()
	if result == nil {
		dialog.ShowInformation("No Data", msgNoData, app.window)
		return
	}

	err := app.clipboard.SetContent(result.TreeText)
	if err != nil {
		app.showError("Clipboard Error", err)
		return
	}

	dialog.ShowInformation("Success", msgCopySuccess, app.window)
}

// getCurrentResult returns the current scan result.
func (app *FileTreeApp) getCurrentResult() *ScanResult {
	return app.currentResult
}

// showError shows an error dialog.
func (app *FileTreeApp) showError(title string, err error) {
	dialog.ShowError(fmt.Errorf("%s: %w", title, err), app.window)
}

func (app *FileTreeApp) enableDragDrop() {
	app.window.SetOnDropped(func(position fyne.Position, uris []fyne.URI) {
		if len(uris) > 0 {
			uri := uris[0] // Take first dropped item

			// Convert URI to local path
			if uri.Scheme() == "file" {
				path := uri.Path()

				// Check if it's a directory
				if info, err := os.Stat(path); err == nil && info.IsDir() {
					app.scanDirectoryAsync(path)
				} else {
					dialog.ShowError(fmt.Errorf("please drop a folder, not a file"), app.window)
				}
			} else {
				dialog.ShowError(fmt.Errorf("invalid file path"), app.window)
			}
		}
	})
}

func main() {
	log.Println("Starting File Tree Scanner...")

	config := DefaultConfig()
	log.Printf("Config: MaxDepth=%d, ShowHidden=%v", config.MaxDepth, config.ShowHidden)

	app := NewFileTreeApp(config)
	log.Println("App created, starting UI...")

	app.Run()
}
