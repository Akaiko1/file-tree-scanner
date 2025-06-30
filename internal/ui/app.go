package ui

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/Akaiko1/file-tree-scanner/internal/clipboard"
	"github.com/Akaiko1/file-tree-scanner/internal/config"
	"github.com/Akaiko1/file-tree-scanner/internal/renderer"
	"github.com/Akaiko1/file-tree-scanner/internal/scanner"
)

const (
	// UI Constants
	appTitle     = "File Tree Scanner: AI Agent helper"
	windowWidth  = 800
	windowHeight = 600

	// Icons
	folderIcon = "📁"
	fileIcon   = "📄"

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

// FileTreeApp represents the main GUI application for directory tree scanning and visualization.
type FileTreeApp struct {
	// Core components
	app    fyne.App
	window fyne.Window
	config *config.Config

	// Services
	scanner   scanner.FileSystemScanner
	renderer  renderer.TreeRenderer
	clipboard clipboard.ClipboardManager

	// UI components
	tree        *widget.Tree
	statusLabel *widget.Label

	// State - UI thread only, no synchronization needed
	treeData      map[string][]string
	currentResult *scanner.ScanResult

	// Context for cancelling operations
	cancelFunc context.CancelFunc
}

// NewFileTreeApp creates a new FileTreeApp with the given configuration.
func NewFileTreeApp(cfg *config.Config) *FileTreeApp {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	fyneApp := app.New()
	fyneApp.SetIcon(theme.FolderIcon())

	window := fyneApp.NewWindow(appTitle)
	window.Resize(fyne.NewSize(windowWidth, windowHeight))

	scanner := scanner.NewFileTreeScanner(cfg)
	renderer := &renderer.StandardTreeRenderer{}
	clipboard := clipboard.NewFyneClipboardManager(fyneApp.Clipboard())

	return &FileTreeApp{
		app:         fyneApp,
		window:      window,
		config:      cfg,
		scanner:     scanner,
		renderer:    renderer,
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
	progressBar := widget.NewProgressBarInfinite()
	progressBar.Start()
	progress := dialog.NewCustomWithoutButtons("Scanning", progressBar, app.window)

	// UI updates must be dispatched to the main thread
	fyne.Do(func() {
		progress.Show()
		app.statusLabel.SetText("Scanning: " + path)
	})

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Panic during scan: %v", r)
				// UI updates must use main thread dispatcher
				fyne.Do(func() {
					app.statusLabel.SetText("Scan failed due to panic")
				})
			}
			// UI updates must use main thread dispatcher
			fyne.Do(func() {
				progressBar.Stop()
				progress.Hide()
			})
			cancel()
		}()

		result, err := app.scanner.ScanDirectory(ctx, path)

		// Generate tree text using renderer
		if result != nil && result.Root != nil {
			result.TreeText = app.renderer.RenderTree(result.Root)
		}

		// UI updates must use main thread dispatcher
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
func (app *FileTreeApp) updateTreeDataSimple(result *scanner.ScanResult) {
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
func (app *FileTreeApp) buildTreeDataFromTreeNode(node *scanner.TreeNode) {
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
func (app *FileTreeApp) getCurrentResult() *scanner.ScanResult {
	return app.currentResult
}

// showError shows an error dialog.
func (app *FileTreeApp) showError(title string, err error) {
	dialog.ShowError(fmt.Errorf("%s: %w", title, err), app.window)
}

// enableDragDrop enables drag and drop functionality.
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