package helpers

import (
	"bufio"
	"encoding/json"
	"os"

	"github.com/pkg/errors"
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

	return results, nil
}
