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
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory"
	"github.com/EternisAI/enchanted-twin/pkg/db"
)

type DirectoryWatcher struct {
	watcher        *fsnotify.Watcher
	store          *db.Store
	memoryStorage  evolvingmemory.MemoryStorage
	logger         *log.Logger
	temporalClient client.Client

	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	shutdownCh chan struct{}

	fileBuffer     map[string]*FileEvent
	bufferTimer    *time.Timer
	bufferMu       sync.RWMutex
	bufferDuration time.Duration

	processedFiles int64
	errorCount     int64
	statsMu        sync.RWMutex

	supportedExts []string
}

type FileEvent struct {
	Path      string    `json:"path"`
	Operation string    `json:"operation"`
	Timestamp time.Time `json:"timestamp"`
}

const (
	MaxFileSize    = 50 * 1024 * 1024
	DefaultTimeout = 30 * time.Second
	BufferTimeout  = 5 * time.Second
)

func NewDirectoryWatcher(store *db.Store, memoryStorage evolvingmemory.MemoryStorage, logger *log.Logger, temporalClient client.Client) (*DirectoryWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create file watcher")
	}

	dw := &DirectoryWatcher{
		watcher:        watcher,
		store:          store,
		memoryStorage:  memoryStorage,
		logger:         logger,
		temporalClient: temporalClient,
		shutdownCh:     make(chan struct{}),
		fileBuffer:     make(map[string]*FileEvent),
		bufferDuration: BufferTimeout,
		supportedExts: []string{
			".json", ".mbox", ".zip", ".sqlite", ".db", ".tar", ".tar.gz", ".txt", ".pdf",
		},
	}

	return dw, nil
}

func (dw *DirectoryWatcher) Start(ctx context.Context) error {
	dw.ctx, dw.cancel = context.WithCancel(ctx)

	folders, err := dw.store.GetEnabledTrackedFolders(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to load tracked folders")
	}

	successfullyAdded := 0
	for _, folder := range folders {
		if err := os.MkdirAll(folder.Path, 0o755); err != nil {
			dw.logger.Warn("Failed to create watch directory", "dir", folder.Path, "error", err)
			continue
		}

		if err := dw.addDirectoryRecursively(folder.Path); err != nil {
			dw.logger.Error("Failed to add directory recursively", "dir", folder.Path, "error", err)
			continue
		}
		successfullyAdded++
	}

	watchList := dw.watcher.WatchList()
	for i, path := range watchList {
		dw.logger.Info("� Watching", "index", i, "path", path)
	}

	dw.wg.Add(2)
	go dw.eventLoop()
	go dw.performInitialScan(ctx)

	return nil
}

func (dw *DirectoryWatcher) Stop() error {
	dw.logger.Info("Stopping directory watcher")

	if dw.cancel != nil {
		dw.cancel()
	}

	close(dw.shutdownCh)

	dw.bufferMu.Lock()
	if dw.bufferTimer != nil {
		dw.bufferTimer.Stop()
		dw.bufferTimer = nil
	}
	dw.bufferMu.Unlock()

	done := make(chan struct{})
	go func() {
		dw.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		dw.logger.Info("All goroutines stopped cleanly")
	case <-time.After(10 * time.Second):
		dw.logger.Warn("Timeout waiting for goroutines to stop")
	}

	return dw.watcher.Close()
}

func (dw *DirectoryWatcher) eventLoop() {
	defer dw.wg.Done()

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
			dw.incrementErrorCount()

		case <-dw.ctx.Done():
			dw.logger.Info("Event loop context canceled")
			return

		case <-dw.shutdownCh:
			dw.logger.Info("Directory watcher shutting down")
			return
		}
	}
}

func (dw *DirectoryWatcher) handleFileEvent(event fsnotify.Event) {
	if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
		if strings.Contains(event.Op.String(), "CREATE") {
			dw.logger.Info("New directory created, adding to watcher", "path", event.Name)
			if err := dw.watcher.Add(event.Name); err != nil {
				dw.logger.Error("Failed to add new directory to watcher", "error", err, "path", event.Name)
			}
		}
		return
	}

	fileName := filepath.Base(event.Name)
	if strings.HasPrefix(fileName, ".") || strings.HasSuffix(fileName, ".tmp") {
		return
	}

	if !dw.isSupportedFile(event.Name) {
		dw.logger.Info("Skipping unsupported file", "path", event.Name)
		return
	}

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

	processedPaths := make(map[string]bool)
	renamePairs := dw.identifyRenamePairs(events)

	for oldPath, newPath := range renamePairs {
		if err := dw.processRenameEventWithNewPath(oldPath, newPath); err != nil {
			dw.logger.Error("Failed to process rename pair", "error", err, "oldPath", oldPath, "newPath", newPath)
		}

		processedPaths[oldPath] = true
		processedPaths[newPath] = true
	}

	for _, event := range events {
		if strings.Contains(event.Operation, "RENAME") && !processedPaths[event.Path] {
			if err := dw.processRenameEvent(event.Path); err != nil {
				dw.logger.Error("Failed to process rename event", "error", err, "path", event.Path)
			}

			processedPaths[event.Path] = true
			if err := dw.markRelatedEventsAsProcessed(events, event.Path, processedPaths); err != nil {
				dw.logger.Error("Failed to mark related events as processed", "error", err, "renamePath", event.Path)
			}
		}
	}

	for _, event := range events {
		if processedPaths[event.Path] {
			dw.logger.Info("Skipping already processed event", "path", event.Path, "operation", event.Operation)
			continue
		}

		if strings.Contains(event.Operation, "CREATE") || strings.Contains(event.Operation, "WRITE") || strings.Contains(event.Operation, "CHMOD") {
			if err := dw.processNewFile(event.Path); err != nil {
				dw.logger.Error("Failed to process new file", "error", err, "path", event.Path)
			}
		} else if strings.Contains(event.Operation, "REMOVE") {
			if err := dw.processRemovedFile(event.Path); err != nil {
				dw.logger.Error("Failed to process removed file", "error", err, "path", event.Path)
			}
		} else {
			dw.logger.Warn("Unhandled file event operation", "path", event.Path, "operation", event.Operation)
		}
	}

	if err := dw.triggerProcessingWorkflow(); err != nil {
		dw.logger.Error("Failed to trigger processing workflow", "error", err)
	}
}

func (dw *DirectoryWatcher) identifyRenamePairs(events []*FileEvent) map[string]string {
	renamePairs := make(map[string]string)

	var renameEvents []*FileEvent
	for _, event := range events {
		if strings.Contains(event.Operation, "RENAME") {
			renameEvents = append(renameEvents, event)
		}
	}

	for _, renameEvent := range renameEvents {
		renameDir := filepath.Dir(renameEvent.Path)
		renameBase := filepath.Base(renameEvent.Path)
		renameExt := filepath.Ext(renameEvent.Path)

		for _, event := range events {
			if strings.Contains(event.Operation, "CREATE") || strings.Contains(event.Operation, "CHMOD") {
				if dw.couldBeRelatedRenameEvent(event, renameDir, renameBase, renameExt) {
					renamePairs[renameEvent.Path] = event.Path
					break
				}
			}
		}
	}

	return renamePairs
}

func (dw *DirectoryWatcher) markRelatedEventsAsProcessed(events []*FileEvent, renamePath string, processedPaths map[string]bool) error {
	renameDir := filepath.Dir(renamePath)
	renameBase := filepath.Base(renamePath)
	renameExt := filepath.Ext(renamePath)

	for _, event := range events {
		if processedPaths[event.Path] {
			continue
		}

		if dw.couldBeRelatedRenameEvent(event, renameDir, renameBase, renameExt) {
			processedPaths[event.Path] = true
		}
	}

	return nil
}

func (dw *DirectoryWatcher) couldBeRelatedRenameEvent(event *FileEvent, renameDir, renameBase, renameExt string) bool {
	eventDir := filepath.Dir(event.Path)
	eventBase := filepath.Base(event.Path)
	eventExt := filepath.Ext(event.Path)

	if eventDir != renameDir {
		return false
	}

	if eventExt != renameExt {
		return false
	}

	if !strings.Contains(event.Operation, "CREATE") && !strings.Contains(event.Operation, "CHMOD") {
		return false
	}

	if dw.hasCommonPattern(renameBase, eventBase) {
		return true
	}

	return false
}

func (dw *DirectoryWatcher) hasCommonPattern(name1, name2 string) bool {
	base1 := strings.TrimSuffix(name1, filepath.Ext(name1))
	base2 := strings.TrimSuffix(name2, filepath.Ext(name2))

	if strings.HasPrefix(base1, base2) || strings.HasPrefix(base2, base1) {
		return true
	}

	commonPrefix := 0
	minLen := len(base1)
	if len(base2) < minLen {
		minLen = len(base2)
	}

	for i := 0; i < minLen; i++ {
		if base1[i] == base2[i] {
			commonPrefix++
		} else {
			break
		}
	}

	isRelated := commonPrefix >= 5 || float64(commonPrefix)/float64(minLen) >= 0.7
	return isRelated
}

func (dw *DirectoryWatcher) processNewFile(filePath string) error {
	ctx := context.Background()

	ctx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		dw.logger.Error("Failed to resolve absolute path", "error", err, "path", filePath)
		absPath = filePath
	}

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

	if exists {
		dw.logger.Info("Active data source exists - marking as replaced and creating new version", "path", absPath)

		if err := dw.removeFileFromMemory(ctx, absPath); err != nil {
			dw.logger.Error("Failed to remove old memories for replaced file", "error", err, "path", absPath)
		}

		if err := dw.store.MarkDataSourceAsReplaced(ctx, absPath); err != nil {
			dw.logger.Error("Failed to mark existing data source as replaced", "error", err, "path", absPath)
			return errors.Wrap(err, "failed to mark existing data source as replaced")
		}
		dw.logger.Info("Successfully marked existing data source as replaced", "path", absPath)
	}

	dataSourceName := dw.determineDataSourceType(filePath)
	if dataSourceName == "" {
		dw.logger.Warn("Unsupported file type, skipping processing", "path", filePath)
		return nil
	}

	dataSourceID, err := dw.store.CreateDataSourceFromFile(ctx, &db.CreateDataSourceFromFileInput{
		Name: dataSourceName,
		Path: absPath,
	})
	if err != nil {
		dw.logger.Error("Failed to create data source from file", "error", err, "path", absPath, "type", dataSourceName)
		return errors.Wrap(err, "failed to create data source from file")
	}

	dw.logger.Info("Successfully created data source", "path", absPath, "id", dataSourceID, "type", dataSourceName)

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

	dw.logger.Info("Processing removed file", "originalPath", filePath, "absolutePath", absPath)

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

func (dw *DirectoryWatcher) findNewPathForRenameEvent(oldPath string) string {
	oldDir := filepath.Dir(oldPath)
	oldExt := filepath.Ext(oldPath)
	oldBase := filepath.Base(oldPath)

	entries, err := os.ReadDir(oldDir)
	if err != nil {
		dw.logger.Error("Failed to read directory for rename detection", "error", err, "dir", oldDir)
		return ""
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		newPath := filepath.Join(oldDir, entry.Name())
		newExt := filepath.Ext(entry.Name())
		newBase := entry.Name()

		if newExt != oldExt {
			continue
		}

		if newPath == oldPath {
			continue
		}

		if dw.hasCommonPattern(oldBase, newBase) {
			info, err := entry.Info()
			if err != nil {
				continue
			}

			if time.Since(info.ModTime()) < 30*time.Second {
				dw.logger.Info("Found potential new path for renamed file", "oldPath", oldPath, "newPath", newPath)
				return newPath
			}
		}
	}

	return ""
}

func (dw *DirectoryWatcher) processRenameEvent(filePath string) error {
	ctx := context.Background()

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		dw.logger.Error("Failed to resolve absolute path for rename event", "error", err, "path", filePath)
		absPath = filePath
	}

	_, err = os.Stat(absPath)
	if os.IsNotExist(err) {
		newPath := dw.findNewPathForRenameEvent(absPath)
		if newPath != "" {
			return dw.processFileRename(absPath, newPath)
		}

		dw.logger.Info("Could not find new path for renamed file - treating as deletion", "oldPath", absPath)
		return dw.processRemovedFile(absPath)
	} else if err != nil {
		dw.logger.Error("Failed to check file existence for RENAME event", "error", err, "path", absPath)
		return err
	}

	exists, err := dw.store.ActiveDataSourceExistsByPath(ctx, absPath)
	if err != nil {
		dw.logger.Error("Failed to check if data source exists for new path", "error", err, "path", absPath)
	} else if exists {
		dw.logger.Info("Data source already exists for new path - treating as replacement", "path", absPath)
		return dw.processNewFile(absPath)
	}

	orphanedSources, err := dw.store.FindOrphanedDataSources(ctx)
	if err != nil {
		dw.logger.Error("Failed to find orphaned data sources", "error", err)
		return dw.processNewFile(absPath)
	}

	var matchingSource *db.DataSource
	for _, source := range orphanedSources {
		if _, err := os.Stat(source.Path); !os.IsNotExist(err) {
			continue
		}

		if dw.couldBeRenamedFile(source.Path, absPath, source.Name) {
			matchingSource = source
			break
		}
	}

	if matchingSource != nil {
		return dw.processFileRename(matchingSource.Path, absPath)
	}

	dw.logger.Info("No matching orphaned data source found - treating as new file", "path", absPath)
	return dw.processNewFile(absPath)
}

func (dw *DirectoryWatcher) processRenameEventWithNewPath(oldPath, newPath string) error {
	ctx := context.Background()

	oldAbsPath, err := filepath.Abs(oldPath)
	if err != nil {
		dw.logger.Error("Failed to resolve absolute path for old file", "error", err, "path", oldPath)
		oldAbsPath = oldPath
	}

	newAbsPath, err := filepath.Abs(newPath)
	if err != nil {
		dw.logger.Error("Failed to resolve absolute path for new file", "error", err, "path", newPath)
		newAbsPath = newPath
	}

	if _, err := os.Stat(newAbsPath); os.IsNotExist(err) {
		dw.logger.Warn("New file path does not exist, treating as deletion", "newPath", newAbsPath)
		return dw.processRemovedFile(oldAbsPath)
	}

	exists, err := dw.store.ActiveDataSourceExistsByPath(ctx, newAbsPath)
	if err != nil {
		dw.logger.Error("Failed to check if data source exists for new path", "error", err, "path", newAbsPath)
	} else if exists {
		dw.logger.Info("Data source already exists for new path - treating as replacement", "path", newAbsPath)
		if err := dw.processRemovedFile(oldAbsPath); err != nil {
			dw.logger.Error("Failed to process old file as removed", "error", err, "oldPath", oldAbsPath)
		}
		return dw.processNewFile(newAbsPath)
	}

	orphanedSources, err := dw.store.FindOrphanedDataSources(ctx)
	if err != nil {
		dw.logger.Error("Failed to find orphaned data sources for rename", "error", err)
		return dw.processNewFile(newAbsPath)
	}

	var matchingSource *db.DataSource
	for _, source := range orphanedSources {
		if source.Path == oldAbsPath {
			matchingSource = source
			break
		}
	}

	if matchingSource == nil {
		dw.logger.Info("No data source found for old path, treating as new file", "oldPath", oldAbsPath)
		return dw.processNewFile(newAbsPath)
	}

	newDataSourceType := dw.determineDataSourceType(newAbsPath)
	if newDataSourceType != matchingSource.Name {
		dw.logger.Warn("Data source type mismatch, treating as replacement", "oldType", matchingSource.Name, "newType", newDataSourceType)
		if err := dw.processRemovedFile(oldAbsPath); err != nil {
			dw.logger.Error("Failed to process old file as removed", "error", err, "oldPath", oldAbsPath)
		}
		return dw.processNewFile(newAbsPath)
	}

	if err := dw.store.UpdateDataSourcePath(ctx, matchingSource.ID, newAbsPath); err != nil {
		dw.logger.Error("Failed to update data source path", "error", err, "dataSourceID", matchingSource.ID)
		return dw.processNewFile(newAbsPath)
	}

	if err := dw.updateFilePathInMemory(ctx, oldAbsPath, newAbsPath); err != nil {
		dw.logger.Error("Failed to update file path in memory", "error", err, "oldPath", oldAbsPath, "newPath", newAbsPath)
	}

	return nil
}

func (dw *DirectoryWatcher) processFileRename(oldPath, newPath string) error {
	ctx := context.Background()

	orphanedSources, err := dw.store.FindOrphanedDataSources(ctx)
	if err != nil {
		dw.logger.Error("Failed to find data sources for rename", "error", err)
		return err
	}

	var dataSourceID string
	for _, source := range orphanedSources {
		if source.Path == oldPath {
			dataSourceID = source.ID
			break
		}
	}

	if dataSourceID == "" {
		dw.logger.Error("Could not find data source for old path", "oldPath", oldPath)
		return dw.processNewFile(newPath)
	}

	if err := dw.store.UpdateDataSourcePath(ctx, dataSourceID, newPath); err != nil {
		dw.logger.Error("Failed to update data source path", "error", err, "dataSourceID", dataSourceID)
		return dw.processNewFile(newPath)
	}

	if err := dw.updateFilePathInMemory(ctx, oldPath, newPath); err != nil {
		dw.logger.Error("Failed to update file path in memory", "error", err, "oldPath", oldPath, "newPath", newPath)
	}

	return nil
}

func (dw *DirectoryWatcher) couldBeRenamedFile(oldPath, newPath, dataSourceType string) bool {
	oldExt := strings.ToLower(filepath.Ext(oldPath))
	newExt := strings.ToLower(filepath.Ext(newPath))

	if oldExt != newExt {
		return false
	}

	newDataSourceType := dw.determineDataSourceType(newPath)
	if newDataSourceType != dataSourceType {
		return false
	}

	dw.logger.Info("File could be renamed version", "oldPath", oldPath, "newPath", newPath, "dataSourceType", dataSourceType)
	return true
}

func (dw *DirectoryWatcher) triggerProcessingWorkflow() error {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()

	workflowOptions := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("sync-dataprocessing-%d", time.Now().Unix()),
		TaskQueue: "default",
	}

	_, err := dw.temporalClient.ExecuteWorkflow(
		ctx,
		workflowOptions,
		"InitializeWorkflow",
		map[string]interface{}{},
	)
	if err != nil {
		dw.logger.Error("Failed to start initialize workflow", "error", err, "workflowID", workflowOptions.ID)
		return errors.Wrap(err, "failed to start initialize workflow")
	}

	return nil
}

func (dw *DirectoryWatcher) performInitialScan(ctx context.Context) {
	defer dw.wg.Done()

	scanCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	dw.logger.Info("🚀 Starting initial scan of watched directories")

	folders, err := dw.store.GetEnabledTrackedFolders(scanCtx)
	if err != nil {
		dw.logger.Error("❌ Failed to load tracked folders for initial scan", "error", err)
		return
	}

	if len(folders) == 0 {
		dw.logger.Warn("⚠️ No folders to scan - initial scan complete")
		return
	}

	totalProcessed := 0
	for folderIndex, folder := range folders {
		dw.logger.Info("🔍 Scanning directory", "folderIndex", folderIndex+1, "totalFolders", len(folders), "dir", folder.Path)

		folderProcessed := 0
		err := filepath.Walk(folder.Path, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			select {
			case <-scanCtx.Done():
				return scanCtx.Err()
			default:
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
				absPath = path
			}

			exists, err := dw.store.ActiveDataSourceExistsByPath(scanCtx, absPath)
			if err != nil {
				return nil
			}

			if !exists {
				if err := dw.processNewFile(path); err != nil {
					dw.logger.Error("Failed to process file during initial scan", "error", err, "path", path)
				} else {
					dw.incrementProcessedFiles()
					folderProcessed++
					totalProcessed++
				}
			}

			return nil
		})

		if err != nil {
			if err == context.Canceled {
				dw.logger.Info("Initial scan canceled", "dir", folder.Path)
			} else {
				dw.logger.Error("Error during initial scan", "error", err, "dir", folder.Path)
				dw.incrementErrorCount()
			}
		} else {
			dw.logger.Info("✅ Initial scan completed for directory", "dir", folder.Path, "filesProcessed", folderProcessed)
		}
	}

	dw.logger.Info("All directories scanned")

	if err := dw.triggerProcessingWorkflow(); err != nil {
		dw.logger.Error("❌ Failed to trigger processing workflow after initial scan", "error", err)
	} else {
		dw.logger.Info("✅ Processing workflow triggered successfully")
	}
}

func (dw *DirectoryWatcher) isSupportedFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	fileName := strings.ToLower(filepath.Base(filePath))

	if strings.HasPrefix(fileName, ".") || strings.HasSuffix(fileName, ".tmp") {
		return false
	}

	for _, supportedExt := range dw.supportedExts {
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
		contentType := dw.detectJSONContentType(filePath)
		return contentType
	case ext == ".zip":
		return "X"
	default:
		return "misc"
	}
}

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

	if strings.Contains(contentLower, "\"like\"") && strings.Contains(contentLower, "\"tweetid\"") ||
		strings.Contains(contentLower, "\"tweet\"") && strings.Contains(contentLower, "\"full_text\"") ||
		strings.Contains(contentLower, "\"dmconversation\"") ||
		strings.Contains(contentLower, "\"conversationid\"") && strings.Contains(contentLower, "\"senderid\"") {
		return "X"
	}

	if strings.Contains(contentLower, "\"personal_information\"") && strings.Contains(contentLower, "\"chats\"") ||
		strings.Contains(contentLower, "\"date_unixtime\"") && strings.Contains(contentLower, "\"from_id\"") {
		return "Telegram"
	}

	if strings.Contains(contentLower, "\"conversations\"") && strings.Contains(contentLower, "\"mapping\"") ||
		strings.Contains(contentLower, "\"message\"") && strings.Contains(contentLower, "\"author\"") && strings.Contains(contentLower, "\"role\"") {
		return "ChatGPT"
	}

	trimmedContent := strings.TrimSpace(content)
	if strings.HasPrefix(trimmedContent, "[") {
		if strings.Contains(contentLower, "\"conversation\"") && strings.Contains(contentLower, "\"people\"") {
			return "X"
		}
		return "X"
	}

	dw.logger.Warn("Could not determine data source type for JSON file", "path", filePath)
	return ""
}

func (dw *DirectoryWatcher) storeFileInMemory(ctx context.Context, filePath, dataSourceType string, dataSourceID string) error {
	if dw.memoryStorage == nil {
		return nil
	}

	fileContent, err := dw.readFileContent(filePath)
	if err != nil {
		dw.logger.Error("Failed to read file content for memory storage", "error", err, "path", filePath)
		return errors.Wrap(err, "failed to read file content")
	}

	memoryFact := &memory.MemoryFact{
		ID:        fmt.Sprintf("file-indexed-%s", dataSourceID),
		Content:   fileContent,
		Timestamp: time.Now(),
		Category:  "file-indexed",
		Source:    "file-indexing",
		FilePath:  filePath,
		Tags:      []string{"file-indexed", "data-source", dataSourceType},

		Metadata: map[string]string{
			"data_source_id":   dataSourceID,
			"indexed_at":       time.Now().Format(time.RFC3339),
			"file_name":        filepath.Base(filePath),
			"data_source_type": dataSourceType,
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

	err = dw.memoryStorage.Store(ctx, documents, func(processed, total int) {
	})
	if err != nil {
		return errors.Wrap(err, "failed to store file metadata in memory")
	}

	dw.logger.Info("Stored file content in memory using indexed FilePath field", "path", filePath, "category", memoryFact.Category, "file_path", memoryFact.FilePath, "content_length", len(fileContent))
	return nil
}

func (dw *DirectoryWatcher) removeFileFromMemory(ctx context.Context, filePath string) error {
	dw.logger.Info("Removing file from memory", "path", filePath)
	if dw.memoryStorage == nil {
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
		return nil
	}

	dw.logger.Info("Found memories to delete for file", "path", filePath, "count", len(result.Facts))

	for _, fact := range result.Facts {
		dw.logger.Info("Deleting memory for removed file", "memory_id", fact.ID, "file_path", fact.FilePath)

		if err := dw.memoryStorage.Delete(ctx, fact.ID); err != nil {
			dw.logger.Error("Failed to delete memory", "error", err, "memory_id", fact.ID)
			continue
		}

		dw.logger.Info("Successfully deleted memory", "memory_id", fact.ID)
	}

	dw.logger.Info("Completed memory cleanup for file", "path", filePath, "memories_to_delete", len(result.Facts))
	return nil
}

func (dw *DirectoryWatcher) updateFilePathInMemory(ctx context.Context, oldPath, newPath string) error {
	dw.logger.Info("Updating file path in memory", "oldPath", oldPath, "newPath", newPath)
	if dw.memoryStorage == nil {
		return nil
	}

	filter := &memory.Filter{
		FactFilePath: &oldPath,
		Limit:        &[]int{100}[0],
	}

	result, err := dw.memoryStorage.Query(ctx, "", filter)
	if err != nil {
		return errors.Wrap(err, "failed to query memories for path update")
	}

	if len(result.Facts) == 0 {
		dw.logger.Info("No memories found for old file path", "oldPath", oldPath)
		return nil
	}

	dw.logger.Info("Found memories to update for file path change", "oldPath", oldPath, "newPath", newPath, "count", len(result.Facts))

	for _, fact := range result.Facts {
		dw.logger.Info("Updating memory file path", "memory_id", fact.ID, "oldPath", oldPath, "newPath", newPath)

		fact.FilePath = newPath
		if fact.Metadata == nil {
			fact.Metadata = make(map[string]string)
		}
		fact.Metadata["file_name"] = filepath.Base(newPath)

		fileDoc := &memory.TextDocument{
			FieldID:        fact.ID,
			FieldContent:   fact.Content,
			FieldTimestamp: &fact.Timestamp,
			FieldSource:    fact.Source,
			FieldTags:      fact.Tags,
			FieldMetadata:  fact.Metadata,
			FieldFilePath:  fact.FilePath,
		}

		documents := []memory.Document{fileDoc}

		err = dw.memoryStorage.Store(ctx, documents, func(processed, total int) {
			dw.logger.Info("Updating file path in memory", "processed", processed, "total", total)
		})
		if err != nil {
			dw.logger.Error("Failed to update memory file path", "error", err, "memory_id", fact.ID)
			continue
		}

		dw.logger.Info("Successfully updated memory file path", "memory_id", fact.ID, "newPath", newPath)
	}

	dw.logger.Info("Completed memory path update", "oldPath", oldPath, "newPath", newPath, "memories_updated", len(result.Facts))
	return nil
}

func (dw *DirectoryWatcher) readFileContent(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", errors.Wrap(err, "failed to open file")
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			dw.logger.Warn("Failed to close file during content reading", "path", filePath, "error", closeErr)
		}
	}()

	fileInfo, err := file.Stat()
	if err != nil {
		return "", errors.Wrap(err, "failed to get file info")
	}

	if fileInfo.Size() > MaxFileSize {
		dw.logger.Warn("File too large, reading first portion only", "path", filePath, "size", fileInfo.Size(), "maxSize", MaxFileSize)
		buffer := make([]byte, MaxFileSize)
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			return "", errors.Wrap(err, "failed to read file content")
		}
		return string(buffer[:n]), nil
	}

	content, err := io.ReadAll(file)
	if err != nil {
		return "", errors.Wrap(err, "failed to read file content")
	}

	return string(content), nil
}

// Add helper methods for metrics.
func (dw *DirectoryWatcher) incrementProcessedFiles() {
	dw.statsMu.Lock()
	defer dw.statsMu.Unlock()
	dw.processedFiles++
}

func (dw *DirectoryWatcher) incrementErrorCount() {
	dw.statsMu.Lock()
	defer dw.statsMu.Unlock()
	dw.errorCount++
}

// addDirectoryRecursively adds a directory and all its subdirectories to the watcher.
func (dw *DirectoryWatcher) addDirectoryRecursively(rootPath string) error {
	dirsAdded := 0
	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			dw.logger.Warn("Failed to access path during recursive watch setup", "path", path, "error", err)
			return nil
		}

		if info.IsDir() {
			if err := dw.watcher.Add(path); err != nil {
				dw.logger.Warn("Failed to add subdirectory to watcher", "path", path, "error", err)
				return nil
			}
			dw.logger.Info("Added subdirectory to watcher", "path", path)
			dirsAdded++
		}

		return nil
	})
	if err != nil {
		return err
	}

	dw.logger.Info("Recursively added directories to watcher", "rootPath", rootPath, "totalDirsAdded", dirsAdded)
	return nil
}

// AddWatchedDirectory adds a new directory to watch recursively.
func (dw *DirectoryWatcher) AddWatchedDirectory(path string) error {
	watchList := dw.watcher.WatchList()
	for _, watchedPath := range watchList {
		if watchedPath == path {
			return fmt.Errorf("directory already being watched: %s", path)
		}
	}

	if err := os.MkdirAll(path, 0o755); err != nil {
		return errors.Wrap(err, "failed to create directory")
	}

	if err := dw.addDirectoryRecursively(path); err != nil {
		return errors.Wrap(err, "failed to add directory recursively")
	}

	dw.logger.Info("Added directory to watcher recursively", "path", path)

	go dw.performInitialScanForDirectory(path)

	return nil
}

// performInitialScanForDirectory scans a specific directory for existing files.
func (dw *DirectoryWatcher) performInitialScanForDirectory(dirPath string) {
	ctx := context.Background()
	scanCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	dw.logger.Info("🔍 Starting initial scan for newly added directory", "dir", dirPath)

	filesProcessed := 0
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		select {
		case <-scanCtx.Done():
			return scanCtx.Err()
		default:
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
			absPath = path
		}

		exists, err := dw.store.ActiveDataSourceExistsByPath(scanCtx, absPath)
		if err != nil {
			return nil
		}

		if !exists {
			if err := dw.processNewFile(path); err != nil {
				dw.logger.Error("Failed to process file during directory scan", "error", err, "path", path)
			} else {
				dw.incrementProcessedFiles()
				filesProcessed++
			}
		}

		return nil
	})

	if err != nil {
		if err == context.Canceled {
			dw.logger.Info("Directory scan canceled", "dir", dirPath)
		} else {
			dw.logger.Error("Error during directory scan", "error", err, "dir", dirPath)
			dw.incrementErrorCount()
		}
	} else {
		dw.logger.Info("✅ Initial scan completed for newly added directory", "dir", dirPath, "filesProcessed", filesProcessed)
	}

	if filesProcessed > 0 {
		if err := dw.triggerProcessingWorkflow(); err != nil {
			dw.logger.Error("❌ Failed to trigger processing workflow after directory scan", "error", err)
		} else {
			dw.logger.Info("✅ Processing workflow triggered after directory scan", "dir", dirPath)
		}
	}
}

// RemoveWatchedDirectory removes a directory from watching.
func (dw *DirectoryWatcher) RemoveWatchedDirectory(path string) error {
	// Remove from watcher
	if err := dw.watcher.Remove(path); err != nil {
		dw.logger.Warn("Failed to remove directory from watcher", "path", path, "error", err)
	}

	dw.logger.Info("Removed directory from watcher", "path", path)
	return nil
}

// ReloadTrackedFolders reloads the tracked folders from the database.
func (dw *DirectoryWatcher) ReloadTrackedFolders(ctx context.Context) error {
	dw.logger.Info("🔄 ReloadTrackedFolders called - syncing with database...")

	// Get current enabled folders from database
	folders, err := dw.store.GetEnabledTrackedFolders(ctx)
	if err != nil {
		dw.logger.Error("❌ Failed to load tracked folders from database", "error", err)
		return err
	}

	dw.logger.Info("📊 Database query completed", "folders_found", len(folders))
	for i, folder := range folders {
		dw.logger.Info("📋 Folder from DB", "index", i, "path", folder.Path, "name", folder.Name, "enabled", folder.IsEnabled)
	}

	currentWatchList := dw.watcher.WatchList()
	dw.logger.Info("👀 Current fsnotify watch list", "count", len(currentWatchList))
	for i, path := range currentWatchList {
		dw.logger.Info("📍 Currently watching", "index", i, "path", path)
	}

	currentWatched := make(map[string]bool)
	for _, path := range currentWatchList {
		currentWatched[path] = true
	}

	desiredDirs := make(map[string]bool)
	for _, folder := range folders {
		desiredDirs[folder.Path] = true
	}

	removedCount := 0
	for watchedPath := range currentWatched {
		if !desiredDirs[watchedPath] {
			dw.logger.Info("🗑️ Removing directory from watcher", "path", watchedPath)
			if err := dw.RemoveWatchedDirectory(watchedPath); err != nil {
				dw.logger.Error("❌ Failed to remove directory", "path", watchedPath, "error", err)
			} else {
				removedCount++
			}
		}
	}

	addedCount := 0
	for _, folder := range folders {
		if !currentWatched[folder.Path] {
			dw.logger.Info("➕ Adding new directory to watcher", "path", folder.Path)
			if err := dw.AddWatchedDirectory(folder.Path); err != nil {
				dw.logger.Error("❌ Failed to add directory", "path", folder.Path, "error", err)
			} else {
				addedCount++
			}
		}
	}

	finalWatchList := dw.watcher.WatchList()
	dw.logger.Info("📈 Reload summary",
		"total_folders", len(folders),
		"removed_count", removedCount,
		"added_count", addedCount,
		"final_watch_count", len(finalWatchList))

	return nil
}

// GetWatchedDirectories returns the list of directories currently being watched.
func (dw *DirectoryWatcher) GetWatchedDirectories() []string {
	if dw.watcher == nil {
		dw.logger.Warn("⚠️ fsnotify watcher is nil")
		return []string{}
	}

	watchList := dw.watcher.WatchList()
	dw.logger.Info("📋 Current watched directories", "count", len(watchList))
	for i, path := range watchList {
		dw.logger.Info("📂 Watched directory", "index", i, "path", path)
	}

	return watchList
}

// GetTrackedFoldersFromDB returns the tracked folders from the database for debugging.
func (dw *DirectoryWatcher) GetTrackedFoldersFromDB(ctx context.Context) error {
	folders, err := dw.store.GetEnabledTrackedFolders(ctx)
	if err != nil {
		dw.logger.Error("❌ Failed to get tracked folders from DB", "error", err)
		return err
	}

	dw.logger.Info("📊 Tracked folders from database", "count", len(folders))
	for i, folder := range folders {
		dw.logger.Info("📋 DB folder", "index", i, "id", folder.ID, "path", folder.Path, "name", folder.Name, "enabled", folder.IsEnabled)
	}

	return nil
}
