package google_addresses

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestProcessFile(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "addresses.json")

	testData := `{
		"type": "FeatureCollection",
		"features": [
			{
				"geometry": {
					"coordinates": [
						-122.4341346,
						37.7536259
					],
					"type": "Point"
				},
				"properties": {
					"date": "2023-03-07T04:55:55Z",
					"google_maps_url": "http://maps.google.com/?cid=12345678901234567890",
					"location": {
						"address": "123 Main St, San Francisco, CA 94114, USA",
						"country_code": "US",
						"name": "Test Location 1"
					}
				},
				"type": "Feature"
			},
			{
				"geometry": {
					"coordinates": [
						0,
						0
					],
					"type": "Point"
				},
				"properties": {
					"date": "2023-02-09T18:59:57Z",
					"google_maps_url": "http://maps.google.com/?q=456+Side+St,+San+Francisco,+CA+94114",
					"Comment": "No location information is available for this saved address"
				},
				"type": "Feature"
			}
		]
	}`

	err := os.WriteFile(testFile, []byte(testData), 0o644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	source := New(tempDir)
	records, err := source.ProcessFile(testFile)
	if err != nil {
		t.Fatalf("ProcessFile failed: %v", err)
	}

	if len(records) != 2 {
		t.Errorf("Expected 2 records, got %d", len(records))
	}

	if records[0].Source != "google_addresses" {
		t.Errorf("Expected source 'google_addresses', got '%s'", records[0].Source)
	}

	expectedTime, _ := time.Parse(time.RFC3339, "2023-03-07T04:55:55Z")
	if !records[0].Timestamp.Equal(expectedTime) {
		t.Errorf("Expected timestamp %v, got %v", expectedTime, records[0].Timestamp)
	}

	data := records[0].Data
	if data["type"] != "address" {
		t.Errorf("Expected type 'address', got '%v'", data["type"])
	}

	if data["longitude"] != -122.4341346 {
		t.Errorf("Expected longitude -122.4341346, got %v", data["longitude"])
	}

	if data["latitude"] != 37.7536259 {
		t.Errorf("Expected latitude 37.7536259, got %v", data["latitude"])
	}

	if data["name"] != "Test Location 1" {
		t.Errorf("Expected name 'Test Location 1', got '%v'", data["name"])
	}

	data2 := records[1].Data
	if data2["has_coordinates"] != false {
		t.Errorf("Expected has_coordinates to be false for zero coordinates")
	}

	if data2["comment"] != "No location information is available for this saved address" {
		t.Errorf("Expected comment to be set, got '%v'", data2["comment"])
	}
}

func TestProcessDirectory(t *testing.T) {
	tempDir := t.TempDir()

	testFile1 := filepath.Join(tempDir, "addresses1.json")
	testFile2 := filepath.Join(tempDir, "addresses2.json")

	testData1 := `{
		"type": "FeatureCollection",
		"features": [
			{
				"geometry": {
					"coordinates": [
						-122.4341346,
						37.7536259
					],
					"type": "Point"
				},
				"properties": {
					"date": "2023-03-07T04:55:55Z",
					"google_maps_url": "http://maps.google.com/?cid=12345678901234567890",
					"location": {
						"address": "123 Main St, San Francisco, CA 94114, USA",
						"country_code": "US",
						"name": "Test Location 1"
					}
				},
				"type": "Feature"
			}
		]
	}`

	testData2 := `{
		"type": "FeatureCollection",
		"features": [
			{
				"geometry": {
					"coordinates": [
						0,
						0
					],
					"type": "Point"
				},
				"properties": {
					"date": "2023-02-09T18:59:57Z",
					"google_maps_url": "http://maps.google.com/?q=456+Side+St,+San+Francisco,+CA+94114",
					"Comment": "No location information is available for this saved address"
				},
				"type": "Feature"
			}
		]
	}`

	err := os.WriteFile(testFile1, []byte(testData1), 0o644)
	if err != nil {
		t.Fatalf("Failed to create test file 1: %v", err)
	}

	err = os.WriteFile(testFile2, []byte(testData2), 0o644)
	if err != nil {
		t.Fatalf("Failed to create test file 2: %v", err)
	}

	ignoredFile := filepath.Join(tempDir, "ignored.txt")
	err = os.WriteFile(ignoredFile, []byte("This is not a JSON file"), 0o644)
	if err != nil {
		t.Fatalf("Failed to create ignored file: %v", err)
	}

	source := New(tempDir)
	records, err := source.ProcessFile(testFile1)
	if err != nil {
		t.Fatalf("ProcessDirectory failed: %v", err)
	}

	if len(records) != 1 {
		t.Errorf("Expected 1 record, got %d", len(records))
	}
}
