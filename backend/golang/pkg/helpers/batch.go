package helpers

func Batch[T any](items []T, batchSize int) [][]T {
	batches := make([][]T, 0)
	for i := 0; i < len(items); i += batchSize {
		end := min(i+batchSize, len(items))
		batches = append(batches, items[i:end])
	}
	return batches
}

// BatchWithMaxTextLength batches items into groups of a maximum text length ensuring we do not bigger batches than API can accept
func BatchWithMaxTextLength[T any](
	items []T,
	maxBatchTextLength int,
	desiredBatchSize int,
	textLenFn func(T) int,
) [][]T {
	if maxBatchTextLength <= 0 || desiredBatchSize <= 0 {
		return nil
	}

	var batches [][]T
	batch := make([]T, 0, desiredBatchSize)
	batchLen := 0

	for _, item := range items {
		l := textLenFn(item)

		if l > maxBatchTextLength {
			if len(batch) > 0 {
				batches = append(batches, batch)
				batch = make([]T, 0, desiredBatchSize)
				batchLen = 0
			}
			batches = append(batches, []T{item})
			continue
		}

		if batchLen+l > maxBatchTextLength || len(batch) >= desiredBatchSize {
			batches = append(batches, batch)
			batch = make([]T, 0, desiredBatchSize)
			batchLen = 0
		}

		batch = append(batch, item)
		batchLen += l
	}

	if len(batch) > 0 {
		batches = append(batches, batch)
	}

	return batches
}
