package helpers

import (
	"bufio"
	"encoding/json"
	"os"
	"sort"

	"github.com/pkg/errors"

	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
)

func ReadJSONL[T any](filePath string) ([]T, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open file")
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Wrap(closeErr, "failed to close file")
		}
	}()

	var results []T
	scanner := bufio.NewScanner(file)

	const maxCapacity = 10 * 1024 * 1024
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, maxCapacity)

	for scanner.Scan() {
		line := scanner.Bytes()
		var item T
		if err := json.Unmarshal(line, &item); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal line")
		}
		results = append(results, item)
	}

	if err := scanner.Err(); err != nil {
		return nil, errors.Wrap(err, "scanner error")
	}

	if len(results) > 0 {
		if records, ok := any(results).([]types.Record); ok {
			sort.Slice(records, func(i, j int) bool {
				return records[i].Timestamp.Before(records[j].Timestamp)
			})
			results, ok = any(records).([]T)
			if !ok {
				return nil, errors.New("failed to convert records to []T")
			}
		}
	}

	return results, nil
}

func CountJSONLLines(filePath string) (int, error) {
	if filePath == "" {
		return 0, errors.New("filePath cannot be empty")
	}

	file, err := os.Open(filePath)
	if err != nil {
		return 0, errors.Wrap(err, "failed to open file")
	}

	var closeErr error
	defer func() {
		closeErr = file.Close()
	}()

	scanner := bufio.NewScanner(file)
	lineCount := 0

	for scanner.Scan() {
		lineCount++
	}

	if err := scanner.Err(); err != nil {
		return 0, errors.Wrap(err, "scanner error")
	}

	if closeErr != nil {
		return 0, errors.Wrap(closeErr, "failed to close file")
	}

	return lineCount, nil
}

func ReadJSONLBatch[T any](filePath string, startIndex, batchSize int) ([]T, error) {
	if filePath == "" {
		return nil, errors.New("filePath cannot be empty")
	}
	if startIndex < 0 {
		return nil, errors.New("startIndex cannot be negative")
	}
	if batchSize <= 0 {
		return nil, errors.New("batchSize must be positive")
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open file")
	}

	var closeErr error
	defer func() {
		closeErr = file.Close()
	}()

	scanner := bufio.NewScanner(file)
	const maxCapacity = 10 * 1024 * 1024
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, maxCapacity)

	var results []T
	lineIndex := 0
	endIndex := startIndex + batchSize

	for scanner.Scan() {
		if lineIndex >= startIndex && lineIndex < endIndex {
			line := scanner.Bytes()
			var item T
			if err := json.Unmarshal(line, &item); err != nil {
				return nil, errors.Wrapf(err, "failed to unmarshal line %d", lineIndex)
			}
			results = append(results, item)
		}

		lineIndex++

		if lineIndex >= endIndex {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, errors.Wrap(err, "scanner error")
	}

	if closeErr != nil {
		return nil, errors.Wrap(closeErr, "failed to close file")
	}

	if len(results) > 0 {
		if records, ok := any(results).([]types.Record); ok {
			sort.Slice(records, func(i, j int) bool {
				return records[i].Timestamp.Before(records[j].Timestamp)
			})
			sortedResults, ok := any(records).([]T)
			if !ok {
				return nil, errors.New("failed to convert sorted records back to []T")
			}
			results = sortedResults
		}
	}

	return results, nil
}
