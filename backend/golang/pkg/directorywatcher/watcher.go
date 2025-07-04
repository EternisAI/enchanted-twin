package directorywatcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/fsnotify/fsnotify"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory/storage"
)

type DirectoryWatcher struct {
	logger        *log.Logger
	watcher       *fsnotify.Watcher
	storage       storage.Interface
	watchedPaths  []string
	stopChan      chan struct{}
	running       bool
	memoryService memory.Storage
}

type WatcherConfig struct {
	Logger        *log.Logger
	Storage       storage.Interface
	WatchedPaths  []string
	MemoryService memory.Storage
}

func NewDirectoryWatcher(config WatcherConfig) (*DirectoryWatcher, error) {
	if config.Logger == nil {
		return nil, fmt.Errorf("logger is required")
	}
	if config.Storage == nil {
		return nil, fmt.Errorf("storage is required")
	}

	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	return &DirectoryWatcher{
		logger:        config.Logger,
		watcher:       fsWatcher,
		storage:       config.Storage,
		watchedPaths:  config.WatchedPaths,
		stopChan:      make(chan struct{}),
		memoryService: config.MemoryService,
	}, nil
}

func (dw *DirectoryWatcher) Start(ctx context.Context) error {
	if dw.running {
		return fmt.Errorf("directory watcher is already running")
	}

	dw.running = true
	dw.logger.Info("Starting directory watcher", "paths", dw.watchedPaths)

	// Add watched paths
	for _, path := range dw.watchedPaths {
		if err := dw.addWatchPath(path); err != nil {
			dw.logger.Error("Failed to add watch path", "path", path, "error", err)
			continue
		}
		dw.logger.Info("Added watch path", "path", path)
	}

	// Start the event loop
	go dw.eventLoop(ctx)

	return nil
}

func (dw *DirectoryWatcher) Stop() {
	if !dw.running {
		return
	}

	dw.logger.Info("Stopping directory watcher")
	close(dw.stopChan)
	dw.watcher.Close()
	dw.running = false
}

func (dw *DirectoryWatcher) addWatchPath(path string) error {
	// Check if path exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("path does not exist: %s", path)
	}

	// Add the path to the watcher
	if err := dw.watcher.Add(path); err != nil {
		return fmt.Errorf("failed to add path to watcher: %w", err)
	}

	return nil
}

func (dw *DirectoryWatcher) eventLoop(ctx context.Context) {
	dw.logger.Info("Directory watcher event loop started")

	for {
		select {
		case <-ctx.Done():
			dw.logger.Info("Directory watcher stopped due to context cancellation")
			return
		case <-dw.stopChan:
			dw.logger.Info("Directory watcher stopped")
			return
		case event, ok := <-dw.watcher.Events:
			if !ok {
				dw.logger.Warn("Watcher events channel closed")
				return
			}
			dw.handleFileEvent(ctx, event)
		case err, ok := <-dw.watcher.Errors:
			if !ok {
				dw.logger.Warn("Watcher errors channel closed")
				return
			}
			dw.logger.Error("File watcher error", "error", err)
		}
	}
}

func (dw *DirectoryWatcher) handleFileEvent(ctx context.Context, event fsnotify.Event) {
	dw.logger.Info("File event detected", "event", event.Op.String(), "path", event.Name)

	switch {
	case event.Op&fsnotify.Create == fsnotify.Create:
		dw.logger.Info("File created", "path", event.Name)
		if err := dw.handleFileCreated(ctx, event.Name); err != nil {
			dw.logger.Error("Failed to handle file creation", "path", event.Name, "error", err)
		}
	case event.Op&fsnotify.Write == fsnotify.Write:
		dw.logger.Info("File modified", "path", event.Name)
		if err := dw.handleFileModified(ctx, event.Name); err != nil {
			dw.logger.Error("Failed to handle file modification", "path", event.Name, "error", err)
		}
	case event.Op&fsnotify.Remove == fsnotify.Remove:
		dw.logger.Info("File removed", "path", event.Name)
		if err := dw.handleFileRemoved(ctx, event.Name); err != nil {
			dw.logger.Error("Failed to handle file removal", "path", event.Name, "error", err)
		}
	case event.Op&fsnotify.Rename == fsnotify.Rename:
		dw.logger.Info("File renamed", "path", event.Name)
		if err := dw.handleFileRemoved(ctx, event.Name); err != nil {
			dw.logger.Error("Failed to handle file rename", "path", event.Name, "error", err)
		}
	}
}

func (dw *DirectoryWatcher) handleFileCreated(ctx context.Context, filePath string) error {
	// Check if it's a regular file
	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	if info.IsDir() {
		dw.logger.Debug("Ignoring directory", "path", filePath)
		return nil
	}

	// Skip hidden files and temporary files
	filename := filepath.Base(filePath)
	if strings.HasPrefix(filename, ".") || strings.HasPrefix(filename, "~") {
		dw.logger.Debug("Ignoring hidden/temp file", "path", filePath)
		return nil
	}

	dw.logger.Info("Processing new file", "path", filePath)

	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Create document for storage
	doc := &memory.TextDocument{
		FieldID:       filePath,
		FieldContent:  string(content),
		FieldSource:   "file_system",
		FieldMetadata: map[string]string{
			"path":         filePath,
			"filename":     filename,
			"size":         fmt.Sprintf("%d", len(content)),
			"created_at":   time.Now().Format(time.RFC3339),
			"content_type": "text/plain",
		},
	}

	// Store as SourceDocument
	documentID, err := dw.storage.UpsertDocument(ctx, doc)
	if err != nil {
		return fmt.Errorf("failed to store document: %w", err)
	}

	dw.logger.Info("File stored in memory", "path", filePath, "document_id", documentID)

	// Also store in evolving memory if service is available
	if dw.memoryService != nil {
		dw.logger.Info("Storing file in evolving memory", "path", filePath)
		if err := dw.memoryService.Store(ctx, []memory.Document{doc}, nil); err != nil {
			dw.logger.Error("Failed to store in evolving memory", "path", filePath, "error", err)
			// Don't return error, as the file was successfully stored in SourceDocument
		} else {
			dw.logger.Info("File stored in evolving memory", "path", filePath)
		}
	}

	return nil
}

func (dw *DirectoryWatcher) handleFileModified(ctx context.Context, filePath string) error {
	// For now, treat modifications the same as creation
	return dw.handleFileCreated(ctx, filePath)
}

func (dw *DirectoryWatcher) handleFileRemoved(ctx context.Context, filePath string) error {
	dw.logger.Info("Attempting to remove file from memory", "path", filePath)

	// We need to find the document by path and remove it
	// For now, we'll log the removal attempt
	dw.logger.Info("File removal detected - would remove from memory", "path", filePath)

	// TODO: Implement document removal by path
	// This would require adding a method to find documents by path
	
	return nil
}

// Helper function to check if a file should be processed
func (dw *DirectoryWatcher) shouldProcessFile(filePath string) bool {
	// Skip hidden files
	if strings.HasPrefix(filepath.Base(filePath), ".") {
		return false
	}

	// Skip temporary files
	if strings.HasPrefix(filepath.Base(filePath), "~") {
		return false
	}

	// You can add more filters here (file extensions, size limits, etc.)
	return true
}

// ProcessExistingFiles processes all existing files in the watched directories
func (dw *DirectoryWatcher) ProcessExistingFiles(ctx context.Context) error {
	dw.logger.Info("Processing existing files in watched directories")

	for _, watchPath := range dw.watchedPaths {
		if err := dw.processDirectoryFiles(ctx, watchPath); err != nil {
			dw.logger.Error("Failed to process directory", "path", watchPath, "error", err)
			continue
		}
	}

	return nil
}

func (dw *DirectoryWatcher) processDirectoryFiles(ctx context.Context, dirPath string) error {
	return filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if !dw.shouldProcessFile(path) {
			return nil
		}

		dw.logger.Info("Processing existing file", "path", path)
		return dw.handleFileCreated(ctx, path)
	})
}