package whatsapp

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/dataimport"

	_ "github.com/mattn/go-sqlite3"
)

type Source struct{}

func New() *Source {
	return &Source{}
}

func (s *Source) Name() string {
	return "whatsapp"
}

// ReadWhatsAppDB reads the WhatsApp database and returns Records
func ReadWhatsAppDB(dbPath string) ([]dataimport.Record, error) {
	// Open the database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}
	defer db.Close()
	// Query all messages from ZWAMESSAGE table
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
	defer rows.Close()

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

	var records []dataimport.Record
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
		record := dataimport.Record{
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

func (s *Source) ProcessFile(filepath string) ([]dataimport.Record, error) {
	return ReadWhatsAppDB(filepath)
}
