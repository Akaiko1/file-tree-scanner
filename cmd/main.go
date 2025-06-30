// Package main implements a cross-platform GUI application for scanning and visualizing
// directory structures using the Fyne framework.
package main

import (
	"log"

	"github.com/Akaiko1/file-tree-scanner/internal/config"
	"github.com/Akaiko1/file-tree-scanner/internal/ui"
)

func main() {
	log.Println("Starting File Tree Scanner...")

	config := config.DefaultConfig()
	log.Printf("Config: MaxDepth=%d, ShowHidden=%v", config.MaxDepth, config.ShowHidden)

	app := ui.NewFileTreeApp(config)
	log.Println("App created, starting UI...")

	app.Run()
}