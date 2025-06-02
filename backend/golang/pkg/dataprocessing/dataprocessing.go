// Owner: slimane@eternis.ai
package dataprocessing

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/chatgpt"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/gmail"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/misc"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/slack"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/telegram"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/whatsapp"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/x"
	"github.com/EternisAI/enchanted-twin/pkg/db"
)

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
}

func NewDataProcessingService(openAiService *ai.Service, completionsModel string, store *db.Store) *DataProcessingService {
	return &DataProcessingService{
		openAiService:    openAiService,
		completionsModel: completionsModel,
		store:            store,
	}
}

func (s *DataProcessingService) ProcessSource(ctx context.Context, sourceType string, inputPath string, outputPath string) (bool, error) {
	var records []types.Record
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

	switch strings.ToLower(sourceType) {
	case "telegram":
		processor := telegram.NewTelegramProcessor()
		records, err = processor.ProcessFile(context.Background(), inputPath, s.store)
	case "slack":
		source := slack.New(inputPath)
		records, err = source.ProcessDirectory("")
	case "gmail":
		source := gmail.NewGmailProcessor()
		records, err = source.ProcessDirectory(inputPath, "")
	case "x":
		source := x.NewXProcessor()
		records, err = source.ProcessDirectory(inputPath)
	case "whatsapp":
		source := whatsapp.NewWhatsappProcessor()
		records, err = source.ProcessFile(context.Background(), inputPath, s.store)
	case "chatgpt":
		chatgptProcessor := chatgpt.NewChatGPTProcessor(inputPath)
		records, err = chatgptProcessor.ProcessDirectory(context.Background(), s.store)
	case "misc":
		source := misc.NewTextDocumentProcessor(s.openAiService, s.completionsModel)
		records, err = source.ProcessDirectory(inputPath)
	default:
		return false, fmt.Errorf("unsupported source: %s", sourceType)
	}
	if err != nil {
		return false, err
	}

	err = SaveRecords(records, outputPath)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (s *DataProcessingService) ToDocuments(ctx context.Context, sourceType string, records []types.Record) ([]memory.Document, error) {
	var documents []memory.Document
	var err error

	sourceType = strings.ToLower(sourceType)
	switch sourceType {
	case "chatgpt":
		chatgptProcessor := chatgpt.NewChatGPTProcessor("")
		documents, err = chatgptProcessor.ToDocuments(records)
		if err != nil {
			return nil, err
		}
	case "telegram":
		telegramProcessor := telegram.NewTelegramProcessor()
		documents, err = telegramProcessor.ToDocuments(records)
		if err != nil {
			return nil, err
		}
	case "slack":
		slackProcessor := slack.New("")
		documents, err = slackProcessor.ToDocuments(records)
		if err != nil {
			return nil, err
		}
	case "gmail":
		gmailProcessor := gmail.NewGmailProcessor()
		documents, err = gmailProcessor.ToDocuments(records)
		if err != nil {
			return nil, err
		}
	case "whatsapp":
		whatsappProcessor := whatsapp.NewWhatsappProcessor()
		documents, err = whatsappProcessor.ToDocuments(records)
		if err != nil {
			return nil, err
		}
	case "x":
		xProcessor := x.NewXProcessor()
		documents, err = xProcessor.ToDocuments(records)
		if err != nil {
			return nil, err
		}
	case "misc":
		miscProcessor := misc.NewTextDocumentProcessor(s.openAiService, s.completionsModel)
		documents, err = miscProcessor.ToDocuments(records)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported source type: %s", sourceType)
	}

	return documents, nil
}

func SaveRecords(records []types.Record, outputPath string) error {
	// Create the output directory if it doesn't exist
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("error creating output directory: %v", err)
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("error creating output file: %v", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("Error closing file: %v", err)
		}
	}()

	// Determine output format based on file extension
	ext := strings.ToLower(filepath.Ext(outputPath))
	switch ext {
	case ".json":
		// For JSON output, create a slice of records with their data
		type jsonRecord struct {
			Data      map[string]interface{} `json:"data"`
			Timestamp string                 `json:"timestamp"`
			Source    string                 `json:"source"`
		}

		jsonRecords := make([]jsonRecord, len(records))
		for i, record := range records {
			jsonRecords[i] = jsonRecord{
				Data:      record.Data,
				Timestamp: record.Timestamp.Format(time.RFC3339),
				Source:    record.Source,
			}
		}

		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(jsonRecords); err != nil {
			return fmt.Errorf("error writing JSON: %v", err)
		}

	case ".jsonl":
		// For JSONL output, write each record as a separate line
		for _, record := range records {
			jsonRecord := struct {
				Data      map[string]interface{} `json:"data"`
				Timestamp string                 `json:"timestamp"`
				Source    string                 `json:"source"`
			}{
				Data:      record.Data,
				Timestamp: record.Timestamp.Format(time.RFC3339),
				Source:    record.Source,
			}

			jsonData, err := json.Marshal(jsonRecord)
			if err != nil {
				log.Printf("Error marshaling record to JSON: %v", err)
				continue
			}

			if _, err := file.Write(jsonData); err != nil {
				log.Printf("Error writing JSONL record: %v", err)
				continue
			}
			if _, err := file.Write([]byte("\n")); err != nil {
				log.Printf("Error writing newline: %v", err)
				continue
			}
		}

	case ".csv":
		writer := csv.NewWriter(file)
		defer writer.Flush()

		header := []string{"data", "timestamp", "source"}
		if err := writer.Write(header); err != nil {
			return fmt.Errorf("error writing CSV header: %v", err)
		}

		for _, record := range records {
			csvRecord, err := record.ToCSVRecord()
			if err != nil {
				log.Printf("Error converting record to CSV: %v", err)
				continue
			}

			if err := writer.Write(csvRecord); err != nil {
				log.Printf("Error writing record: %v", err)
				continue
			}
		}

	default:
		return fmt.Errorf("unsupported output format: %s (use .csv, .jsonl, .json)", ext)
	}

	fmt.Printf("Successfully processed %d records and wrote to %s\n", len(records), outputPath)
	return nil
}

func Sync(ctx context.Context, sourceName string, accessToken string, store *db.Store) ([]types.Record, error) {
	var records []types.Record
	var err error

	var authorized bool
	switch sourceName {
	case "gmail":
		gmailProcessor := gmail.NewGmailProcessor()
		records, authorized, err = gmailProcessor.Sync(ctx, accessToken)
	case "x":
		xProcessor := x.NewXProcessor()
		records, authorized, err = xProcessor.Sync(ctx, accessToken)
	default:
		return nil, fmt.Errorf("unsupported source: %s", sourceName)
	}

	if !authorized {
		if err := store.SetOAuthTokenError(ctx, accessToken, true); err != nil {
			// Log the error, but continue as the main error (if any) is handled below
			// TODO: Consider how to handle this more robustly
			log.Printf("Error setting OAuth token error status for %s: %v", sourceName, err)
		}
	}

	if err != nil {
		return nil, err
	}

	return records, nil
}
