package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

func main() {
	fmt.Println("🚀 Simple File Watching Test")
	fmt.Println("Testing basic file detection in ./input_data/")

	// Create the watch directory
	watchDir := "./input_data"
	if err := os.MkdirAll(watchDir, 0o755); err != nil {
		fmt.Printf("Failed to create directory: %v\n", err)
		return
	}

	// Create file watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Printf("Failed to create watcher: %v\n", err)
		return
	}
	defer watcher.Close()

	// Add directory to watcher
	err = watcher.Add(watchDir)
	if err != nil {
		fmt.Printf("Failed to watch directory: %v\n", err)
		return
	}

	fmt.Printf("✅ Watching directory: %s\n", watchDir)
	fmt.Println("📄 Create a .json file in the directory to test...")

	// Start watching in background
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				fmt.Printf("🔔 File event: %s %s\n", event.Op, event.Name)

				// Check if it's a JSON file
				if filepath.Ext(event.Name) == ".json" {
					fmt.Printf("✅ JSON file detected: %s\n", filepath.Base(event.Name))

					// Check file content
					if event.Op&fsnotify.Create == fsnotify.Create || event.Op&fsnotify.Write == fsnotify.Write {
						content, err := os.ReadFile(event.Name)
						if err != nil {
							fmt.Printf("❌ Failed to read file: %v\n", err)
						} else {
							fmt.Printf("📄 File content: %s\n", string(content))
							fmt.Printf("🎯 FilePath that would be stored: %s\n", event.Name)
						}
					}
				}

			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				fmt.Printf("❌ Watcher error: %v\n", err)
			}
		}
	}()

	// Keep the program running
	fmt.Println("⏳ Waiting for file events (press Ctrl+C to exit)...")
	time.Sleep(30 * time.Second)
	fmt.Println("�� Test completed")
}
