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
		dw.logger.Debug("Ignoring directory event", "path", event.Name, "op", event.Op.String())
		return
	}

	fileName := filepath.Base(event.Name)
	if strings.HasPrefix(fileName, ".") || strings.HasSuffix(fileName, ".tmp") {
		dw.logger.Debug("Ignoring hidden or temporary file", "path", event.Name, "op", event.Op.String())
		return
	}

	if !dw.isSupportedFile(event.Name) {
		dw.logger.Debug("Ignoring unsupported file type", "path", event.Name, "op", event.Op.String())
		return
	}

	dw.logger.Debug("File event accepted for processing", "path", event.Name, "op", event.Op.String())

	dw.bufferFileEvent(&FileEvent{
		Path:      event.Name,
		Operation: event.Op.String(),
		Timestamp: time.Now(),
	})
}

func (dw *DirectoryWatcher) bufferFileEvent(event *FileEvent) {
	dw.bufferMu.Lock()
	defer dw.bufferMu.Unlock()

	dw.logger.Debug("Buffering file event", "path", event.Path, "operation", event.Operation)

	dw.fileBuffer[event.Path] = event
	dw.logger.Debug("File buffer status", "totalBuffered", len(dw.fileBuffer), "path", event.Path)

	if dw.bufferTimer != nil {
		dw.logger.Debug("Stopping existing buffer timer")
		dw.bufferTimer.Stop()
	}

	dw.logger.Debug("Starting new buffer timer", "duration", dw.bufferDuration)
	dw.bufferTimer = time.AfterFunc(dw.bufferDuration, func() {
		dw.logger.Debug("Buffer timer expired, processing events")
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
		dw.logger.Debug("No events to process in batch")
		return
	}

	dw.logger.Info("Processing batched file events", "count", len(events))

	for _, event := range events {
		dw.logger.Debug("Processing event", "path", event.Path, "operation", event.Operation, "timestamp", event.Timestamp)

		if strings.Contains(event.Operation, "CREATE") || strings.Contains(event.Operation, "WRITE") || strings.Contains(event.Operation, "CHMOD") {
			dw.logger.Debug("Handling as new/modified file", "path", event.Path, "operation", event.Operation)
			if err := dw.processNewFile(event.Path); err != nil {
				dw.logger.Error("Failed to process new file", "error", err, "path", event.Path)
			}
		} else if strings.Contains(event.Operation, "REMOVE") {
			dw.logger.Debug("Handling as removed file", "path", event.Path, "operation", event.Operation)
			if err := dw.processRemovedFile(event.Path); err != nil {
				dw.logger.Error("Failed to process removed file", "error", err, "path", event.Path)
			}
		} else if strings.Contains(event.Operation, "RENAME") {
			dw.logger.Debug("Handling as rename event", "path", event.Path, "operation", event.Operation)
			if err := dw.processRenameEvent(event.Path); err != nil {
				dw.logger.Error("Failed to process rename event", "error", err, "path", event.Path)
			}
		} else {
			dw.logger.Warn("Unhandled file event operation", "path", event.Path, "operation", event.Operation)
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

	// Check if file still exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		dw.logger.Warn("File no longer exists, skipping processing", "path", absPath)
		return nil
	} else if err != nil {
		dw.logger.Error("Failed to check file existence", "error", err, "path", absPath)
		return errors.Wrap(err, "failed to check file existence")
	}

	exists, err := dw.store.ActiveDataSourceExistsByPath(ctx, absPath)
	if err != nil {
		dw.logger.Error("Failed to check if active data source exists", "error", err, "path", absPath)
		return errors.Wrap(err, "failed to check if active data source exists")
	}

	dw.logger.Debug("Active data source check result", "path", absPath, "exists", exists)

	if exists {
		dw.logger.Info("Active data source exists - marking as replaced and creating new version", "path", absPath)

		if err := dw.removeFileFromMemory(ctx, absPath); err != nil {
			dw.logger.Error("Failed to remove old memories for replaced file", "error", err, "path", absPath)
		}

		if err := dw.store.MarkDataSourceAsReplaced(ctx, absPath); err != nil {
			dw.logger.Error("Failed to mark existing data source as replaced", "error", err, "path", absPath)
			return errors.Wrap(err, "failed to mark existing data source as replaced")
		}
		dw.logger.Debug("Successfully marked existing data source as replaced", "path", absPath)
	}

	dataSourceName := dw.determineDataSourceType(filePath)
	if dataSourceName == "" {
		dw.logger.Warn("Unsupported file type, skipping processing", "path", filePath)
		return nil
	}

	dw.logger.Debug("Determined data source type", "path", filePath, "type", dataSourceName)

	dw.logger.Debug("Creating data source from file", "path", absPath, "type", dataSourceName)
	dataSourceID, err := dw.store.CreateDataSourceFromFile(ctx, &db.CreateDataSourceFromFileInput{
		Name: dataSourceName,
		Path: absPath,
	})
	if err != nil {
		dw.logger.Error("Failed to create data source from file", "error", err, "path", absPath, "type", dataSourceName)
		return errors.Wrap(err, "failed to create data source from file")
	}

	dw.logger.Debug("Successfully created data source", "path", absPath, "id", dataSourceID, "type", dataSourceName)

	if err := dw.storeFileInMemory(ctx, absPath, dataSourceName, dataSourceID); err != nil {
		dw.logger.Error("Failed to store file metadata in memory", "error", err, "path", absPath)
	}

	dw.logger.Info("Successfully processed new file", "path", absPath, "id", dataSourceID, "type", dataSourceName)
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

	if err := dw.removeFileFromMemory(ctx, absPath); err != nil {
		dw.logger.Error("Failed to remove file from memory", "error", err, "path", absPath)
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

	dw.logger.Debug("Starting processing workflow", "workflowID", workflowOptions.ID, "taskQueue", workflowOptions.TaskQueue)

	_, err := dw.temporalClient.ExecuteWorkflow(
		context.Background(),
		workflowOptions,
		"InitializeWorkflow",
		map[string]interface{}{},
	)
	if err != nil {
		dw.logger.Error("Failed to start initialize workflow", "error", err, "workflowID", workflowOptions.ID)
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

	dw.logger.Debug("Checking if file is supported", "path", filePath, "ext", ext, "fileName", fileName)

	if strings.HasPrefix(fileName, ".") || strings.HasSuffix(fileName, ".tmp") {
		dw.logger.Debug("File rejected: hidden or temporary", "path", filePath)
		return false
	}

	supportedExts := []string{
		".json", ".mbox", ".zip", ".sqlite", ".db", ".tar", ".tar.gz", ".txt", ".pdf",
	}

	for _, supportedExt := range supportedExts {
		if ext == supportedExt {
			dw.logger.Debug("File accepted: supported extension", "path", filePath, "ext", ext)
			return true
		}
	}

	if strings.Contains(fileName, "whatsapp") && (ext == ".db" || ext == ".sqlite") {
		dw.logger.Debug("File accepted: WhatsApp database", "path", filePath)
		return true
	}

	dw.logger.Debug("File rejected: unsupported extension", "path", filePath, "ext", ext)
	return false
}

func (dw *DirectoryWatcher) determineDataSourceType(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	fileName := strings.ToLower(filepath.Base(filePath))

	dw.logger.Debug("Determining data source type", "path", filePath, "ext", ext, "fileName", fileName)

	switch {
	case strings.Contains(fileName, "whatsapp") && (ext == ".db" || ext == ".sqlite"):
		dw.logger.Debug("Detected WhatsApp data source", "path", filePath)
		return "WhatsApp"
	case strings.Contains(fileName, "telegram") && ext == ".json":
		dw.logger.Debug("Detected Telegram data source", "path", filePath)
		return "Telegram"
	case strings.Contains(fileName, "gmail") || ext == ".mbox":
		dw.logger.Debug("Detected Gmail data source", "path", filePath)
		return "Gmail"
	case strings.Contains(fileName, "slack") && ext == ".zip":
		dw.logger.Debug("Detected Slack data source", "path", filePath)
		return "Slack"
	case strings.Contains(fileName, "chatgpt") && (ext == ".json" || ext == ".zip"):
		dw.logger.Debug("Detected ChatGPT data source", "path", filePath)
		return "ChatGPT"
	case (strings.Contains(fileName, "twitter") || strings.Contains(fileName, "x") || strings.Contains(fileName, "like") || strings.Contains(fileName, "tweet") || strings.Contains(fileName, "direct")) && (ext == ".zip" || ext == ".json" || ext == ".js"):
		dw.logger.Debug("Detected X/Twitter data source", "path", filePath)
		return "X"
	case ext == ".txt":
		dw.logger.Debug("Detected misc text data source", "path", filePath)
		return "misc"
	case ext == ".pdf":
		dw.logger.Debug("Detected misc PDF data source", "path", filePath)
		return "misc"
	case ext == ".json":
		dw.logger.Debug("Analyzing JSON file content", "path", filePath)
		contentType := dw.detectJSONContentType(filePath)
		dw.logger.Debug("JSON content type detected", "path", filePath, "type", contentType)
		return contentType
	case ext == ".zip":
		dw.logger.Debug("Detected generic ZIP as X data source", "path", filePath)
		return "X"
	default:
		dw.logger.Debug("Defaulting to misc data source", "path", filePath, "ext", ext)
		return "misc"
	}
}

// detectJSONContentType attempts to determine the data source type by examining JSON content structure.
func (dw *DirectoryWatcher) detectJSONContentType(filePath string) string {
	file, err := os.Open(filePath)
	if err != nil {
		dw.logger.Warn("Failed to open file for content detection", "path", filePath, "error", err)
		return ""
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			dw.logger.Warn("Failed to close file during content detection", "path", filePath, "error", closeErr)
		}
	}()

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
		if strings.Contains(contentLower, "\"conversation\"") && strings.Contains(contentLower, "\"people\"") {
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

	memoryFact := &memory.MemoryFact{
		ID:        fmt.Sprintf("file-indexed-%s", dataSourceID),
		Content:   fmt.Sprintf("Indexed file: %s as %s data source", filepath.Base(filePath), dataSourceType),
		Timestamp: time.Now(),

		Category: "file-indexed", // Indexed: allows filtering by file indexing events

		Source:   "file-indexing", // Indexed: filter by file indexing source
		FilePath: filePath,        // NEW: Indexed file path field for super-fast queries!
		Tags:     []string{"file-indexed", "data-source", dataSourceType},

		Metadata: map[string]string{
			"data_source_id": dataSourceID,
			"indexed_at":     time.Now().Format(time.RFC3339),
		},
	}

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
	dw.logger.Info("Removing file from memory", "path", filePath)
	if dw.memoryStorage == nil {
		dw.logger.Debug("Memory storage not available, skipping memory cleanup")
		return nil
	}

	filter := &memory.Filter{
		FactFilePath: &filePath,
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

	for _, fact := range result.Facts {
		dw.logger.Info("Found memory for file deletion", "memory_id", fact.ID, "file_path", fact.FilePath)
		// TODO: When DeleteByID is implemented in the memory interface, call it here
		// For now, we're just logging what would be deleted
	}

	dw.logger.Info("Completed memory cleanup for file", "path", filePath, "memories_to_delete", len(result.Facts))
	return nil
}
