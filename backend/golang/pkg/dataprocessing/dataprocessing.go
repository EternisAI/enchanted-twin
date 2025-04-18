package dataprocessing

import (
	"archive/zip"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/gmail"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/google_addresses"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/slack"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/telegram"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/whatsapp"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/x"
)

func extractZip(zipPath string) (string, error) {
	tempDir, err := os.MkdirTemp("", "extracted_zip_")
	if err != nil {
		return "", fmt.Errorf("error creating temp directory: %v", err)
	}

	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		os.RemoveAll(tempDir)
		return "", fmt.Errorf("error opening zip file: %v", err)
	}
	defer reader.Close()

	for _, file := range reader.File {

		path := filepath.Join(tempDir, file.Name)

		if file.FileInfo().IsDir() {
			os.MkdirAll(path, file.Mode())
			continue
		}

		fileReader, err := file.Open()
		if err != nil {
			os.RemoveAll(tempDir)
			return "", fmt.Errorf("error opening file in zip: %v", err)
		}

		os.MkdirAll(filepath.Dir(path), 0o755)

		targetFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			fileReader.Close()
			os.RemoveAll(tempDir)
			return "", fmt.Errorf("error creating file: %v", err)
		}

		if _, err := io.Copy(targetFile, fileReader); err != nil {
			fileReader.Close()
			targetFile.Close()
			os.RemoveAll(tempDir)
			return "", fmt.Errorf("error extracting file: %v", err)
		}

		fileReader.Close()
		targetFile.Close()
	}

	return tempDir, nil
}

func ProcessSource(sourceType, inputPath, outputPath, name, xApiKey string) (bool, error) {
	var records []types.Record
	var err error

	ext := strings.ToLower(filepath.Ext(inputPath))
	if ext == ".zip" {

		tempDir, err := extractZip(inputPath)
		if err != nil {
			return false, fmt.Errorf("error extracting zip file: %v", err)
		}
		defer os.RemoveAll(tempDir)

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
		records, err = source.ProcessFile(inputPath, name)
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
	default:
		return false, fmt.Errorf("unsupported source type: %s", sourceType)
	}

	if err != nil {
		return false, fmt.Errorf("error processing input: %v", err)
	}

	if err := os.MkdirAll("./output", 0o755); err != nil {
		return false, fmt.Errorf("error creating output directory: %v", err)
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return false, fmt.Errorf("error creating output file: %v", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("Error closing file: %v", err)
		}
	}()

	// Determine output format based on file extension
	ext = strings.ToLower(filepath.Ext(outputPath))
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
			return false, fmt.Errorf("error writing JSON: %v", err)
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
			return false, fmt.Errorf("error writing CSV header: %v", err)
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
		return false, fmt.Errorf("unsupported output format: %s (use .csv or .json)", ext)
	}

	fmt.Printf("Successfully processed %d records and wrote to %s\n", len(records), outputPath)
	return true, nil
}
