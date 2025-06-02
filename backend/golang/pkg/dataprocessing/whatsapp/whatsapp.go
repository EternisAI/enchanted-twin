package whatsapp

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
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

func ReadWhatsAppDB(ctx context.Context, dbPath string) ([]types.Record, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Error closing database: %v", err)
		}
	}()

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context canceled before query: %w", err)
	}

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

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query failed: %v", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Error closing rows: %v", err)
		}
	}()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get column names: %v", err)
	}

	count := len(columns)
	values := make([]interface{}, count)
	valuePtrs := make([]interface{}, count)
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	var records []types.Record
	rowCount := 0
	for rows.Next() {
		if rowCount%100 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, fmt.Errorf("context canceled during row processing: %w", err)
			}
		}

		err := rows.Scan(valuePtrs...)
		if err != nil {
			log.Printf("Scan error: %v (row %d, expected %d columns)", err, rowCount, count)
			return nil, fmt.Errorf("scan failed: %v", err)
		}
		rowCount++

		data := make(map[string]interface{})
		var timestamp time.Time

		importantFields := map[string]bool{
			"ZTEXT":     true,
			"ZISFROMME": true,
			"ZFROMJID":  true,
			"ZTOJID":    true,
			"ZFROMNAME": true,
			"ZTONAME":   true,
		}

		foundFields := make(map[string]bool)

		for i, col := range columns {
			val := values[i]

			if importantFields[col] {
				foundFields[col] = true
			}

			simplifiedKey := col
			if len(col) > 1 && col[0] == 'Z' {
				simplifiedKey = col[1:]
			}
			simplifiedKey = strings.ToLower(simplifiedKey)

			switch col {
			case "ZMESSAGEDATE", "ZSENTDATE":

				if v, ok := val.(int64); ok {
					t := time.Unix(v, 0)
					data[simplifiedKey] = t
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

		for field := range importantFields {
			if !foundFields[field] {
				fmt.Printf("Warning: Important field %s not found in query results\n", field)
			}
		}

		if timestamp.IsZero() {
			timestamp = time.Now()
		}

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
	return ReadWhatsAppDB(ctx, filePath)
}

func (s *WhatsappProcessor) Sync(ctx context.Context, accessToken string) ([]types.Record, bool, error) {
	return nil, false, fmt.Errorf("sync operation not supported for WhatsApp")
}

func (s *WhatsappProcessor) ToDocuments(records []types.Record) ([]memory.Document, error) {
	documents := make([]memory.TextDocument, 0, len(records))
	for _, record := range records {
		content, ok := record.Data["text"].(string)
		if !ok {
			return nil, fmt.Errorf("failed to convert text field to string")
		}

		fromName, ok := record.Data["fromname"].(string)
		if !ok {
			return nil, fmt.Errorf("failed to convert fromname field to string")
		}

		toName, ok := record.Data["toname"].(string)
		if !ok {
			return nil, fmt.Errorf("failed to convert toname field to string")
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
