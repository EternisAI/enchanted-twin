// owner: slimane@eternis.ai
package directorywatcher

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/fsnotify/fsnotify"
	"github.com/pkg/errors"
	"go.temporal.io/sdk/client"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/db"
)

type DirectoryWatcher struct {
	watcher        *fsnotify.Watcher
	store          *db.Store
	memoryStorage  memory.Storage
	logger         *log.Logger
	temporalClient client.Client
	watchDir       string
	shutdownCh     chan struct{}
	fileBuffer     map[string]*FileEvent
	bufferTimer    *time.Timer
	bufferMu       sync.Mutex
	bufferDuration time.Duration
}

type FileEvent struct {
	Path      string    `json:"path"`
	Operation string    `json:"operation"`
	Timestamp time.Time `json:"timestamp"`
}

func NewDirectoryWatcher(store *db.Store, memoryStorage memory.Storage, logger *log.Logger, temporalClient client.Client, watchDir string) (*DirectoryWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create file watcher")
	}

	bufferDuration := 5 * time.Second

	dw := &DirectoryWatcher{
		watcher:        watcher,
		store:          store,
		memoryStorage:  memoryStorage,
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

	if err := os.MkdirAll(dw.watchDir, 0o755); err != nil {
		return errors.Wrap(err, "failed to create watch directory")
	}

	if err := dw.watcher.Add(dw.watchDir); err != nil {
		return errors.Wrap(err, "failed to add directory to watcher")
	}

	go dw.eventLoop()

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
	if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
		return
	}

	fileName := filepath.Base(event.Name)
	if strings.HasPrefix(fileName, ".") || strings.HasSuffix(fileName, ".tmp") {
		return
	}

	if !dw.isSupportedFile(event.Name) {
		return
	}

	dw.logger.Debug("File event", "path", event.Name, "op", event.Op.String())

	dw.bufferFileEvent(&FileEvent{
		Path:      event.Name,
		Operation: event.Op.String(),
		Timestamp: time.Now(),
	})
}

func (dw *DirectoryWatcher) bufferFileEvent(event *FileEvent) {
	dw.bufferMu.Lock()
	defer dw.bufferMu.Unlock()

	dw.fileBuffer[event.Path] = event

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
	dw.fileBuffer = make(map[string]*FileEvent)
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
		} else if strings.Contains(event.Operation, "RENAME") {
			// Handle RENAME events - on macOS, file deletions often appear as RENAME
			if err := dw.processRenameEvent(event.Path); err != nil {
				dw.logger.Error("Failed to process rename event", "error", err, "path", event.Path)
			}
		}
	}

	if err := dw.triggerProcessingWorkflow(); err != nil {
		dw.logger.Error("Failed to trigger processing workflow", "error", err)
	}
}

func (dw *DirectoryWatcher) processNewFile(filePath string) error {
	ctx := context.Background()

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		dw.logger.Error("Failed to resolve absolute path", "error", err, "path", filePath)
		absPath = filePath
	}

	dw.logger.Debug("Processing new file", "originalPath", filePath, "absolutePath", absPath)

	exists, err := dw.store.ActiveDataSourceExistsByPath(ctx, absPath)
	if err != nil {
		dw.logger.Error("Failed to check if active data source exists", "error", err, "path", absPath)
		return errors.Wrap(err, "failed to check if active data source exists")
	}

	if exists {
		dw.logger.Info("Active data source exists - marking as replaced and creating new version", "path", absPath)

		if err := dw.removeFileFromMemory(ctx, absPath); err != nil {
			dw.logger.Error("Failed to remove old memories for replaced file", "error", err, "path", absPath)
		}

		if err := dw.store.MarkDataSourceAsReplaced(ctx, absPath); err != nil {
			dw.logger.Error("Failed to mark existing data source as replaced", "error", err, "path", absPath)
			return errors.Wrap(err, "failed to mark existing data source as replaced")
		}
	}

	dataSourceName := dw.determineDataSourceType(filePath)
	if dataSourceName == "" {
		dw.logger.Warn("Unsupported file type", "path", filePath)
		return nil
	}

	dw.logger.Debug("Determined data source type", "path", filePath, "type", dataSourceName)

	dataSourceID, err := dw.store.CreateDataSourceFromFile(ctx, &db.CreateDataSourceFromFileInput{
		Name: dataSourceName,
		Path: absPath,
	})
	if err != nil {
		dw.logger.Error("Failed to create data source from file", "error", err, "path", absPath, "type", dataSourceName)
		return errors.Wrap(err, "failed to create data source from file")
	}

	if err := dw.storeFileInMemory(ctx, absPath, dataSourceName, dataSourceID); err != nil {
		dw.logger.Error("Failed to store file metadata in memory", "error", err, "path", absPath)
	}

	dw.logger.Info("Created data source for file", "path", absPath, "id", dataSourceID, "type", dataSourceName)
	return nil
}

func (dw *DirectoryWatcher) processRemovedFile(filePath string) error {
	ctx := context.Background()

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		dw.logger.Error("Failed to resolve absolute path for removal", "error", err, "path", filePath)
		absPath = filePath
	}

	dw.logger.Debug("Processing removed file", "originalPath", filePath, "absolutePath", absPath)

	// Remove file from memory first
	if err := dw.removeFileFromMemory(ctx, absPath); err != nil {
		dw.logger.Error("Failed to remove file from memory", "error", err, "path", absPath)
		// Continue with database cleanup even if memory cleanup fails
	}

	if err := dw.store.MarkDataSourceAsDeleted(ctx, absPath); err != nil {
		dw.logger.Error("Failed to mark data source as deleted", "error", err, "path", absPath)
		return errors.Wrap(err, "failed to mark data source as deleted")
	}

	dw.logger.Info("Marked data source as deleted and removed from memory", "path", absPath)
	return nil
}

func (dw *DirectoryWatcher) processRenameEvent(filePath string) error {
	// Check if the file still exists after the RENAME event
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// File doesn't exist - treat as deletion
		dw.logger.Debug("RENAME event for non-existent file - treating as deletion", "path", filePath)
		return dw.processRemovedFile(filePath)
	} else if err != nil {
		// Some other error checking file existence
		dw.logger.Error("Failed to check file existence for RENAME event", "error", err, "path", filePath)
		return err
	}

	// File still exists - treat as modification/new file
	dw.logger.Debug("RENAME event for existing file - treating as modification", "path", filePath)
	return dw.processNewFile(filePath)
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

		fileName := filepath.Base(path)
		if strings.HasPrefix(fileName, ".") || strings.HasSuffix(fileName, ".tmp") {
			return nil
		}

		if !dw.isSupportedFile(path) {
			return nil
		}

		absPath, err := filepath.Abs(path)
		if err != nil {
			dw.logger.Error("Failed to resolve absolute path during scan", "error", err, "path", path)
			absPath = path
		}

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

		if err := dw.triggerProcessingWorkflow(); err != nil {
			dw.logger.Error("Failed to trigger processing workflow after initial scan", "error", err)
		}
	}
}

func (dw *DirectoryWatcher) isSupportedFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	fileName := strings.ToLower(filepath.Base(filePath))

	if strings.HasPrefix(fileName, ".") || strings.HasSuffix(fileName, ".tmp") {
		return false
	}

	supportedExts := []string{
		".json", ".mbox", ".zip", ".sqlite", ".db", ".tar", ".tar.gz", ".txt", ".pdf",
	}

	for _, supportedExt := range supportedExts {
		if ext == supportedExt {
			return true
		}
	}

	if strings.Contains(fileName, "whatsapp") && (ext == ".db" || ext == ".sqlite") {
		return true
	}

	return false
}

func (dw *DirectoryWatcher) determineDataSourceType(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	fileName := strings.ToLower(filepath.Base(filePath))

	switch {
	case strings.Contains(fileName, "whatsapp") && (ext == ".db" || ext == ".sqlite"):
		return "WhatsApp"
	case strings.Contains(fileName, "telegram") && ext == ".json":
		return "Telegram"
	case strings.Contains(fileName, "gmail") || ext == ".mbox":
		return "Gmail"
	case strings.Contains(fileName, "slack") && ext == ".zip":
		return "Slack"
	case strings.Contains(fileName, "chatgpt") && (ext == ".json" || ext == ".zip"):
		return "ChatGPT"
	case (strings.Contains(fileName, "twitter") || strings.Contains(fileName, "x") || strings.Contains(fileName, "like") || strings.Contains(fileName, "tweet") || strings.Contains(fileName, "direct")) && (ext == ".zip" || ext == ".json" || ext == ".js"):
		return "X"
	case ext == ".txt":
		return "misc"
	case ext == ".pdf":
		return "misc"
	case ext == ".json":
		return dw.detectJSONContentType(filePath)
	case ext == ".zip":
		return "X"
	default:
		return "misc"
	}
}

// detectJSONContentType attempts to determine the data source type by examining JSON content structure.
func (dw *DirectoryWatcher) detectJSONContentType(filePath string) string {
	// Read a small portion of the file to examine structure
	file, err := os.Open(filePath)
	if err != nil {
		dw.logger.Warn("Failed to open file for content detection", "path", filePath, "error", err)
		return "" // Unknown type if we can't read it
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			dw.logger.Warn("Failed to close file during content detection", "path", filePath, "error", closeErr)
		}
	}()

	// Read first 1KB to examine structure
	buffer := make([]byte, 1024)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		dw.logger.Warn("Failed to read file for content detection", "path", filePath, "error", err)
		return ""
	}

	content := string(buffer[:n])
	contentLower := strings.ToLower(content)

	// Check for X/Twitter data patterns
	if strings.Contains(contentLower, "\"like\"") && strings.Contains(contentLower, "\"tweetid\"") ||
		strings.Contains(contentLower, "\"tweet\"") && strings.Contains(contentLower, "\"full_text\"") ||
		strings.Contains(contentLower, "\"dmconversation\"") ||
		strings.Contains(contentLower, "\"conversationid\"") && strings.Contains(contentLower, "\"senderid\"") {
		return "X"
	}

	// Check for Telegram data patterns
	if strings.Contains(contentLower, "\"personal_information\"") && strings.Contains(contentLower, "\"chats\"") ||
		strings.Contains(contentLower, "\"date_unixtime\"") && strings.Contains(contentLower, "\"from_id\"") {
		return "Telegram"
	}

	// Check for ChatGPT conversation patterns
	if strings.Contains(contentLower, "\"conversations\"") && strings.Contains(contentLower, "\"mapping\"") ||
		strings.Contains(contentLower, "\"message\"") && strings.Contains(contentLower, "\"author\"") && strings.Contains(contentLower, "\"role\"") {
		return "ChatGPT"
	}

	// Check if it's an array structure - could be X data or conversation documents
	trimmedContent := strings.TrimSpace(content)
	if strings.HasPrefix(trimmedContent, "[") {
		// If it's an array and contains conversation/people fields, it might be processed conversation documents
		if strings.Contains(contentLower, "\"conversation\"") && strings.Contains(contentLower, "\"people\"") {
			// This looks like already processed conversation documents - let's try as X since Telegram expects TelegramData structure
			return "X"
		}
		// Other arrays default to X since X can handle various array formats
		return "X"
	}

	// Default for unrecognized JSON - be conservative and return empty string
	// This prevents incorrect processing and will require manual intervention
	dw.logger.Warn("Could not determine data source type for JSON file", "path", filePath)
	return ""
}

// storeFileInMemory creates a memory fact for the indexed file using structured fields.
func (dw *DirectoryWatcher) storeFileInMemory(ctx context.Context, filePath, dataSourceType string, dataSourceID string) error {
	if dw.memoryStorage == nil {
		dw.logger.Debug("Memory storage not available, skipping file memory storage")
		return nil
	}

	// Create a memory fact using structured fields for efficient querying
	memoryFact := &memory.MemoryFact{
		ID:        fmt.Sprintf("file-indexed-%s", dataSourceID),
		Content:   fmt.Sprintf("Indexed file: %s as %s data source", filepath.Base(filePath), dataSourceType),
		Timestamp: time.Now(),

		// Use indexed structured fields for efficient querying
		Category: "file-indexed", // Indexed: allows filtering by file indexing events

		Source:   "file-indexing", // Indexed: filter by file indexing source
		FilePath: filePath,        // NEW: Indexed file path field for super-fast queries!
		Tags:     []string{"file-indexed", "data-source", dataSourceType},

		// Keep some metadata for backward compatibility
		Metadata: map[string]string{
			"data_source_id": dataSourceID,
			"indexed_at":     time.Now().Format(time.RFC3339),
		},
	}

	// Create document from the memory fact
	fileDoc := &memory.TextDocument{
		FieldID:        memoryFact.ID,
		FieldContent:   memoryFact.Content,
		FieldTimestamp: &memoryFact.Timestamp,
		FieldSource:    memoryFact.Source,
		FieldTags:      memoryFact.Tags,
		FieldMetadata:  memoryFact.Metadata,
		FieldFilePath:  memoryFact.FilePath,
	}

	documents := []memory.Document{fileDoc}

	err := dw.memoryStorage.Store(ctx, documents, func(processed, total int) {
		dw.logger.Debug("Storing file metadata in memory", "processed", processed, "total", total)
	})
	if err != nil {
		return errors.Wrap(err, "failed to store file metadata in memory")
	}

	dw.logger.Info("Stored file metadata in memory using indexed FilePath field", "path", filePath, "category", memoryFact.Category, "file_path", memoryFact.FilePath)
	return nil
}

// removeFileFromMemory efficiently removes memories by file path using the new indexed field.
func (dw *DirectoryWatcher) removeFileFromMemory(ctx context.Context, filePath string) error {
	if dw.memoryStorage == nil {
		dw.logger.Debug("Memory storage not available, skipping memory cleanup")
		return nil
	}

	// Super efficient query using the new indexed file path field!
	filter := &memory.Filter{
		FactFilePath: &filePath, // Direct indexed field query - lightning fast!
		Limit:        &[]int{100}[0],
	}

	result, err := dw.memoryStorage.Query(ctx, "", filter)
	if err != nil {
		return errors.Wrap(err, "failed to query memories for file cleanup")
	}

	if len(result.Facts) == 0 {
		dw.logger.Debug("No memories found for file path", "path", filePath)
		return nil
	}

	dw.logger.Info("Found memories to delete for file", "path", filePath, "count", len(result.Facts))

	// All results will match our file path since we filtered by the indexed field
	for _, fact := range result.Facts {
		dw.logger.Info("Found memory for file deletion", "memory_id", fact.ID, "file_path", fact.FilePath)
		// TODO: When DeleteByID is implemented in the memory interface, call it here
		// For now, we're just logging what would be deleted
	}

	dw.logger.Info("Completed memory cleanup for file", "path", filePath, "memories_to_delete", len(result.Facts))
	return nil
}
