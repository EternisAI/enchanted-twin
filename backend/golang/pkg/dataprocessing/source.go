package dataprocessing

import (
	"encoding/json"
	"time"
)

// Record represents a single data record that will be written to CSV
type Record struct {
	Data      map[string]interface{}
	Timestamp time.Time
	Source    string
}

// Source interface defines methods that each data source must implement
type Source interface {
	// ProcessFile processes the input file and returns records
	ProcessFile(filepath string, userName string) ([]Record, error)
	// Name returns the source identifier
	Name() string
}

// ToCSVRecord converts a Record to a CSV record format
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
