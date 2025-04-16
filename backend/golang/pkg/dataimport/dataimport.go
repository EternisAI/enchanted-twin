package dataimport

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/dataimport/gmail"
	"github.com/EternisAI/enchanted-twin/pkg/dataimport/google_addresses"
	"github.com/EternisAI/enchanted-twin/pkg/dataimport/slack"
	"github.com/EternisAI/enchanted-twin/pkg/dataimport/telegram"
	"github.com/EternisAI/enchanted-twin/pkg/dataimport/types"
	"github.com/EternisAI/enchanted-twin/pkg/dataimport/whatsapp"
	"github.com/EternisAI/enchanted-twin/pkg/dataimport/x"
)

func ProcessSource(sourceType, inputPath, outputPath, name, xApiKey string) (bool, error) {
	var records []types.Record
	var err error

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
			return false, fmt.Errorf("error writing JSON: %v", err)
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
