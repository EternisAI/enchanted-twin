package whatsapp

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/processor"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
	"github.com/EternisAI/enchanted-twin/pkg/db"
)

type WhatsappProcessor struct{}

func NewWhatsappProcessor() processor.Processor {
	return &WhatsappProcessor{}
}

func (s *WhatsappProcessor) Name() string {
	return "whatsapp"
}

func ReadWhatsAppDB(dbPath string) ([]types.Record, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Error closing database: %v", err)
		}
	}()

	query := `SELECT 
		m.Z_PK, m.ZISFROMME, m.ZCHATSESSION, m.ZMESSAGEINFO, m.ZMESSAGEDATE, m.ZSENTDATE,
		m.ZFROMJID, m.ZTEXT, m.ZTOJID,
		CASE 
			WHEN m.ZISFROMME = 1 THEN s.ZPARTNERNAME 
			ELSE '' 
		END AS ZTONAME,
		CASE 
			WHEN m.ZISFROMME = 0 THEN s.ZPARTNERNAME 
			ELSE '' 
		END AS ZFROMNAME
		FROM ZWAMESSAGE m
		LEFT JOIN ZWACHATSESSION s ON m.ZCHATSESSION = s.Z_PK
		WHERE m.ZTEXT IS NOT NULL`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query failed: %v", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Error closing rows: %v", err)
		}
	}()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get column names: %v", err)
	}

	// Prepare value containers
	count := len(columns)
	values := make([]interface{}, count)
	valuePtrs := make([]interface{}, count)
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	var records []types.Record
	// Iterate over rows
	rowCount := 0
	for rows.Next() {
		err := rows.Scan(valuePtrs...)
		if err != nil {
			log.Printf("Scan error: %v (row %d, expected %d columns)", err, rowCount, count)
			return nil, fmt.Errorf("scan failed: %v", err)
		}
		rowCount++

		// Create data map for this record
		data := make(map[string]interface{})
		var timestamp time.Time

		// Important fields to ensure are included in the data map
		importantFields := map[string]bool{
			"ZTEXT":     true,
			"ZISFROMME": true,
			"ZFROMJID":  true,
			"ZTOJID":    true,
			"ZFROMNAME": true,
			"ZTONAME":   true,
		}

		// Track if we've found all important fields
		foundFields := make(map[string]bool)

		// Populate data map
		for i, col := range columns {
			val := values[i]

			// Mark important field as found
			if importantFields[col] {
				foundFields[col] = true
			}

			// Simplify column name by removing 'Z' prefix
			simplifiedKey := col
			if len(col) > 1 && col[0] == 'Z' {
				simplifiedKey = col[1:]
			}

			// Handle different column types
			switch col {
			case "ZMESSAGEDATE", "ZSENTDATE":
				// SQLite timestamps can be stored in different formats
				// This assumes they're stored as Unix timestamps (seconds since epoch)
				if v, ok := val.(int64); ok {
					t := time.Unix(v, 0)
					data[simplifiedKey] = t
					// Use message date as the record timestamp if available
					if col == "ZMESSAGEDATE" {
						timestamp = t
					}
				} else if v, ok := val.(float64); ok {
					t := time.Unix(int64(v), 0)
					data[simplifiedKey] = t
					if col == "ZMESSAGEDATE" {
						timestamp = t
					}
				} else {
					data[simplifiedKey] = val
				}
			default:
				data[simplifiedKey] = val
			}
		}

		// Log warning for any important fields that weren't found
		for field := range importantFields {
			if !foundFields[field] {
				fmt.Printf("Warning: Important field %s not found in query results\n", field)
			}
		}

		// If no timestamp was set from ZMESSAGEDATE, use current time as fallback
		if timestamp.IsZero() {
			timestamp = time.Now()
		}

		// Create record
		record := types.Record{
			Data:      data,
			Timestamp: timestamp,
			Source:    "whatsapp",
		}

		records = append(records, record)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error during iteration: %v", err)
	}

	return records, nil
}

func (s *WhatsappProcessor) ProcessDirectory(ctx context.Context, filePath string, store *db.Store) ([]types.Record, error) {
	return nil, fmt.Errorf("sync operation not supported for WhatsApp")
}

func (s *WhatsappProcessor) ProcessFile(ctx context.Context, filePath string, store *db.Store) ([]types.Record, error) {
	return ReadWhatsAppDB(filePath)
}

func (s *WhatsappProcessor) Sync(ctx context.Context, accessToken string) ([]types.Record, bool, error) {
	return nil, false, fmt.Errorf("sync operation not supported for WhatsApp")
}

func (s *WhatsappProcessor) ToDocuments(records []types.Record) ([]memory.Document, error) {
	documents := make([]memory.TextDocument, 0, len(records))
	for _, record := range records {
		content, ok := record.Data["TEXT"].(string)
		if !ok {
			return nil, fmt.Errorf("failed to convert TEXT field to string")
		}

		fromName, ok := record.Data["FROMNAME"].(string)
		if !ok {
			return nil, fmt.Errorf("failed to convert FROMNAME field to string")
		}

		toName, ok := record.Data["TONAME"].(string)
		if !ok {
			return nil, fmt.Errorf("failed to convert TONAME field to string")
		}

		documents = append(documents, memory.TextDocument{
			FieldSource:    "whatsapp",
			FieldContent:   content,
			FieldTimestamp: &record.Timestamp,
			FieldTags:      []string{"whatsapp"},
			FieldMetadata: map[string]string{
				"from": fromName,
				"to":   toName,
			},
		})
	}
	var documents_ []memory.Document
	for _, document := range documents {
		documents_ = append(documents_, &document)
	}
	return documents_, nil
}
