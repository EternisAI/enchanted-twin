// Owner: slimane@eternis.ai
package dataprocessing

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/log"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/chatgpt"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/constants"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/gmail"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/misc"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/slack"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/telegram"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/whatsapp"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/x"
	"github.com/EternisAI/enchanted-twin/pkg/db"
)

// DocumentProcessor represents the new clean interface - each source implements this
// to convert raw input directly to ConversationDocument, eliminating the lossy types.Record step.
type DocumentProcessor interface {
	// File processing (required for all processors)
	ProcessFile(ctx context.Context, filepath string) ([]memory.ConversationDocument, error)
}

// LiveSync is an optional interface that processors can implement
// if they support syncing from live APIs.
type LiveSync interface {
	Sync(ctx context.Context, accessToken string) ([]memory.ConversationDocument, error)
}

// DateRangeSync is an optional interface that processors can implement
// if they support syncing within specific date ranges.
type DateRangeSync interface {
	SyncWithDateRange(ctx context.Context, accessToken string, startDate, endDate time.Time) ([]memory.ConversationDocument, error)
}

func validateInputPath(inputPath string) error {
	cleanPath := filepath.Clean(inputPath)

	info, err := os.Stat(cleanPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("input path does not exist: %s", cleanPath)
	}
	if err != nil {
		return fmt.Errorf("error accessing input path: %v", err)
	}

	_ = info
	return nil
}

func validateOutputPath(outputPath string) error {
	cleanPath := filepath.Clean(outputPath)

	dir := filepath.Dir(cleanPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("cannot create output directory: %v", err)
	}

	return nil
}

func extractZip(zipPath string) (extractedPath string, err error) {
	tempDir, err := os.MkdirTemp("", "extracted_zip_")
	if err != nil {
		return "", fmt.Errorf("error creating temp directory: %v", err)
	}

	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		err = os.RemoveAll(tempDir)
		if err != nil {
			return "", fmt.Errorf("error removing temp directory: %v", err)
		}
		return "", fmt.Errorf("error opening zip file: %v", err)
	}
	defer func() {
		if closeErr := reader.Close(); closeErr != nil {
			if err == nil {
				err = fmt.Errorf("error closing zip reader: %v", closeErr)
			} else {
				log.Printf("Error closing zip reader: %v", closeErr)
			}
		}
	}()

	for _, file := range reader.File {
		path := filepath.Join(tempDir, file.Name)

		if file.FileInfo().IsDir() {
			err = os.MkdirAll(path, file.Mode())
			if err != nil {
				return "", fmt.Errorf("error creating directory: %v", err)
			}
			continue
		}

		fileReader, err := file.Open()
		if err != nil {
			err = os.RemoveAll(tempDir)
			if err != nil {
				return "", fmt.Errorf("error removing temp directory: %v", err)
			}
			return "", fmt.Errorf("error opening file in zip: %v", err)
		}
		defer func() {
			if closeErr := fileReader.Close(); closeErr != nil {
				if err == nil {
					err = fmt.Errorf("error closing file reader: %v", closeErr)
				} else {
					log.Printf("Error closing file reader: %v", closeErr)
				}
			}
		}()

		err = os.MkdirAll(filepath.Dir(path), 0o755)
		if err != nil {
			return "", fmt.Errorf("error creating directory: %v", err)
		}

		targetFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			err = os.RemoveAll(tempDir)
			if err != nil {
				return "", fmt.Errorf("error removing temp directory: %v", err)
			}
			return "", fmt.Errorf("error creating file: %v", err)
		}
		defer func() {
			if closeErr := targetFile.Close(); closeErr != nil {
				if err == nil {
					err = fmt.Errorf("error closing target file: %v", closeErr)
				} else {
					log.Printf("Error closing target file: %v", closeErr)
				}
			}
		}()

		if _, err := io.Copy(targetFile, fileReader); err != nil {
			err = os.RemoveAll(tempDir)
			if err != nil {
				return "", fmt.Errorf("error removing temp directory: %v", err)
			}
			return "", fmt.Errorf("error extracting file: %v", err)
		}
	}

	return tempDir, nil
}

func extractTarGz(tarGzPath string) (extractedPath string, err error) {
	tempDir, err := os.MkdirTemp("", "extracted_tar_")
	if err != nil {
		return "", fmt.Errorf("error creating temp directory: %v", err)
	}

	file, err := os.Open(tarGzPath)
	if err != nil {
		err = os.RemoveAll(tempDir)
		if err != nil {
			return "", fmt.Errorf("error removing temp directory: %v", err)
		}
		return "", fmt.Errorf("error opening tar.gz file: %v", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			if err == nil {
				err = fmt.Errorf("error closing tar.gz file: %v", closeErr)
			} else {
				log.Printf("Error closing tar.gz file: %v", closeErr)
			}
		}
	}()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		err = os.RemoveAll(tempDir)
		if err != nil {
			return "", fmt.Errorf("error removing temp directory: %v", err)
		}
		return "", fmt.Errorf("error creating gzip reader: %v", err)
	}
	defer func() {
		if closeErr := gzReader.Close(); closeErr != nil {
			if err == nil {
				err = fmt.Errorf("error closing gzip reader: %v", closeErr)
			} else {
				log.Printf("Error closing gzip reader: %v", closeErr)
			}
		}
	}()

	tarReader := tar.NewReader(gzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			err = os.RemoveAll(tempDir)
			if err != nil {
				return "", fmt.Errorf("error removing temp directory: %v", err)
			}
			return "", fmt.Errorf("error reading tar header: %v", err)
		}

		path := filepath.Join(tempDir, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, header.FileInfo().Mode()); err != nil {
				err = os.RemoveAll(tempDir)
				if err != nil {
					return "", fmt.Errorf("error removing temp directory: %v", err)
				}
				return "", fmt.Errorf("error creating directory: %v", err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				err = os.RemoveAll(tempDir)
				if err != nil {
					return "", fmt.Errorf("error removing temp directory: %v", err)
				}
				return "", fmt.Errorf("error creating directory: %v", err)
			}

			outFile, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, header.FileInfo().Mode())
			if err != nil {
				err = os.RemoveAll(tempDir)
				if err != nil {
					return "", fmt.Errorf("error removing temp directory: %v", err)
				}
				return "", fmt.Errorf("error creating file: %v", err)
			}

			if _, err := io.Copy(outFile, tarReader); err != nil {
				if errClose := outFile.Close(); errClose != nil {
					log.Printf("Error closing file: %v", errClose)
				}
				err = os.RemoveAll(tempDir)
				if err != nil {
					return "", fmt.Errorf("error removing temp directory: %v", err)
				}
				return "", fmt.Errorf("error extracting file: %v", err)
			}

			if err := outFile.Close(); err != nil {
				err = os.RemoveAll(tempDir)
				if err != nil {
					return "", fmt.Errorf("error removing temp directory: %v", err)
				}
				return "", fmt.Errorf("error closing file: %v", err)
			}
		}
	}

	return tempDir, nil
}

type DataProcessingService struct {
	openAiService    *ai.Service
	completionsModel string
	store            *db.Store
	memory           memory.Storage
	logger           *log.Logger
}

func NewDataProcessingService(openAiService *ai.Service, completionsModel string, store *db.Store, memoryStorage memory.Storage, logger *log.Logger) *DataProcessingService {
	return &DataProcessingService{
		openAiService:    openAiService,
		completionsModel: completionsModel,
		store:            store,
		memory:           memoryStorage,
		logger:           logger,
	}
}

func (s *DataProcessingService) ProcessSource(ctx context.Context, sourceType string, inputPath string, outputPath string) (bool, error) {
	if err := validateInputPath(inputPath); err != nil {
		return false, fmt.Errorf("invalid input path: %v", err)
	}

	if err := validateOutputPath(outputPath); err != nil {
		return false, fmt.Errorf("invalid output path: %v", err)
	}

	var err error

	ext := strings.ToLower(filepath.Ext(inputPath))
	if ext == ".zip" || ext == ".tar" || ext == ".tar.gz" {
		var tempDir string
		if ext == ".zip" {
			tempDir, err = extractZip(inputPath)
		} else {
			tempDir, err = extractTarGz(inputPath)
		}
		if err != nil {
			return false, fmt.Errorf("error extracting archive file: %v", err)
		}
		defer func() {
			err = os.RemoveAll(tempDir)
			if err != nil {
				log.Printf("Error removing temp directory: %v", err)
			}
		}()

		inputPath = tempDir
	}

	switch constants.ProcessorType(strings.ToLower(sourceType)) {
	case constants.ProcessorTelegram:
		s.logger.Info("ProcessSource: Processing Telegram data source",
			"inputPath", inputPath,
			"outputPath", outputPath)

		processor, err := telegram.NewTelegramProcessor(s.store, s.logger)
		if err != nil {
			return false, err
		}

		documents, err := processor.ProcessFile(ctx, inputPath)
		if err != nil {
			return false, err
		}

		if outputPath == "" {
			s.logger.Info("ProcessSource: Storing Telegram documents directly in memory",
				"documentCount", len(documents))

			var memoryDocs []memory.Document
			for _, doc := range documents {
				docCopy := doc
				memoryDocs = append(memoryDocs, &docCopy)
			}

			progressCallback := func(processed, total int) {
				s.logger.Info("Processing documents", "processed", processed, "total", total)
			}

			if err := s.memory.Store(ctx, memoryDocs, progressCallback); err != nil {
				return false, fmt.Errorf("failed to store documents: %w", err)
			}

			s.logger.Info("Successfully processed and stored documents", "count", len(documents))
			return true, nil
		} else {
			if err := memory.ExportConversationDocumentsJSON(documents, outputPath); err != nil {
				return false, err
			}
			return true, nil
		}
	case constants.ProcessorSlack:
		s.logger.Info("ProcessSource: Processing Slack data source",
			"inputPath", inputPath,
			"outputPath", outputPath)

		processor, err := slack.NewSlackProcessor(s.store, s.logger)
		if err != nil {
			return false, err
		}

		documents, err := processor.ProcessDirectory(ctx, inputPath)
		if err != nil {
			return false, err
		}

		if outputPath == "" {
			s.logger.Info("ProcessSource: Storing Slack documents directly in memory",
				"documentCount", len(documents))

			var memoryDocs []memory.Document
			for _, doc := range documents {
				docCopy := doc
				memoryDocs = append(memoryDocs, &docCopy)
			}

			progressCallback := func(processed, total int) {
				s.logger.Info("Processing documents", "processed", processed, "total", total)
			}

			if err := s.memory.Store(ctx, memoryDocs, progressCallback); err != nil {
				return false, fmt.Errorf("failed to store documents: %w", err)
			}

			s.logger.Info("Successfully processed and stored documents", "count", len(documents))
			return true, nil
		} else {
			if err := memory.ExportConversationDocumentsJSON(documents, outputPath); err != nil {
				return false, err
			}
			return true, nil
		}
	case constants.ProcessorGmail:
		processor, err := gmail.NewGmailProcessor(s.store, s.logger)
		if err != nil {
			return false, err
		}
		documents, err := processor.ProcessFile(ctx, inputPath)
		if err != nil {
			return false, err
		}

		// Check if outputPath is empty (direct memory storage)
		if outputPath == "" {
			// Store directly in memory
			var memoryDocs []memory.Document
			for _, doc := range documents {
				docCopy := doc
				memoryDocs = append(memoryDocs, &docCopy)
			}

			progressCallback := func(processed, total int) {
				s.logger.Info("Processing documents", "processed", processed, "total", total)
			}

			if err := s.memory.Store(ctx, memoryDocs, progressCallback); err != nil {
				return false, fmt.Errorf("failed to store documents: %w", err)
			}

			s.logger.Info("Successfully processed and stored documents", "count", len(documents))
			return true, nil
		} else {
			// Save to file for backward compatibility
			if err := memory.ExportConversationDocumentsJSON(documents, outputPath); err != nil {
				return false, err
			}
			return true, nil
		}
	case constants.ProcessorX:
		s.logger.Info("ProcessSource: Processing X/Twitter data source",
			"inputPath", inputPath,
			"outputPath", outputPath)

		processor, err := x.NewXProcessor(s.store, s.logger)
		if err != nil {
			return false, err
		}

		documents, err := processor.ProcessDirectory(ctx, inputPath)
		if err != nil {
			return false, err
		}

		if outputPath == "" {
			s.logger.Info("ProcessSource: Storing X documents directly in memory",
				"documentCount", len(documents))

			var memoryDocs []memory.Document
			for _, doc := range documents {
				docCopy := doc
				memoryDocs = append(memoryDocs, &docCopy)
			}

			progressCallback := func(processed, total int) {
				s.logger.Info("Processing documents", "processed", processed, "total", total)
			}

			if err := s.memory.Store(ctx, memoryDocs, progressCallback); err != nil {
				return false, fmt.Errorf("failed to store documents: %w", err)
			}

			s.logger.Info("Successfully processed and stored documents", "count", len(documents))
			return true, nil
		} else {
			if err := memory.ExportConversationDocumentsJSON(documents, outputPath); err != nil {
				return false, err
			}
			return true, nil
		}
	case constants.ProcessorWhatsapp:
		processor, err := whatsapp.NewWhatsappProcessor(s.store, s.logger)
		if err != nil {
			return false, err
		}
		// WhatsApp uses the new direct approach - skip the records step
		documents, err := processor.ProcessFile(ctx, inputPath)
		if err != nil {
			return false, err
		}

		// Check if outputPath is empty (direct memory storage)
		if outputPath == "" {
			// Store directly in memory
			var memoryDocs []memory.Document
			for _, doc := range documents {
				docCopy := doc
				memoryDocs = append(memoryDocs, &docCopy)
			}

			progressCallback := func(processed, total int) {
				s.logger.Info("Processing documents", "processed", processed, "total", total)
			}

			if err := s.memory.Store(ctx, memoryDocs, progressCallback); err != nil {
				return false, fmt.Errorf("failed to store documents: %w", err)
			}

			s.logger.Info("Successfully processed and stored documents", "count", len(documents))
			return true, nil
		} else {
			// Save to file for backward compatibility
			if err := memory.ExportConversationDocumentsJSON(documents, outputPath); err != nil {
				return false, err
			}
			return true, nil
		}
	case constants.ProcessorChatGPT:
		processor, err := chatgpt.NewChatGPTProcessor(s.store, s.logger)
		if err != nil {
			return false, err
		}
		// ChatGPT uses the new direct approach - skip the records step
		documents, err := processor.ProcessFile(ctx, inputPath)
		if err != nil {
			return false, err
		}

		// Check if outputPath is empty (direct memory storage)
		if outputPath == "" {
			// Store directly in memory
			var memoryDocs []memory.Document
			for _, doc := range documents {
				docCopy := doc
				memoryDocs = append(memoryDocs, &docCopy)
			}

			progressCallback := func(processed, total int) {
				s.logger.Info("Processing documents", "processed", processed, "total", total)
			}

			if err := s.memory.Store(ctx, memoryDocs, progressCallback); err != nil {
				return false, fmt.Errorf("failed to store documents: %w", err)
			}

			s.logger.Info("Successfully processed and stored documents", "count", len(documents))
			return true, nil
		} else {
			// Save to file for backward compatibility
			if err := memory.ExportConversationDocumentsJSON(documents, outputPath); err != nil {
				return false, err
			}
			return true, nil
		}
	case constants.ProcessorSyncedDocument:
		source, err := misc.NewTextDocumentProcessor(s.store, s.logger)
		if err != nil {
			return false, err
		}
		fileDocuments, err := source.ProcessFile(ctx, inputPath)
		if err != nil {
			return false, err
		}

		// Convert FileDocuments to Document interface for storage
		var documents []memory.Document
		for _, fileDoc := range fileDocuments {
			documents = append(documents, &fileDoc)
		}

		// Store via the memory system which will route to DocumentChunk storage
		progressCallback := func(processed, total int) {
			s.logger.Info("Processing documents", "processed", processed, "total", total)
		}

		if err := s.memory.Store(ctx, documents, progressCallback); err != nil {
			return false, fmt.Errorf("failed to store documents: %w", err)
		}

		s.logger.Info("Successfully processed and stored documents", "count", len(documents))
		return true, nil
	default:
		return false, fmt.Errorf("unsupported source: %s", sourceType)
	}

	if err != nil {
		return false, err
	}

	return true, nil
}

func (s *DataProcessingService) ToDocuments(ctx context.Context, sourceType string, records []types.Record) ([]memory.Document, error) {
	var documents []memory.Document

	switch constants.ProcessorType(strings.ToLower(sourceType)) {
	case constants.ProcessorChatGPT:
		// ChatGPT no longer supports ToDocuments - use direct ProcessFile interface instead
		return nil, fmt.Errorf("ChatGPT processor has been upgraded to new DocumentProcessor interface - use ProcessFile directly")
	case constants.ProcessorTelegram:
		telegramProcessor, err := telegram.NewTelegramProcessor(s.store, s.logger)
		if err != nil {
			return nil, err
		}
		documents, err = telegramProcessor.ToDocuments(ctx, records)
		if err != nil {
			return nil, err
		}
	case constants.ProcessorSlack:
		// Slack processor has been upgraded to new DocumentProcessor interface - use ProcessDirectory directly
		return nil, fmt.Errorf("slack processor has been upgraded to new DocumentProcessor interface - use ProcessDirectory directly")
	case constants.ProcessorGmail:
		// Gmail no longer supports ToDocuments - use direct ProcessFile interface instead
		return nil, fmt.Errorf("gmail processor has been upgraded to new DocumentProcessor interface - use ProcessFile directly")
	case constants.ProcessorWhatsapp:
		whatsappProcessor, err := whatsapp.NewWhatsappProcessor(s.store, s.logger)
		if err != nil {
			return nil, err
		}
		documents, err = whatsappProcessor.ToDocuments(ctx, records)
		if err != nil {
			return nil, err
		}
	case constants.ProcessorX:
		// X processor has been upgraded to new DocumentProcessor interface - use ProcessDirectory directly
		return nil, fmt.Errorf("x processor has been upgraded to new DocumentProcessor interface - use ProcessDirectory directly")
	case constants.ProcessorSyncedDocument:
		// Misc processor has been upgraded to new DocumentProcessor interface - use ProcessFile directly
		return nil, fmt.Errorf("misc processor has been upgraded to new DocumentProcessor interface - use ProcessFile directly")
	default:
		return nil, fmt.Errorf("unsupported source type: %s", sourceType)
	}

	return documents, nil
}

func (d *DataProcessingService) Sync(ctx context.Context, sourceName string, accessToken string) ([]types.Record, error) {
	var records []types.Record

	var authorized bool
	switch constants.ProcessorType(sourceName) {
	case constants.ProcessorGmail:
		// Gmail no longer supports Sync - use direct ProcessFile interface instead
		return nil, fmt.Errorf("gmail processor has been upgraded to new DocumentProcessor interface - use ProcessFile directly")
	case constants.ProcessorX:
		xProcessor, err := x.NewXProcessor(d.store, d.logger)
		if err != nil {
			return nil, err
		}
		records, authorized, err = xProcessor.Sync(ctx, accessToken)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported source: %s", sourceName)
	}

	if !authorized {
		if err := d.store.SetOAuthTokenError(ctx, accessToken, true); err != nil {
			d.logger.Warn("Error setting OAuth token error status", "sourceName", sourceName, "error", err)
		}
	}

	return records, nil
}
