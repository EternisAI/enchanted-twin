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

	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/chatgpt"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/gmail"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/google_addresses"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/slack"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/telegram"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/whatsapp"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/x"
	"github.com/EternisAI/enchanted-twin/pkg/db"
)

// Record represents a single data record that will be written to CSV.
type Record struct {
	Data      map[string]any
	Timestamp time.Time
	Source    string
}

// Source interface defines methods that each data source must implement.
type Source interface {
	// ProcessFile processes the input file and returns records
	ProcessFile(filepath string, userName string) ([]Record, error)
	// Name returns the source identifier
	Name() string
	// Sync returns the records from the source
	Sync(ctx context.Context) ([]types.Record, error)
}

// ToCSVRecord converts a Record to a CSV record format.
func (r Record) ToCSVRecord() ([]string, error) {
	dataJSON, err := json.Marshal(r.Data)
	if err != nil {
		return nil, err
	}

	return []string{
		string(dataJSON),
		r.Timestamp.Format(time.RFC3339),
		r.Source,
	}, nil
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

func ProcessSource(sourceType, inputPath, outputPath, name, xApiKey string) (bool, error) {
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
		if name == "" {
			return false, fmt.Errorf("telegram requires a username")
		}
		source := telegram.New()
		records, err = source.ProcessFile(inputPath, name)
	case "slack":
		if name == "" {
			return false, fmt.Errorf("slack requires a username")
		}
		source := slack.New(inputPath)
		records, err = source.ProcessDirectory(name)
	case "gmail":
		if name == "" {
			return false, fmt.Errorf("gmail requires an email")
		}
		source := gmail.New()
		records, err = source.ProcessDirectory(inputPath, name)
	case "x":
		if name == "" {
			return false, fmt.Errorf("x requires a username")
		}
		source := x.New(inputPath)
		records, err = source.ProcessDirectory(name, xApiKey)
	case "whatsapp":
		source := whatsapp.New()
		records, err = source.ProcessFile(inputPath)
	case "google_addresses":
		source := google_addresses.New(inputPath)
		records, err = source.ProcessFile(inputPath)
	case "chatgpt":
		source := chatgpt.New(inputPath)
		records, err = source.ProcessDirectory(name)
	default:
		return false, fmt.Errorf("unsupported source type: %s", sourceType)
	}

	err = SaveRecords(records, outputPath)
	if err != nil {
		return false, err
	}

	return true, nil
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
		records, authorized, err = gmail.New().Sync(ctx, accessToken)
	case "x":
		records, authorized, err = x.New("").Sync(ctx, accessToken)
	default:
		return nil, fmt.Errorf("unsupported source: %s", sourceName)
	}

	if !authorized {
		store.SetOAuthTokenError(ctx, accessToken, true)
	}

	if err != nil {
		return nil, err
	}

	return records, nil
}
