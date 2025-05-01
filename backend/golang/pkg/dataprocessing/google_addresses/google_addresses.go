package google_addresses

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
)

type Coordinates []float64

type Geometry struct {
	Coordinates Coordinates `json:"coordinates"`
	Type        string      `json:"type"`
}

type Location struct {
	Address     string `json:"address,omitempty"`
	CountryCode string `json:"country_code,omitempty"`
	Name        string `json:"name,omitempty"`
}

type Properties struct {
	Date          string    `json:"date"`
	GoogleMapsURL string    `json:"google_maps_url"`
	Location      *Location `json:"location,omitempty"`
	Comment       string    `json:"Comment,omitempty"`
}

type Feature struct {
	Geometry   Geometry   `json:"geometry"`
	Properties Properties `json:"properties"`
	Type       string     `json:"type"`
}

type AddressCollection struct {
	Type     string    `json:"type"`
	Features []Feature `json:"features"`
}

type Source struct {
	inputPath string
}

func New(inputPath string) *Source {
	return &Source{
		inputPath: inputPath,
	}
}

func (s *Source) Name() string {
	return "google_addresses"
}

func (s *Source) ProcessFile(filePath string) ([]types.Record, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	var collection AddressCollection
	if err := json.Unmarshal(data, &collection); err != nil {
		return nil, fmt.Errorf("error parsing JSON: %w", err)
	}

	if collection.Type != "FeatureCollection" {
		return nil, fmt.Errorf(
			"invalid GeoJSON: expected FeatureCollection, got %s",
			collection.Type,
		)
	}

	var records []types.Record
	for _, feature := range collection.Features {
		if feature.Type != "Feature" {
			fmt.Printf("Warning: Skipping non-Feature element of type %s\n", feature.Type)
			continue
		}

		timestamp, err := time.Parse(time.RFC3339, feature.Properties.Date)
		if err != nil {
			fmt.Printf("Warning: Failed to parse timestamp %s: %v\n", feature.Properties.Date, err)
			continue
		}

		addressData := map[string]interface{}{
			"google_maps_url": feature.Properties.GoogleMapsURL,
			"type":            "address",
		}

		if feature.Geometry.Type == "Point" {
			if len(feature.Geometry.Coordinates) >= 2 {
				addressData["longitude"] = feature.Geometry.Coordinates[0]
				addressData["latitude"] = feature.Geometry.Coordinates[1]
				addressData["has_coordinates"] = feature.Geometry.Coordinates[0] != 0 ||
					feature.Geometry.Coordinates[1] != 0
			}
		}

		if feature.Properties.Location != nil {
			addressData["address"] = feature.Properties.Location.Address
			if feature.Properties.Location.CountryCode != "" {
				addressData["country_code"] = feature.Properties.Location.CountryCode
			}
			if feature.Properties.Location.Name != "" {
				addressData["name"] = feature.Properties.Location.Name
			}
		}

		if feature.Properties.Comment != "" && addressData["has_coordinates"] == false {
			addressData["comment"] = feature.Properties.Comment
		}

		record := types.Record{
			Data:      addressData,
			Timestamp: timestamp,
			Source:    s.Name(),
		}

		records = append(records, record)
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("no valid records found in the file")
	}

	return records, nil
}
