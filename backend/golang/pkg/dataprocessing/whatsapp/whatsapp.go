package whatsapp

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
)

type WhatsappProcessor struct{}

func NewWhatsappProcessor() *WhatsappProcessor {
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

func (s *WhatsappProcessor) ProcessFile(filepath string) ([]types.Record, error) {
	return ReadWhatsAppDB(filepath)
}


func (s *WhatsappProcessor) Sync(ctx context.Context) ([]types.Record, error) {
	return nil, fmt.Errorf("sync operation not supported for WhatsApp")
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

// ProcessNewMessage processes a new WhatsApp message and stores it in memory.
func (s *WhatsappProcessor) ProcessNewMessage(ctx context.Context, memoryStorage memory.Storage, message string, fromName string, toName string) (memory.TextDocument, error) {
	if message == "" {
		return memory.TextDocument{}, fmt.Errorf("empty message content")
	}

	timestamp := time.Now()

	document := memory.TextDocument{
		FieldSource:    "whatsapp",
		FieldID:        fmt.Sprintf("whatsapp-%d", time.Now().UnixNano()),
		FieldContent:   message,
		FieldTimestamp: &timestamp,
		FieldTags:      []string{"whatsapp", "message"},
		FieldMetadata: map[string]string{
			"from": fromName,
			"to":   toName,
			"type": "message",
		},
	}

	return document, nil
}

// ProcessNewContact stores a WhatsApp contact in memory.
func (s *WhatsappProcessor) ProcessNewContact(ctx context.Context, memoryStorage memory.Storage, contactID string, contactName string) (memory.TextDocument, error) {
	if contactName == "" || contactID == "" {
		return memory.TextDocument{}, fmt.Errorf("empty contact information")
	}

	timestamp := time.Now()

	document := memory.TextDocument{
		FieldID:        fmt.Sprintf("whatsapp-contact-%d", time.Now().UnixNano()),
		FieldContent:   fmt.Sprintf("Whatsapp Contact name: %s. Contact ID: %s.", contactName, contactID),
		FieldTimestamp: &timestamp,
		FieldSource:    "whatsapp",
		FieldTags:      []string{"whatsapp", "contact"},
		FieldMetadata: map[string]string{
			"contact_id": contactID,
			"name":       contactName,
			"type":       "contact",
		},
	}

	return document, nil
}

// ProcessHistoricalMessage processes a historical WhatsApp message and stores it in memory.
func (s *WhatsappProcessor) ProcessHistoricalMessage(ctx context.Context, memoryStorage memory.Storage, message string, fromName string, toName string, timestampPtr uint64) (memory.TextDocument, error) {
	if message == "" {
		return memory.TextDocument{}, fmt.Errorf("empty message content")
	}

	var timestamp time.Time
	if timestampPtr != 0 {
		timestamp = time.Unix(int64(timestampPtr), 0)
	} else {
		timestamp = time.Now()
	}

	document := memory.TextDocument{
		FieldID:        fmt.Sprintf("whatsapp-history-%d", time.Now().UnixNano()),
		FieldContent:   message,
		FieldTimestamp: &timestamp,
		FieldSource:    "whatsapp",
		FieldTags:      []string{"whatsapp", "message", "conversation"},
		FieldMetadata: map[string]string{
			"from": fromName,
			"to":   toName,
			"type": "message",
		},
	}

	return document, nil
}
