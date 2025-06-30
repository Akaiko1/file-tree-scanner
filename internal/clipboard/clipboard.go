package clipboard

import (
	"fmt"

	"fyne.io/fyne/v2"
)

// ClipboardManager defines the interface for clipboard operations.
type ClipboardManager interface {
	SetContent(content string) error
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