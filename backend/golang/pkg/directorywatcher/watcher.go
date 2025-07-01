package directorywatcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/fsnotify/fsnotify"
	"github.com/pkg/errors"
	"go.temporal.io/sdk/client"

	"github.com/EternisAI/enchanted-twin/pkg/db"
)

type DirectoryWatcher struct {
	watcher        *fsnotify.Watcher
	store          *db.Store
	logger         *log.Logger
	temporalClient client.Client
	watchDir       string
	shutdownCh     chan struct{}
	fileBuffer     map[string]*FileEvent
	bufferTimer    *time.Timer
	bufferMu       sync.Mutex
	bufferDuration time.Duration
	mu             sync.RWMutex
}

type FileEvent struct {
	Path      string    `json:"path"`
	Operation string    `json:"operation"`
	Timestamp time.Time `json:"timestamp"`
}

func NewDirectoryWatcher(store *db.Store, logger *log.Logger, temporalClient client.Client, watchDir string) (*DirectoryWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create file watcher")
	}

	// Default buffer duration of 5 seconds to batch rapid file events
	bufferDuration := 5 * time.Second

	dw := &DirectoryWatcher{
		watcher:        watcher,
		store:          store,
		logger:         logger,
		temporalClient: temporalClient,
		watchDir:       watchDir,
		shutdownCh:     make(chan struct{}),
		fileBuffer:     make(map[string]*FileEvent),
		bufferDuration: bufferDuration,
	}

	return dw, nil
}

func (dw *DirectoryWatcher) Start(ctx context.Context) error {
	dw.logger.Info("Starting directory watcher", "watchDir", dw.watchDir)

	// Ensure the watch directory exists
	if err := os.MkdirAll(dw.watchDir, 0o755); err != nil {
		return errors.Wrap(err, "failed to create watch directory")
	}

	// Add directory to watcher
	if err := dw.watcher.Add(dw.watchDir); err != nil {
		return errors.Wrap(err, "failed to add directory to watcher")
	}

	// Start the main event loop
	go dw.eventLoop()

	// Perform initial scan for existing files
	go dw.performInitialScan(ctx)

	dw.logger.Info("Directory watcher started successfully")
	return nil
}

func (dw *DirectoryWatcher) Stop() error {
	dw.logger.Info("Stopping directory watcher")
	close(dw.shutdownCh)

	if dw.bufferTimer != nil {
		dw.bufferTimer.Stop()
	}

	return dw.watcher.Close()
}

func (dw *DirectoryWatcher) eventLoop() {
	for {
		select {
		case event, ok := <-dw.watcher.Events:
			if !ok {
				dw.logger.Info("File watcher events channel closed")
				return
			}
			dw.handleFileEvent(event)

		case err, ok := <-dw.watcher.Errors:
			if !ok {
				dw.logger.Info("File watcher errors channel closed")
				return
			}
			dw.logger.Error("File watcher error", "error", err)

		case <-dw.shutdownCh:
			dw.logger.Info("Directory watcher shutting down")
			return
		}
	}
}

func (dw *DirectoryWatcher) handleFileEvent(event fsnotify.Event) {
	// Skip directory events
	if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
		return
	}

	// Skip temporary files and hidden files
	fileName := filepath.Base(event.Name)
	if strings.HasPrefix(fileName, ".") || strings.HasSuffix(fileName, ".tmp") {
		return
	}

	// Only process supported file types
	if !dw.isSupportedFile(event.Name) {
		return
	}

	dw.logger.Debug("File event", "path", event.Name, "op", event.Op.String())

	// Buffer the event to handle rapid changes
	dw.bufferFileEvent(&FileEvent{
		Path:      event.Name,
		Operation: event.Op.String(),
		Timestamp: time.Now(),
	})
}

func (dw *DirectoryWatcher) bufferFileEvent(event *FileEvent) {
	dw.bufferMu.Lock()
	defer dw.bufferMu.Unlock()

	// Use the file path as the key to deduplicate rapid events
	dw.fileBuffer[event.Path] = event

	// Reset or create buffer timer
	if dw.bufferTimer != nil {
		dw.bufferTimer.Stop()
	}

	dw.bufferTimer = time.AfterFunc(dw.bufferDuration, func() {
		dw.processBatchedEvents()
	})
}

func (dw *DirectoryWatcher) processBatchedEvents() {
	dw.bufferMu.Lock()
	events := make([]*FileEvent, 0, len(dw.fileBuffer))
	for _, event := range dw.fileBuffer {
		events = append(events, event)
	}
	dw.fileBuffer = make(map[string]*FileEvent) // Clear buffer
	dw.bufferMu.Unlock()

	if len(events) == 0 {
		return
	}

	dw.logger.Info("Processing batched file events", "count", len(events))

	for _, event := range events {
		if strings.Contains(event.Operation, "CREATE") || strings.Contains(event.Operation, "WRITE") {
			if err := dw.processNewFile(event.Path); err != nil {
				dw.logger.Error("Failed to process new file", "error", err, "path", event.Path)
			}
		} else if strings.Contains(event.Operation, "REMOVE") {
			if err := dw.processRemovedFile(event.Path); err != nil {
				dw.logger.Error("Failed to process removed file", "error", err, "path", event.Path)
			}
		}
	}

	// After processing new files, trigger the initialize workflow
	if err := dw.triggerProcessingWorkflow(); err != nil {
		dw.logger.Error("Failed to trigger processing workflow", "error", err)
	}
}

func (dw *DirectoryWatcher) processNewFile(filePath string) error {
	ctx := context.Background()

	// Convert to absolute path for consistency
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		dw.logger.Error("Failed to resolve absolute path", "error", err, "path", filePath)
		// Use original path as fallback
		absPath = filePath
	}

	dw.logger.Debug("Processing new file", "originalPath", filePath, "absolutePath", absPath)

	// Check if active data source already exists for this path
	exists, err := dw.store.ActiveDataSourceExistsByPath(ctx, absPath)
	if err != nil {
		dw.logger.Error("Failed to check if active data source exists", "error", err, "path", absPath)
		return errors.Wrap(err, "failed to check if active data source exists")
	}

	if exists {
		dw.logger.Info("Active data source exists - marking as replaced and creating new version", "path", absPath)

		// Mark existing active data source as replaced
		if err := dw.store.MarkDataSourceAsReplaced(ctx, absPath); err != nil {
			dw.logger.Error("Failed to mark existing data source as replaced", "error", err, "path", absPath)
			return errors.Wrap(err, "failed to mark existing data source as replaced")
		}
	}

	// Determine data source type from file extension/name
	dataSourceName := dw.determineDataSourceType(filePath)
	if dataSourceName == "" {
		dw.logger.Warn("Unsupported file type", "path", filePath)
		return nil
	}

	dw.logger.Debug("Determined data source type", "path", filePath, "type", dataSourceName)

	// Create new data source
	dataSourceID, err := dw.store.CreateDataSourceFromFile(ctx, &db.CreateDataSourceFromFileInput{
		Name: dataSourceName,
		Path: absPath,
	})
	if err != nil {
		dw.logger.Error("Failed to create data source from file", "error", err, "path", absPath, "type", dataSourceName)
		return errors.Wrap(err, "failed to create data source from file")
	}

	dw.logger.Info("Created data source for file", "path", absPath, "id", dataSourceID, "type", dataSourceName)
	return nil
}

func (dw *DirectoryWatcher) processRemovedFile(filePath string) error {
	ctx := context.Background()

	// Convert to absolute path for consistency
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		dw.logger.Error("Failed to resolve absolute path for removal", "error", err, "path", filePath)
		absPath = filePath
	}

	dw.logger.Debug("Processing removed file", "originalPath", filePath, "absolutePath", absPath)

	// Mark active data source as deleted
	if err := dw.store.MarkDataSourceAsDeleted(ctx, absPath); err != nil {
		dw.logger.Error("Failed to mark data source as deleted", "error", err, "path", absPath)
		return errors.Wrap(err, "failed to mark data source as deleted")
	}

	dw.logger.Info("Marked data source as deleted", "path", absPath)
	return nil
}

func (dw *DirectoryWatcher) triggerProcessingWorkflow() error {
	workflowOptions := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("auto-initialize-%d", time.Now().Unix()),
		TaskQueue: "default",
	}

	_, err := dw.temporalClient.ExecuteWorkflow(
		context.Background(),
		workflowOptions,
		"InitializeWorkflow",
		map[string]interface{}{},
	)
	if err != nil {
		return errors.Wrap(err, "failed to start initialize workflow")
	}

	dw.logger.Info("Triggered processing workflow", "workflowID", workflowOptions.ID)
	return nil
}

func (dw *DirectoryWatcher) performInitialScan(ctx context.Context) {
	dw.logger.Info("Starting initial scan of watch directory")

	err := filepath.Walk(dw.watchDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Skip hidden and temporary files
		fileName := filepath.Base(path)
		if strings.HasPrefix(fileName, ".") || strings.HasSuffix(fileName, ".tmp") {
			return nil
		}

		// Check if supported file type
		if !dw.isSupportedFile(path) {
			return nil
		}

		// Convert to absolute path for consistency
		absPath, err := filepath.Abs(path)
		if err != nil {
			dw.logger.Error("Failed to resolve absolute path during scan", "error", err, "path", path)
			absPath = path // Use original path as fallback
		}

		// Check if already tracked as active
		exists, err := dw.store.ActiveDataSourceExistsByPath(ctx, absPath)
		if err != nil {
			dw.logger.Error("Failed to check active data source existence", "error", err, "path", absPath)
			return nil
		}

		if !exists {
			if err := dw.processNewFile(path); err != nil {
				dw.logger.Error("Failed to process file during initial scan", "error", err, "path", path)
			}
		}

		return nil
	})

	if err != nil {
		dw.logger.Error("Error during initial scan", "error", err)
	} else {
		dw.logger.Info("Initial scan completed")

		// Trigger processing workflow after initial scan
		if err := dw.triggerProcessingWorkflow(); err != nil {
			dw.logger.Error("Failed to trigger processing workflow after initial scan", "error", err)
		}
	}
}

func (dw *DirectoryWatcher) isSupportedFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	fileName := strings.ToLower(filepath.Base(filePath))

	// Supported file extensions
	supportedExts := []string{
		".json", ".mbox", ".zip", ".sqlite", ".db", ".tar", ".tar.gz",
	}

	for _, supportedExt := range supportedExts {
		if ext == supportedExt {
			return true
		}
	}

	// Special case for WhatsApp database files
	if strings.Contains(fileName, "whatsapp") && (ext == ".db" || ext == ".sqlite") {
		return true
	}

	return false
}

func (dw *DirectoryWatcher) determineDataSourceType(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	fileName := strings.ToLower(filepath.Base(filePath))

	// Determine data source type based on file characteristics
	switch {
	case strings.Contains(fileName, "whatsapp") && (ext == ".db" || ext == ".sqlite"):
		return "WhatsApp"
	case strings.Contains(fileName, "telegram") && ext == ".json":
		return "Telegram"
	case strings.Contains(fileName, "gmail") || ext == ".mbox":
		return "Gmail"
	case strings.Contains(fileName, "slack") && ext == ".zip":
		return "Slack"
	case strings.Contains(fileName, "twitter") || strings.Contains(fileName, "x") && ext == ".zip":
		return "X"
	case strings.Contains(fileName, "chatgpt") && (ext == ".json" || ext == ".zip"):
		return "ChatGPT"
	case ext == ".json":
		// Default JSON files to Telegram (can be adjusted based on content analysis)
		return "Telegram"
	case ext == ".zip":
		// Generic ZIP files, could be any export
		return "X" // Default to X for now
	default:
		return ""
	}
}
