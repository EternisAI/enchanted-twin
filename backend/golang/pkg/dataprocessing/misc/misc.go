package misc

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
)

const (
	DefaultChunkSize = 1000 // characters per chunk
)

type Source struct {
	inputPath  string
	chunkSize  int
	extensions []string
}

func New(inputPath string) *Source {
	return &Source{
		inputPath:  inputPath,
		chunkSize:  DefaultChunkSize,
		extensions: []string{".txt", ".md", ".log"}, // Default extensions to process
	}
}

// WithChunkSize allows setting a custom chunk size
func (s *Source) WithChunkSize(size int) *Source {
	s.chunkSize = size
	return s
}

// WithExtensions allows setting which file extensions to process
func (s *Source) WithExtensions(extensions []string) *Source {
	s.extensions = extensions
	return s
}

func (s *Source) Name() string {
	return "misc"
}

// ProcessFile processes a single text file and converts it into records
func (s *Source) ProcessFile(filePath string, options ...string) ([]types.Record, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info for %s: %w", filePath, err)
	}

	// Get file modification time as timestamp
	timestamp := fileInfo.ModTime()
	fileName := fileInfo.Name()

	// Read the file content
	scanner := bufio.NewScanner(file)
	var content strings.Builder

	// Track if we've added any lines yet
	firstLine := true
	for scanner.Scan() {
		// Add newline before each line, except the first one
		if !firstLine {
			content.WriteString("\n")
		}
		content.WriteString(scanner.Text())
		firstLine = false
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file %s: %w", filePath, err)
	}

	// Create chunks from the text
	textContent := content.String()
	var records []types.Record

	// Handle empty files
	if len(textContent) == 0 {
		emptyRecord := types.Record{
			Data: map[string]interface{}{
				"content":  "",
				"filename": fileName,
				"type":     "text",
				"path":     filePath,
			},
			Timestamp: timestamp,
			Source:    s.Name(),
		}
		return []types.Record{emptyRecord}, nil
	}

	// Process the text into chunks
	for i := 0; i < len(textContent); i += s.chunkSize {
		end := i + s.chunkSize
		if end > len(textContent) {
			end = len(textContent)
		}

		chunk := textContent[i:end]
		records = append(records, types.Record{
			Data: map[string]interface{}{
				"content":  chunk,
				"filename": fileName,
				"type":     "text",
				"path":     filePath,
				"chunk":    i / s.chunkSize,
			},
			Timestamp: timestamp,
			Source:    s.Name(),
		})
	}

	return records, nil
}

// ProcessDirectory walks through a directory and processes all text files
func (s *Source) ProcessDirectory(options ...string) ([]types.Record, error) {
	var allRecords []types.Record

	err := filepath.WalkDir(s.inputPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		// Check if the file has one of the supported extensions
		ext := strings.ToLower(filepath.Ext(path))
		isSupported := false
		for _, supportedExt := range s.extensions {
			if ext == supportedExt {
				isSupported = true
				break
			}
		}

		if !isSupported {
			return nil
		}

		records, err := s.ProcessFile(path)
		if err != nil {
			fmt.Printf("Warning: Failed to process file %s: %v\n", path, err)
			return nil
		}

		allRecords = append(allRecords, records...)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error walking directory %s: %w", s.inputPath, err)
	}

	fmt.Printf("Successfully processed %d records from directory %s\n", len(allRecords), s.inputPath)
	return allRecords, nil
}

func (s *Source) Sync(ctx context.Context, accessToken string) ([]types.Record, error) {
	// No remote sync capability for local text files
	return nil, fmt.Errorf("sync not supported for local text files")
}
